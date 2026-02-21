package naver

import (
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/stretchr/testify/assert"
)

func TestTask_AnalyzeAndReport_TableDriven(t *testing.T) {
	t.Parallel()

	// -------------------------------------------------------------------------
	// 헬퍼: 테스트용 공연 데이터 생성
	// -------------------------------------------------------------------------
	makePerformance := func(id, title, place, thumb string) *performance {
		return &performance{
			Title:     title,
			Place:     place,
			Thumbnail: thumb,
		}
	}

	pNew := makePerformance("1", "신규 공연", "서울", "http://thumb1")
	pOld := makePerformance("2", "기존 공연", "부산", "http://thumb2")
	pChangedThumb := makePerformance("2", "기존 공연", "부산", "http://thumb2-modified")

	// -------------------------------------------------------------------------
	// 테스트 시나리오 정의
	// -------------------------------------------------------------------------
	tests := []struct {
		name         string
		runBy        contract.TaskRunBy
		current      []*performance
		prev         []*performance
		supportsHTML bool

		// 검증 항목
		expectMessage  []string // 알림 메시지에 포함되어야 하는 키워드 목록
		expectEmptyMsg bool     // 알림 메시지가 완전히 비어있어야 하는지 여부
		expectSave     bool     // shouldSave(스냅샷 갱신 필요 여부) 기댓값
	}{
		// [시나리오 그룹 1] 스케줄러(Scheduler) 실행
		{
			name:  "Scheduler: 신규 공연 1건 발견 (이전 스냅샷 없음)",
			runBy: contract.TaskRunByScheduler,
			current: []*performance{
				pNew,
			},
			prev:         nil, // 최초 실행
			supportsHTML: false,
			expectMessage: []string{
				"새로운 공연정보가 등록되었습니다",
				"신규 공연",
			},
			expectSave: true, // 새로운 스냅샷 저장(갱신) 필요
		},
		{
			name:  "Scheduler: 기존 공연 유지 및 신규 공연 추가",
			runBy: contract.TaskRunByScheduler,
			current: []*performance{
				pOld,
				pNew,
			},
			prev: []*performance{
				pOld,
			},
			supportsHTML: false,
			expectMessage: []string{
				"새로운 공연정보가 등록되었습니다",
				"신규 공연",
			},
			expectSave: true, // 상태가 변했으므로 스냅샷 갱신 필요
		},
		{
			name:  "Scheduler: 변경 사항 없음",
			runBy: contract.TaskRunByScheduler,
			current: []*performance{
				pOld,
			},
			prev: []*performance{
				pOld,
			},
			supportsHTML:   false,
			expectEmptyMsg: true,  // 노이즈 방지를 위해 침묵
			expectSave:     false, // 저장 불필요
		},
		{
			name:    "Scheduler: 기존 공연 삭제만 발생",
			runBy:   contract.TaskRunByScheduler,
			current: []*performance{
				// pOld 삭제됨
			},
			prev: []*performance{
				pOld,
			},
			supportsHTML:   false,
			expectEmptyMsg: true,  // 삭제만 된 경우 새 알림을 보내지 않음
			expectSave:     false, // 단, 스냅샷은 0건으로 갱신해야 함 (다음부터 신규 감지를 위해) -> 사실 안전장치 때문에 false가 됨
		},
		{
			name:  "Scheduler: 썸네일 등 내용만 변경",
			runBy: contract.TaskRunByScheduler,
			current: []*performance{
				pChangedThumb,
			},
			prev: []*performance{
				pOld, // ID, Title, Place는 같지만 Thumbnail이 다름
			},
			supportsHTML:   false,
			expectEmptyMsg: true, // 내용은 바뀌었지만 신규 추가는 아니므로 알림 발송 안 함
			expectSave:     true, // 하지만 이후 중복 변경 감지를 막기 위해 스냅샷은 갱신
		},
		{
			name:  "Scheduler: 0건 버그 방어 로직 (안전장치)",
			runBy: contract.TaskRunByScheduler,
			// analyzer.go 자체가 아닌 snapshot.go의 Compare에서 처리되는 로직이지만,
			// 통합 관점에서 analyzer의 동작(저장 안 함, 메시지 안 보냄)을 검증
			current: []*performance{},
			prev: []*performance{
				pOld,
			},
			// snapshot Compare()에서 "prev가 있는데 current가 0건이면 false 반환"하도록 방어되어 있다면
			// hasChanges는 false가 되고, expectSave도 false가 될 것입니다.
			// (현재 snapshot.go의 내용 확인 결과 이 방어 로직이 적용되어 있습니다)
			supportsHTML:   false,
			expectEmptyMsg: true,
			expectSave:     false, // 비정상 0건으로 간주하여 갱신 방지
		},

		// [시나리오 그룹 2] 수동 사용자(User) 실행
		{
			name:  "User: 변경 사항 없음 - 현재 상태 피드백",
			runBy: contract.TaskRunByUser,
			current: []*performance{
				pOld,
			},
			prev: []*performance{
				pOld,
			},
			supportsHTML: false,
			expectMessage: []string{
				"현재 등록된 공연정보는 아래와 같습니다", // 수동 실행 시 현재 상태 반환
				"기존 공연",
			},
			expectSave: false, // 갱신은 불필요
		},
		{
			name:  "User: 신규 공연 발견",
			runBy: contract.TaskRunByUser,
			current: []*performance{
				pNew,
			},
			prev:         nil,
			supportsHTML: false,
			expectMessage: []string{
				"새로운 공연정보가 등록되었습니다",
				"신규 공연",
			},
			expectSave: true, // 즉각 갱신 필요
		},
		{
			name:    "User: 공연 삭제만 발생",
			runBy:   contract.TaskRunByUser,
			current: []*performance{
				// 전체 다 삭제된 빈 목록
			},
			prev: []*performance{
				pOld,
			},
			supportsHTML: false,
			expectMessage: []string{
				"등록된 공연정보가 존재하지 않습니다.", // renderCurrentStatus 구현 내역 반영
			},
			expectEmptyMsg: false,
			expectSave:     false,
		},
		{
			name:  "User: 내용(썸네일)만 변경",
			runBy: contract.TaskRunByUser,
			current: []*performance{
				pChangedThumb,
			},
			prev: []*performance{
				pOld,
			},
			supportsHTML: false,
			expectMessage: []string{
				"현재 등록된 공연정보는 아래와 같습니다", // 알림 대상 신규 항목은 없지만 User 실행이라 현재 상태 피드백
				"기존 공연", // url등이 변한 최신 상태 객체가 보여야 함 (렌더러 단 검증)
			},
			expectSave: true, // 갱신은 수행되어야 함
		},

		// [시나리오 그룹 3] HTML 렌더링 검증
		{
			name:  "HTML 포맷 지원 (신규)",
			runBy: contract.TaskRunByScheduler,
			current: []*performance{
				pNew,
			},
			prev:         nil,
			supportsHTML: true,
			expectMessage: []string{
				"<b>신규 공연</b>", // HTML 태그 출현 확인 (renderer 세부 구조에 따라 조정 가능)
				"새로운 공연",
			},
			expectSave: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Task 생성 및 RunBy 설정
			tsk := &task{
				Base: provider.NewBase(provider.NewTaskParams{
					Request: &contract.TaskSubmitRequest{
						RunBy: tt.runBy,
					},
					Fetcher: mocks.NewMockHTTPFetcher(),
				}, true),
			}

			// 입력 스냅샷 객체 초기화
			currentSnapshot := &watchNewPerformancesSnapshot{Performances: tt.current}
			var prevSnapshot *watchNewPerformancesSnapshot
			if tt.prev != nil {
				prevSnapshot = &watchNewPerformancesSnapshot{Performances: tt.prev}
			}

			// 실행
			message, shouldSave := tsk.analyzeAndReport(currentSnapshot, prevSnapshot, tt.supportsHTML)

			// 검증: 스냅샷 갱신 여부
			assert.Equal(t, tt.expectSave, shouldSave, "shouldSave(hasChanges) 결과가 예상과 일치해야 합니다")

			// 검증: 메시지 상태
			if tt.expectEmptyMsg {
				assert.Empty(t, strings.TrimSpace(message), "알림 메시지가 완전히 비어있어야 합니다")
			} else {
				assert.NotEmpty(t, message, "알림 메시지가 비어있지 않아야 합니다")
				for _, expectedKeyword := range tt.expectMessage {
					assert.Contains(t, message, expectedKeyword, "알림 메시지에 필수 키워드가 포함되어야 합니다")
				}
			}
		})
	}
}
