package kurly

import (
	"context"
	"strconv"

	"sync"

	applog "github.com/darkkaiser/notify-server/pkg/log"
)

// @@@@@
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
		// 네트워크 응답 순서에 의존하지 않고 CSV 원본 순서를 그대로 보장하기 위해
		// 원본 레코드 개수만큼의 크기로 미리 배열("방")을 할당(Pre-allocate)합니다.
		Products: make([]*product, len(records)),
	}

	var wg sync.WaitGroup
	// 한 번에 최대 처리할 병렬 수 지정 (대상 사이트 상태 및 API 보호 고려)
	sem := make(chan struct{}, 5)

	for i, record := range records {
		// 고루틴 클로저 바인딩용
		i := i
		record := record

		// 작업 취소 여부 확인 (스케줄링 전단의 빠른 취소)
		if t.IsCanceled() {
			t.Log("task.kurly", applog.WarnLevel, "작업 취소 요청이 감지되어 상품 정보 수집 프로세스를 중단합니다", nil, nil)
			return "", nil, nil
		}

		if record[columnStatus] != statusEnabled {
			continue
		}

		select {
		case <-ctx.Done():
			t.Log("task.kurly", applog.WarnLevel, "병렬 처리 루프 중 컨텍스트 취소가 감지되어 예약을 중단합니다", nil, nil)
			return "", nil, nil
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }() // 작업 완료 후 세마포어 반환

			// 작업 취소 여부 확인 (실제 fetch 돌입 전 취소 검사)
			if t.IsCanceled() || ctx.Err() != nil {
				return
			}

			// 상품 코드를 숫자로 변환한다.
			id, err := strconv.Atoi(record[columnID])
			if err != nil {
				// [High] 부분 실패 격리: 데이터 오기입(문자열 포함 등)으로 인해 단일 식별자 파싱이 실패하더라도
				// 전체 감시 루프(다른 정상 상품들)가 중단되지 않도록 에러를 반환하지 않고 로깅만 수행한 뒤 건너뜁니다.
				t.Log("task.kurly", applog.ErrorLevel, "상품 코드 숫자 변환 실패 (건너뜀)", err, applog.Fields{
					"raw_product_id": record[columnID],
					"product_name":   record[columnName],
				})
				return
			}

			// 상품 페이지를 읽어들이고 파싱하여 정보를 추출한다.
			// 상세 페이지에서 상품 정보를 조회 (Fetch + Parse)
			// Context는 상위 원본 ctx를 그대로 사용하여, 다른 고루틴의 에러에 영향을 받지 않는 독립적인 생명주기를 가집니다.
			fetchedProduct, err := t.fetchProductInfo(ctx, id)
			if err != nil {
				// [High] 부분 실패 격리: 단일 상품 조회 실패가 전체 감시 루프를 중단시키지 않도록
				// 임시 실패 객체를 생성하여 배열에 적재합니다. (이후 analyzer에서 3회 이상 실패 시 단종 처리)
				t.Log("task.kurly", applog.ErrorLevel, "개별 상품 정보 수집 실패 (임시 실패 객체로 대체)", err, applog.Fields{
					"product_id": id,
				})

				currentSnapshot.Products[i] = &product{
					ID:               id,
					Name:             record[columnName],
					FetchFailedCount: 1,
				}
				return
			}

			// 각각의 고루틴은 뮤텍스(Mutex) 잠금이나 append 없이 자신에게 할당된 인덱스 방(`i`)에만 접근하여
			// 수집 결과를 적재합니다. 이를 통해 동시성 병목(Contention)을 해소하고 원본 레코드의 순서를 100% 보장합니다.
			currentSnapshot.Products[i] = fetchedProduct

		}()
	}

	// 모든 비동기 조회 고루틴의 작업이 완료되기를 대기합니다.
	wg.Wait()

	// 배열 압축 (Compact):
	// 비활성화(`status != "1"`)된 레코드나 파싱 실패, 통신 실패 등으로 인해 수집되지 못한 '빈 공간(nil)'을
	// 모두 솎아내고, 정상 수집 완료된 순도 100%의 상품들만으로 연속된 배열을 다시 구성합니다.
	actualProducts := make([]*product, 0, len(currentSnapshot.Products))
	for _, p := range currentSnapshot.Products {
		if p != nil {
			actualProducts = append(actualProducts, p)
		}
	}
	currentSnapshot.Products = actualProducts

	// 취소되었는지 다시 확인 (대기 중에 취소될 수 있음)
	if t.IsCanceled() {
		t.Log("task.kurly", applog.WarnLevel, "작업 취소 요청이 감지되어 합성을 완료하지 못하고 중단합니다", nil, nil)
		return "", nil, nil
	}

	// 가비지 컬렉션(GC)을 위해 현재 감시 대상(활성화 상태)인 상품 ID 목록을 수집합니다.
	activeRecordIDs := make(map[int]struct{})
	for _, record := range records {
		if record[columnStatus] == statusEnabled {
			if id, err := strconv.Atoi(record[columnID]); err == nil {
				activeRecordIDs[id] = struct{}{}
			}
		}
	}
	for _, record := range duplicateRecords {
		if record[columnStatus] == statusEnabled {
			if id, err := strconv.Atoi(record[columnID]); err == nil {
				activeRecordIDs[id] = struct{}{}
			}
		}
	}

	// 이전 스냅샷의 정보를 바탕으로 현재 수집된 상품들의 최저가 정보를 최신화합니다.
	prevProductsMap := syncLowestPrices(currentSnapshot, prevSnapshot, activeRecordIDs)

	// 이전 스냅샷에 기록되어 있던 중복 알림 전송 목록
	var prevDuplicateNotifiedIDs []string
	if prevSnapshot != nil {
		prevDuplicateNotifiedIDs = prevSnapshot.DuplicateNotifiedIDs
	}

	// 동기화된 데이터를 바탕으로 변경 사항을 감지하고 알림 메시지를 생성합니다.
	message, shouldSave := analyzeAndReport(t.RunBy(), currentSnapshot, prevProductsMap, prevDuplicateNotifiedIDs, records, duplicateRecords, supportsHTML)

	if shouldSave {
		// "변경 사항이 있다면(shouldSave=true), 반드시 알림 메시지 유무와 상관없이" 스냅샷은 저장해야 합니다.
		// 만약 상태는 변했으나(예: 중복 해제) 파생될 메시지가 없는 상황일 경우 `return "", nil, nil` 로 강제 종료해버리면,
		// 변경된 상태(State) 영속화가 진행되지 않아 State-Machine이 과거에 멈추는 버그가 발생하게 됩니다.
		if message == "" {
			t.Log("task.kurly", applog.InfoLevel, "변경 사항 감지되어 상태 저장을 진행하지만, 알림 생성 조건에 부합하지 않아 메시지 전송은 생략합니다", nil, nil)
		}

		return message, currentSnapshot, nil
	}

	return message, nil, nil
}

// @@@@@
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
		if len(record) <= int(columnID) {
			continue
		}

		productID := record[columnID]
		if _, exists := seenProductIDs[productID]; !exists {
			seenProductIDs[productID] = struct{}{}
			distinctRecords = append(distinctRecords, record)
		} else {
			duplicateRecords = append(duplicateRecords, record)
		}
	}

	return distinctRecords, duplicateRecords
}
