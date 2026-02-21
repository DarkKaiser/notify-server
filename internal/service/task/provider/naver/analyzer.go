package naver

import (
	"fmt"

	"github.com/darkkaiser/notify-server/internal/service/contract"
)

const (
	// emptyPerformancesMsg 수집된 공연 목록이 비어있을 때 사용자에게 전송하는 안내 메시지입니다.
	// 스케줄러가 아닌 사용자 직접 실행 시에만 전송되며, 현재 등록된 공연이 하나도 없음을 알립니다.
	emptyPerformancesMsg = "등록된 공연정보가 존재하지 않습니다."

	// noNewPerformancesFormat 신규 공연은 없지만 현재 등록된 공연 목록을 사용자에게 안내할 때 사용하는 메시지 포맷입니다.
	// 단순 삭제·내용 변경만 발생했거나 변경 사항이 전혀 없는 경우, 사용자 직접 실행 시에
	// 현재 상태를 한눈에 확인할 수 있도록 전체 목록을 함께 전송합니다.
	// %s 자리에는 현재 등록된 공연 목록이 포맷팅되어 삽입됩니다.
	noNewPerformancesFormat = "신규로 등록된 공연정보가 없습니다.\n\n현재 등록된 공연정보는 아래와 같습니다:\n\n%s"
)

// analyzeAndReport 수집된 스냅샷을 이전 상태와 비교하여 변경 사항을 분석하고,
// 실행 방식(스케줄러/사용자)에 맞는 알림 메시지와 갱신 여부를 판단합니다.
//
// 매개변수:
//   - currentSnapshot: 이번에 새로 수집한 최신 공연 목록 스냅샷
//   - prevSnapshot: 직전 실행 시 저장해둔 이전 공연 목록 스냅샷 (최초 실행 시 nil)
//   - supportsHTML: 수신 채널이 HTML 렌더링을 지원하는지 여부
//
// 반환값:
//   - message: 사용자에게 전송할 알림 메시지. 알림이 불필요한 경우 빈 문자열("")
//   - hasChanges: 스냅샷 갱신이 필요한지 여부. 신규 공연 추가, 삭제, 내용 변경 시 true
func (t *task) analyzeAndReport(currentSnapshot *watchNewPerformancesSnapshot, prevSnapshot *watchNewPerformancesSnapshot, supportsHTML bool) (string, bool) {
	var message string

	// 현재 스냅샷과 이전 스냅샷을 비교하여 달라진 공연 목록을 추출합니다.
	// hasChanges는 신규 추가/삭제/내용 변경 중 하나라도 감지된 경우 true가 됩니다.
	diffs, hasChanges := currentSnapshot.Compare(prevSnapshot)

	// 신규로 등록된 공연이 있으면 변경 목록을 포맷팅하여 알림 메시지를 구성합니다.
	// 공연 삭제나 내용 변경만 발생한 경우에는 message가 빈 문자열로 유지됩니다.
	if len(diffs) > 0 {
		message = "새로운 공연정보가 등록되었습니다.\n\n" + renderPerformanceDiffs(diffs, supportsHTML)
	}

	// [변경 사항이 있는 경우 (hasChanges == true)]
	// 신규 추가/삭제/내용 변경 중 하나 이상이 감지된 상태입니다.
	// 단, 알림 메시지가 비어있다는 것은 '신규 추가' 없이 '삭제·내용 변경'만 발생했음을 의미합니다.
	// 이 경우 스케줄러 실행이라면 조용히 스냅샷만 갱신하고,
	// 사용자 직접 실행이라면 "현재 등록된 공연 목록 전체"를 알림으로 대신 전송합니다.
	if hasChanges {
		if message == "" && t.RunBy() == contract.TaskRunByUser {
			if len(currentSnapshot.Performances) == 0 {
				message = emptyPerformancesMsg
			} else {
				message = fmt.Sprintf(noNewPerformancesFormat, renderCurrentStatus(currentSnapshot, supportsHTML))
			}
		}

		return message, true
	}

	// [변경 사항이 없는 경우 (hasChanges == false)]
	// 스냅샷 갱신이 불필요합니다.
	// 사용자 직접 실행인 경우에는 "현재 등록된 공연 목록 전체"를 알림으로 전송하여
	// 시스템이 정상 동작 중임을 확인시켜 줍니다.
	// 스케줄러 실행인 경우에는 변경이 없으므로 빈 문자열("")을 반환하여 알림을 생략합니다.
	if t.RunBy() == contract.TaskRunByUser {
		if len(currentSnapshot.Performances) == 0 {
			return emptyPerformancesMsg, false
		}

		return fmt.Sprintf(noNewPerformancesFormat, renderCurrentStatus(currentSnapshot, supportsHTML)), false
	}

	return message, false
}
