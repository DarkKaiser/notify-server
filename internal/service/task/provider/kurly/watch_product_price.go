package kurly

import (
	"context"
	"strconv"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
)

// executeWatchProductPrice 감시 대상 상품들의 최신 가격을 조회하고, 이전 상태와 비교하여 변동이 있으면 알림을 생성합니다.
func (t *task) executeWatchProductPrice(ctx context.Context, loader WatchListLoader, prevSnapshot *watchProductPriceSnapshot, supportsHTML bool) (message string, changedTaskResultData interface{}, err error) {
	// @@@@@
	//
	// 감시할 상품 목록을 읽어들인다. (추상화된 Loader 사용)
	//
	records, err := loader.Load()
	if err != nil {
		return "", nil, err
	}

	// 감시할 상품 목록에서 중복된 상품을 정규화한다.
	records, duplicateRecords := t.extractDuplicateRecords(records)

	//
	// 읽어들인 상품들의 가격 및 상태를 확인한다.
	//
	currentSnapshot := &watchProductPriceSnapshot{
		Products: make([]*product, 0, len(records)),
	}

	for _, record := range records {
		// 작업 취소 여부 확인
		if t.IsCanceled() {
			t.Log("task.kurly", applog.WarnLevel, "작업 취소 요청이 감지되어 상품 정보 수집 프로세스를 중단합니다", nil, nil)
			return "", nil, nil
		}

		if record[csvColumnStatus] != csvStatusEnabled {
			continue
		}

		// 상품 코드를 숫자로 변환한다.
		id, err := strconv.Atoi(record[csvColumnID])
		if err != nil {
			return "", nil, apperrors.Wrap(err, apperrors.InvalidInput, "상품 코드의 숫자 변환이 실패하였습니다")
		}

		// 상품 페이지를 읽어들이고 파싱하여 정보를 추출한다.
		// 상세 페이지에서 상품 정보를 조회 (Fetch + Parse)
		product, err := t.fetchProductInfo(ctx, id)
		if err != nil {
			return "", nil, err
		}

		currentSnapshot.Products = append(currentSnapshot.Products, product)
	}

	// 이전 스냅샷의 정보를 바탕으로 현재 수집된 상품들의 최저가 정보를 최신화합니다.
	prevProductsMap := syncLowestPrices(currentSnapshot, prevSnapshot)

	// 동기화된 데이터를 바탕으로 변경 사항을 감지하고 알림 메시지를 생성합니다.
	message, shouldSave := analyzeAndReport(t.RunBy(), currentSnapshot, prevProductsMap, records, duplicateRecords, supportsHTML)

	if shouldSave {
		// "변경 사항이 있다면(shouldSave=true), 반드시 알림 메시지도 존재해야 한다"는 규칙을 확인합니다.
		// 만약 메시지 없이 데이터만 갱신되면, 사용자는 변경 사실을 영영 모르게 될 수 있습니다.
		// 이를 방지하기 위해, 이런 비정상적인 상황에서는 저장을 차단하고 즉시 로그를 남깁니다.
		if message == "" {
			t.Log("task.kurly", applog.WarnLevel, "변경 사항 감지 후 저장 프로세스를 시도했으나, 알림 메시지가 비어있습니다 (저장 건너뜀)", nil, nil)
			return "", nil, nil
		}

		return message, currentSnapshot, nil
	}

	return message, nil, nil
}

// extractDuplicateRecords 입력된 감시 대상 상품(레코드) 목록에서 중복 기입된 항목을 추출하여 분리합니다.
//
// [설명]
// 감시 대상 상품(레코드) 목록을 순회하며 상품 ID를 기준으로 중복 여부를 검사합니다.
// 처음 등장하는 상품은 `distinctRecords`에 담고, 이미 등장한 상품은 `duplicateRecords`로 추출합니다.
// 이를 통해 핵심 로직에서는 중복 없는 깨끗한 데이터만 처리할 수 있게 됩니다.
//
// [매개변수]
//   - records: CSV 파일에서 읽어온 원본 감시 대상 상품(레코드) 목록입니다.
//
// [반환값]
//   - distinctRecords: 중복이 제거된 유일한 상품(레코드) 목록입니다.
//   - duplicateRecords: 중복으로 판명되어 추출된 상품(레코드) 목록입니다.
func (t *task) extractDuplicateRecords(records [][]string) ([][]string, [][]string) {
	distinctRecords := make([][]string, 0, len(records))
	duplicateRecords := make([][]string, 0, len(records)/2) // 중복 빈도를 고려하여 초기 용량 절반 할당

	// 메모리 효율성을 위해 빈 구조체 사용
	seenProductIDs := make(map[string]struct{}, len(records))

	for _, record := range records {
		// 필수 컬럼(상품 번호) 존재 여부 확인
		if len(record) <= int(csvColumnID) {
			continue
		}

		productID := record[csvColumnID]
		if _, exists := seenProductIDs[productID]; !exists {
			seenProductIDs[productID] = struct{}{}
			distinctRecords = append(distinctRecords, record)
		} else {
			duplicateRecords = append(duplicateRecords, record)
		}
	}

	return distinctRecords, duplicateRecords
}
