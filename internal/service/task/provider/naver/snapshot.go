package naver

// performanceEventType 공연 정보의 변경 유형을 나타내는 열거형입니다.
type performanceEventType int

const (
	// performanceEventNone 변경 사항이 없음을 나타냅니다.
	performanceEventNone performanceEventType = iota

	// performanceEventNew 이전 스냅샷에 없던 신규 공연임을 나타냅니다.
	performanceEventNew
)

// performanceDiff 스냅샷 비교 결과로 발견된 공연 정보의 변경 사항을 표현하는 구조체입니다.
//
// 이 구조체는 Compare() 메서드의 반환값으로 사용되며, 알림 메시지 생성 시
// 어떤 공연이 어떻게 변경되었는지 판단하는 데 활용됩니다.
type performanceDiff struct {
	// Type 변경 유형 (신규 등록, 삭제 등)
	Type performanceEventType

	// Performance 변경된 공연 정보
	Performance *performance
}

// watchNewPerformancesSnapshot 네이버에서 검색된 공연 목록의 스냅샷입니다.
//
// 이 구조체는 특정 시점에 검색된 공연 정보를 저장하며, 이전 스냅샷과 비교하여
// 신규 공연 등록, 공연 삭제, 정보 변경 등을 감지하는 데 사용됩니다.
type watchNewPerformancesSnapshot struct {
	// Performances 현재 검색 결과에서 발견된 공연 목록입니다.
	Performances []*performance `json:"performances"`
}

// Compare 현재 스냅샷과 이전 스냅샷을 비교하여 공연 정보의 변경 사항을 감지합니다.
//
// 다음 시나리오에 대한 변화를 감지합니다:
//  1. 신규 공연 등록: 이전 스냅샷에 없던 공연이 현재 스냅샷에 추가된 경우
//  2. 공연 삭제: 이전 스냅샷에 있던 공연이 현재 스냅샷에서 사라진 경우
//  3. 공연 정보 변경: 공연은 동일하지만 내용(썸네일 등)이 변경된 경우
//
// 매개변수:
//   - prev: 비교 대상이 되는 이전 스냅샷 (nil일 수 있음)
//
// 반환값:
//   - diffs: 신규로 추가된 공연 목록 (알림 메시지 생성용)
//   - hasChanges: 스냅샷 갱신이 필요한지 여부 (추가/삭제/내용변경 모두 포함)
func (s *watchNewPerformancesSnapshot) Compare(prev *watchNewPerformancesSnapshot) (diffs []performanceDiff, hasChanges bool) {
	// 1단계: 이전 스냅샷의 공연 목록을 Map으로 변환하여 빠른 조회가 가능하게 함
	// Key: 공연 고유 식별자, Value: 공연 객체 (내용 비교용)
	prevMap := make(map[string]*performance)
	if prev != nil {
		for _, p := range prev.Performances {
			prevMap[p.key()] = p
		}
	}

	// 2단계: 현재 스냅샷의 공연들을 순회하며 신규 공연 및 내용 변경 감지
	for _, p := range s.Performances {
		prevPerformance, exists := prevMap[p.key()]
		if !exists {
			// 케이스 1: 신규 공연 발견
			// 이전 스냅샷에 없던 공연이므로 diffs에 추가하고 hasChanges를 true로 설정
			diffs = append(diffs, performanceDiff{
				Type:        performanceEventNew,
				Performance: p,
			})

			hasChanges = true
		} else {
			// 케이스 2: 기존 공연의 내용 변경 확인
			// 공연은 동일하지만 내용(예: 썸네일)이 변경되었을 수 있음!
			// 알림 대상은 아니지만, 스냅샷 갱신은 필요하므로 hasChanges를 true로 설정
			if !p.contentEquals(prevPerformance) {
				hasChanges = true
			}
		}
	}

	// 3단계: 공연 삭제 감지(개수 비교) 및 비정상 상황(0건) 방어
	//
	// [시나리오]
	// 1. A 삭제: len(prev)=2, len(cur)=1 -> hasChanges=true (정상)
	// 2. A 삭제 & B 추가: B 추가 로직에서 이미 hasChanges=true (정상)
	prevLen := 0
	if prev != nil {
		prevLen = len(prev.Performances)
	}

	// [중요] 일시적 오류로 인해 검색 결과가 0건이 된 경우 스냅샷 갱신 방지
	//
	// 시나리오: 네이버 측 일시 오류나 네트워크 문제로 0건이 반환될 수 있습니다.
	// 이때 스냅샷을 0건으로 갱신하면, 다음 정상 실행 시 모든 공연이 '신규'로 인식되어
	// 사용자에게 대량 알림(Spam)이 전송되는 참사가 발생합니다.
	//
	// 전략: '이전에 데이터가 있었는데 갑자기 0건이 된 경우'는 비정상으로 간주하여
	// 변경사항이 없다고 판단(false 반환)하고 기존 스냅샷을 유지합니다.
	//
	// [주의 - 한계점]
	// 이 안전 장치로 인해 실제로 모든 공연이 종료되어 0건이 된 경우에도
	// 스냅샷이 갱신되지 않아, DB에 과거 데이터가 남을 수 있습니다.
	// 하지만 잘못된 알림 폭탄(Spam)으로 인한 사용자 경험 저하를 막는 것이
	// 데이터 정합성보다 우선순위가 높다고 판단하여 이 방식을 채택합니다.
	if len(s.Performances) == 0 && prevLen > 0 {
		return nil, false
	}

	// [참고] 이 로직은 삭제된 공연을 정확히 식별(누가 삭제되었는지)하지 않고,
	// 단순히 전체 개수가 변동되었는지만 확인하여 스냅샷 갱신(`hasChanges=true`)을 유도합니다.
	if len(s.Performances) != prevLen {
		hasChanges = true
	}

	return diffs, hasChanges
}
