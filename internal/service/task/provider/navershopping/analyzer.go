package navershopping

import (
	"fmt"

	"github.com/darkkaiser/notify-server/internal/service/contract"
)

// @@@@@
// analyzeAndReport 수집된 데이터를 분석하여 사용자에게 보낼 알림 메시지를 생성합니다.
//
// [주요 동작]
// 1. 변화 확인: 이전 데이터와 비교해 새로운 상품이나 가격 변동이 있는지 확인합니다.
// 2. 메시지 작성: 발견된 변화를 보기 좋게 포맷팅합니다.
// 3. 알림 결정:
//   - 스케줄러 실행: 변화가 있을 때만 알림을 보냅니다. (조용히 모니터링)
//   - 사용자 실행: 변화가 없어도 "변경 없음"이라고 알려줍니다. (확실한 피드백)
func (t *task) analyzeAndReport(commandSettings *watchPriceSettings, currentSnapshot *watchPriceSnapshot, prevSnapshot *watchPriceSnapshot, supportsHTML bool) (message string, shouldSave bool) {
	// 신규 상품 및 가격 변동을 식별합니다.
	// (단순 비교뿐만 아니라, 사용자 편의를 위한 정렬 로직이 포함됩니다)
	diffs := currentSnapshot.Compare(prevSnapshot)

	// 식별된 변동 사항을 사용자가 이해하기 쉬운 알림 메시지로 변환합니다.
	diffMessage := renderProductDiffs(diffs, supportsHTML)

	// 변경 내역(New/Price Change)이 집계된 경우, 즉시 알림 메시지를 구성하여 반환합니다.
	if len(diffs) > 0 {
		searchConditionsSummary := renderSearchConditionsSummary(commandSettings)

		return fmt.Sprintf("조회 조건에 해당되는 상품 정보가 변경되었습니다.\n\n%s\n\n%s", searchConditionsSummary, diffMessage), true
	}

	// 스케줄러(Scheduler)에 의한 자동 실행이 아닌, 사용자 요청에 의한 수동 실행인 경우입니다.
	//
	// 자동 실행 시에는 변경 사항이 없으면 불필요한 알림(Noise)을 방지하기 위해 침묵하지만,
	// 수동 실행 시에는 "변경 없음"이라는 명시적인 피드백을 제공하여 시스템이 정상 동작 중임을 사용자가 인지할 수 있도록 합니다.
	if t.RunBy() == contract.TaskRunByUser {
		searchConditionsSummary := renderSearchConditionsSummary(commandSettings)

		if len(currentSnapshot.Products) == 0 {
			return fmt.Sprintf("조회 조건에 해당되는 상품이 존재하지 않습니다.\n\n%s", searchConditionsSummary), false
		}

		return fmt.Sprintf("조회 조건에 해당되는 상품의 변경된 정보가 없습니다.\n\n%s\n\n조회 조건에 해당되는 상품은 아래와 같습니다:\n\n%s", searchConditionsSummary, renderCurrentStatus(currentSnapshot, supportsHTML)), false
	}

	return "", false
}
