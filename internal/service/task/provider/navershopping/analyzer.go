package navershopping

import (
	"fmt"

	"github.com/darkkaiser/notify-server/internal/service/contract"
)

// analyzeAndReport 수집된 스냅샷을 이전 상태와 비교하여 변경 사항을 분석하고,
// 실행 방식(스케줄러/사용자)에 맞는 알림 메시지와 갱신 여부를 판단합니다.
//
// 매개변수:
//   - commandSettings: 검색 키워드와 필터 조건 등의 조회 조건 설정
//   - currentSnapshot: 이번에 새로 수집된 최신 상품 목록 스냅샷
//   - prevSnapshot: 직전 실행 시 저장해둔 이전 상품 목록 스냅샷 (최초 실행 시 nil)
//   - supportsHTML: 수신 채널이 HTML 렌더링을 지원하는지 여부
//
// 반환값:
//   - message: 사용자에게 전송할 알림 메시지. 알림이 불필요한 경우 빈 문자열("")
//   - hasChanges: 스냅샷 갱신이 필요한지 여부. 신규 상품 추가, 삭제, 가격변동, 메타 정보 변경 시 true
func (t *task) analyzeAndReport(commandSettings *watchPriceSettings, currentSnapshot *watchPriceSnapshot, prevSnapshot *watchPriceSnapshot, supportsHTML bool) (string, bool) {
	// 현재 스냅샷과 이전 스냅샷을 비교하여 변동된 상품 목록을 추출합니다.
	// hasChanges는 신규 추가/삭제/가격변동/메타 변경 중 하나라도 감지된 경우 true가 됩니다.
	diffs, hasChanges := currentSnapshot.Compare(prevSnapshot)

	// 신규 등록 또는 가격 변동 상품이 발견된 경우, 조회 조건과 변동 목록을 함께 조합하여 알림 메시지를 구성합니다.
	if len(diffs) > 0 {
		searchConditionsSummary := renderSearchConditionsSummary(commandSettings)

		return fmt.Sprintf("조회 조건에 해당되는 상품 정보가 변경되었습니다.\n\n%s\n\n%s", searchConditionsSummary, renderProductDiffs(diffs, supportsHTML)), hasChanges
	}

	// 알림 대상인 diffs(신규 등록, 가격 변동)는 없는 상태입니다.
	// (단, 삭제·메타 변경은 발생했을 수 있으므로 hasChanges가 true일 수도 있습니다)
	// 스케줄러 실행 시에는 조용히 스냅샷만 갱신하고,
	// 사용자 직접 실행 시에는 "현재 조회 조건에 해당되는 상품 목록 전체"를 알림으로 전송하여 시스템이 정상 동작 중임을 확인시켜 줍니다.
	if t.RunBy() == contract.TaskRunByUser {
		searchConditionsSummary := renderSearchConditionsSummary(commandSettings)

		if len(currentSnapshot.Products) == 0 {
			return fmt.Sprintf("조회 조건에 해당되는 상품이 존재하지 않습니다.\n\n%s", searchConditionsSummary), hasChanges
		}

		return fmt.Sprintf("조회 조건에 해당되는 상품의 변경된 정보가 없습니다.\n\n%s\n\n조회 조건에 해당되는 상품은 아래와 같습니다:\n\n%s", searchConditionsSummary, renderCurrentStatus(currentSnapshot, supportsHTML)), hasChanges
	}

	return "", hasChanges
}
