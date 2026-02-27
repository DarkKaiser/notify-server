package kurly

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	applog "github.com/darkkaiser/notify-server/pkg/log"
)

// executeWatchProductPrice 감시 대상 상품 목록을 읽어 최신 가격을 수집하고,
// 이전 실행 시점의 스냅샷과 비교하여 변경 사항이 있을 경우 사용자에게 전달할 알림 메시지를 생성합니다.
//
// 매개변수:
//   - ctx: 요청 취소 등을 위한 컨텍스트
//   - loader: 감시 대상 상품 목록을 읽어오는 추상화된 로더
//   - prevSnapshot: 이전 실행 시 저장된 상품 목록 스냅샷 (nil이면 최초 실행)
//   - supportsHTML: 알림 수신 채널의 HTML 지원 여부
//
// 반환값:
//   - message: 사용자에게 전송할 알림 메시지 (없으면 빈 문자열)
//   - newSnapshot: 변경사항이 감지된 경우 새로 수집된 상품 목록 스냅샷, 없으면 nil
//   - err: 실행 중 발생한 오류
func (t *task) executeWatchProductPrice(ctx context.Context, loader WatchListLoader, prevSnapshot *watchProductPriceSnapshot, supportsHTML bool) (string, any, error) {
	// =========================================================================
	// [1단계] 감시 대상 상품 목록 로드
	// =========================================================================

	// 추상화된 Loader를 통해 감시 대상 상품 목록을 읽어옵니다.
	records, err := loader.Load()
	if err != nil {
		return "", nil, err
	}

	// 감시 대상 상품 목록에서 중복 상품을 분리합니다.
	// 고유 목록(records)과 중복 목록(duplicateRecords)으로 나누어, 이후 단계에서 중복 알림 전송 여부를 별도로 처리합니다.
	records, duplicateRecords := separateDuplicateRecords(records)

	// =========================================================================
	// [2단계] 상품 정보 병렬 수집
	// =========================================================================

	t.Log(component, applog.InfoLevel, "상품 정보 수집 시작: 감시 대상 상품 로드 완료", nil, applog.Fields{
		"total_count":     len(records) + len(duplicateRecords),
		"target_count":    len(records),
		"duplicate_count": len(duplicateRecords),
	})

	// 상품 정보 병렬 수집 작업의 전체 소요 시간을 측정하기 위해 시작 시간을 기록합니다.
	startTime := time.Now()

	// 네트워크 응답 순서에 의존하지 않고 원본 레코드 순서를 그대로 보장하기 위해 원본 레코드 개수만큼의 공간을 미리 배열로 할당합니다.
	// 각 고루틴은 자신에게 배정된 인덱스에만 독립적으로 결과를 적재합니다.
	currentSnapshot := &watchProductPriceSnapshot{
		Products: make([]*product, len(records)),
	}

	var wg sync.WaitGroup

	// 고루틴 수집 루프 도중 취소 신호가 감지되면, 즉시 return하지 않고 이 변수에 에러를 저장한 뒤 break로 루프만 탈출합니다.
	// 이렇게 하면 루프 하단의 wg.Wait()를 반드시 거치게 되어, 이미 실행 중인 고루틴들이
	// 모두 정상 종료된 후 에러를 반환할 수 있습니다. (고아 고루틴 누수 방지)
	var executionErr error

	// 동시에 처리할 최대 고루틴 수를 제한하는 세마포어입니다.
	// 마켓컬리 서버 과부하 및 지속적인 연결 제한(차단)을 방지하기 위한 안전 장치입니다.
	sem := make(chan struct{}, 5)

fetchLoop:
	for i, record := range records {
		// 고루틴 클로저가 루프 변수를 참조할 때 마지막 반복 값만 공유하는 동시성 문제를 방지합니다.
		// 각 고루틴이 고유한 인덱스(i)와 데이터(record)를 갖도록 변수를 섀도잉(Shadowing)하여 독립적인 상태를 보장합니다.
		i := i
		record := record

		// 비활성화된 상품(status != "1")은 수집 대상에서 제외합니다.
		if record[columnStatus] != statusEnabled {
			continue
		}

		// 고루틴 예약 전, 사용자 취소 요청이 있으면 즉시 중단합니다.
		if t.IsCanceled() {
			t.Log(component, applog.InfoLevel, "상품 정보 수집 예약 중단: 사용자 작업 취소 요청 감지", nil, applog.Fields{
				"scheduled_count":    i,
				"total_count":        len(records),
				"pending_product_id": record[columnID],
			})

			executionErr = context.Canceled

			break fetchLoop
		}

		// 세마포어에 토큰을 획득하여 동시 처리 수를 제한합니다.
		// 컨텍스트가 취소된 경우 sem 채널 대기를 건너뛰고 즉시 반환합니다.
		select {
		case <-ctx.Done():
			t.Log(component, applog.InfoLevel, "상품 정보 수집 예약 중단: 컨텍스트 취소 감지", nil, applog.Fields{
				"scheduled_count":    i,
				"total_count":        len(records),
				"pending_product_id": record[columnID],
			})

			executionErr = ctx.Err()

			break fetchLoop

		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			// 고루틴 종료 시(정상/취소/에러 무관) sem 채널에서 토큰을 꺼내 다음 고루틴이 진입할 수 있도록 합니다.
			defer func() { <-sem }()

			// 자식 고루틴 내부에서 발생하는 패닉을 복구하여, 전체 서버가 다운(Crash)되는 것을 방지합니다.
			// 문제 발생 시 해당 상품의 단순 수집 실패로 격리(Localizing)하여 다른 정상 상품들의 처리를 계속 진행합니다.
			defer func() {
				if r := recover(); r != nil {
					t.Log(component, applog.ErrorLevel, "임시 실패 처리: 상품 정보 수집 중 런타임 패닉 발생", fmt.Errorf("panic: %v", r), applog.Fields{
						"raw_product_id": record[columnID],
						"product_name":   record[columnName],
						"row_index":      i,
						"panic":          r,
					})

					id, err := strconv.Atoi(record[columnID])
					if err != nil {
						// 패닉이 발생했음에도 record[columnID] 마저 정수로 변환할 수 없다면,
						// 레코드 자체가 근본적으로 손상되었거나 비어있는 상태입니다.
						// ID가 없는 상품은 이후 단계에서 처리 기준이 없어 의미 없는 데이터가 됩니다.
						// 따라서 임시 실패 객체를 생성하지 않고 조용히 종료하여 슬롯을 nil로 유지합니다.
						// (nil 슬롯은 3단계 결과 정리 단계에서 자동으로 필터링됩니다.)
						return
					}

					// 수집 실패 1건 때문에 다른 모든 상품의 수집이 멈추지 않도록, 임시 실패 객체를 대신 저장합니다.
					// 이 객체는 다음 단계(analyzer)에서 '연속 실패 횟수'를 계산하여 단종(판매 중단) 여부를 판단하는 데 쓰입니다.
					currentSnapshot.Products[i] = &product{
						ID:               id,
						Name:             record[columnName],
						FetchFailedCount: 1,
					}
				}
			}()

			// 세마포어 대기 중 취소 신호가 도달했을 수 있으므로, fetchProduct 호출 전에 재확인합니다.
			// 이미 루프에서 취소 로그를 남겼으므로, 여기서 다시 로그를 남기면 실행 중인 고루틴 수만큼 중복 출력됩니다.
			// 따라서 로그 출력 없이 종료하여, 이후 fetchProduct 호출을 막습니다.
			if t.IsCanceled() || ctx.Err() != nil {
				return
			}

			// 상품 코드(문자열)를 정수(int)로 변환합니다.
			// 잘못된 데이터 등으로 변환이 실패해도 다른 상품 수집을 이어갈 수 있도록,
			// 에러를 상위로 전파하지 않고 로그만 남긴 뒤 해당 상품만 건너뜁니다.
			id, err := strconv.Atoi(record[columnID])
			if err != nil {
				t.Log(component, applog.ErrorLevel, "상품 수집 건너뜀: 상품 ID 숫자 변환 실패", err, applog.Fields{
					"raw_product_id": record[columnID],
					"product_name":   record[columnName],
					"row_index":      i,
				})

				return
			}

			// 상품의 상세 페이지를 스크래핑하여, 현재 판매 상태와 가격 정보를 추출합니다.
			fetchedProduct, err := t.fetchProduct(ctx, id)
			if err != nil {
				t.Log(component, applog.ErrorLevel, "임시 실패 처리: 상품 상세 페이지 데이터 추출 오류", err, applog.Fields{
					"product_id":   id,
					"product_name": record[columnName],
					"row_index":    i,
				})

				// 수집 실패 1건 때문에 다른 모든 상품의 수집이 멈추지 않도록, 임시 실패 객체를 대신 저장합니다.
				// 이 객체는 다음 단계(analyzer)에서 '연속 실패 횟수'를 계산하여 단종(판매 중단) 여부를 판단하는 데 쓰입니다.
				currentSnapshot.Products[i] = &product{
					ID:               id,
					Name:             record[columnName],
					FetchFailedCount: 1,
				}

				return
			}

			// 락(Lock) 없이 고루틴별로 사전에 할당받은 자기 인덱스에만 수집 결과를 저장합니다.
			// 이를 통해 병목 현상 없이, 원본 상품 목록의 순서를 그대로 유지할 수 있습니다.
			currentSnapshot.Products[i] = fetchedProduct
		}()
	}

	// 병렬로 실행 중인 모든 상품 수집 작업이 완료될 때까지 대기합니다.
	wg.Wait()

	// 루프 중간에 취소 신호를 받았더라도, wg.Wait() 이전에 즉시 return하지 않습니다.
	// 이미 고루틴이 시작된 상품이 있다면, 중도에 abandonation(버림)하면 고아 고루틴(Orphan Goroutine)이 발생합니다.
	// 따라서 wg.Wait()으로 모든 고루틴이 완전히 종료될 때까지 기다린 뒤, 이 시점에서 에러를 반환합니다.
	if executionErr != nil {
		return "", nil, executionErr
	}

	// =========================================================================
	// [3단계] 수집 결과 정리
	// =========================================================================

	// 추출 실패 또는 비활성화로 인해 비어있는 데이터(nil)를 걸러내고,
	// 정상 처리된 유효 상품만 추려내면서 동시에 이번 작업의 성공 및 실패 건수를 집계합니다.
	var succeededCount, failedCount int
	var validProducts = make([]*product, 0, len(currentSnapshot.Products))
	for _, p := range currentSnapshot.Products {
		if p != nil {
			validProducts = append(validProducts, p)

			if p.FetchFailedCount > 0 {
				failedCount++
			} else {
				succeededCount++
			}
		}
	}
	currentSnapshot.Products = validProducts

	t.Log(component, applog.InfoLevel, "상품 정보 수집 종료: 전체 수집 작업 완료", nil, applog.Fields{
		"total_count":     len(records) + len(duplicateRecords),
		"target_count":    len(records),
		"duplicate_count": len(duplicateRecords),
		"success_count":   succeededCount,
		"fail_count":      failedCount,
		"elapsed_time":    time.Since(startTime).String(),
	})

	// 다음 단계로 넘어가기 전 취소 요청 여부를 한 번 더 확인합니다.
	if t.IsCanceled() {
		t.Log(component, applog.InfoLevel, "수집 결과 병합 중단: 사용자 작업 취소 요청 감지", nil, nil)
		return "", nil, context.Canceled
	}

	// =========================================================================
	// [4단계] 이전 스냅샷 병합
	// =========================================================================

	// 현재 감시 대상 상품 목록에서 '감시 활성화' 상태인 상품 ID만 추려 Set(집합)을 구성합니다.
	// 고유 레코드 목록(records)과 중복 레코드 목록(duplicateRecords) 모두 포함합니다.
	//
	// 이 집합은 이후 mergeWithPreviousState의 이월/GC 기준으로 사용됩니다.
	// 즉, 감시 대상 상품 목록에서 제거되었거나 비활성화된 상품의 이전 스냅샷 데이터를
	// 현재 스냅샷에서 자동으로 제거(GC)하는 판단 근거가 됩니다.
	watchedProductIDs := make(map[int]struct{})
	for _, record := range records {
		if record[columnStatus] == statusEnabled {
			if id, err := strconv.Atoi(record[columnID]); err == nil {
				watchedProductIDs[id] = struct{}{}
			}
		}
	}
	for _, record := range duplicateRecords {
		if record[columnStatus] == statusEnabled {
			if id, err := strconv.Atoi(record[columnID]); err == nil {
				watchedProductIDs[id] = struct{}{}
			}
		}
	}

	// 이전 스냅샷(prevSnapshot)을 기준으로 현재 수집 결과를 보정합니다.
	//
	// 구체적으로 다음 세 가지 작업을 순서대로 수행합니다.
	//  1. 상태 복원: 이번 사이클에 새로 수집된 객체는 Stateless 상태입니다.
	//              이전 스냅샷에서 역대 최저가·연속 실패 횟수 등 누적 데이터를 현재 객체로 이월합니다.
	//  2. 최저가 갱신: 복원된 최저가와 현재 수집 가격을 비교하여 더 낮은 값으로 갱신합니다.
	//  3. 이월 및 GC: 이번 수집에서 누락된 상품 중, 아직 활성 상태(watchedProductIDs에 포함)인 것은 일시적 실패로 간주하여 스냅샷에 보존하고,
	//              삭제·비활성화된 것은 스냅샷에서 제거합니다. (Ghost 데이터 방지)
	//
	// 이 단계는 5단계 분석(Analyzer)보다 반드시 먼저 실행되어야 하며, 상태 변이를
	// 분석 코드와 분리하여 분석 로직이 항상 '완성된 데이터'를 바라보도록 보장합니다.
	mergedProducts, prevProductsByID := mergeWithPreviousState(currentSnapshot.Products, prevSnapshot, watchedProductIDs)

	// 병합이 완료된 상품 목록으로 현재 스냅샷의 상품 목록을 교체합니다.
	currentSnapshot.Products = mergedProducts

	// =========================================================================
	// [5단계] 변경 사항 분석 및 결과 반환
	// =========================================================================

	// 중복 등록 상품에 대한 알림 중복 발송을 방지하기 위한 State-Machine의 이전 상태값을 복원합니다.
	// extractNewDuplicateRecords에 전달하여, 이미 알림을 보낸 상품은 이번 사이클에서 재발송하지 않도록 합니다.
	// 최초 실행(prevSnapshot == nil)인 경우 이전 발송 이력이 없으므로 빈 슬라이스로 초기화합니다.
	var prevDuplicateNotifiedIDs []string
	if prevSnapshot != nil {
		prevDuplicateNotifiedIDs = prevSnapshot.DuplicateNotifiedIDs
	}

	// -------------------------------------------------------------------------
	// 5-1. 순수 분석 — 데이터 비교 및 이벤트 식별 (Side-effect 없음)
	// -------------------------------------------------------------------------
	// 세 가지 이벤트를 독립적으로 계산합니다. 이 단계는 상태를 '읽기'만 하며 변경하지 않습니다.
	//
	//  · extractProductDiffs : 이번 회차 상품 목록과 직전 스냅샷을 비교하여
	//                          신규 등장·재입고·가격 변동·역대 최저가 등 사용자에게 전달할 변화(Diff) 목록을 생성합니다.
	//
	//  · extractNewDuplicateRecords : 감시 상품 목록에서 중복 등록된 상품을 스캔하여,
	//                                 이미 알림을 보낸 상품(prevDuplicateNotifiedIDs)은 걸러내고
	//                                 이번 회차에 새롭게 중복이 감지된 목록만 추출합니다.
	//                                 (State-Machine 기반 중복 알림 스팸 방지)
	//
	//  · extractNewlyUnavailableProducts : '판매 중 → 단종/품절'로 새롭게 상태가 바뀐 상품만 추출합니다.
	//                                      처음부터 단종 상태였거나 이전에도 이미 단종이었던 상품은 중복 알림을 막기 위해 제외됩니다.
	productDiffs := extractProductDiffs(currentSnapshot, prevProductsByID)
	newDuplicateRecords, updatedDuplicateNotifiedIDs := extractNewDuplicateRecords(duplicateRecords, prevDuplicateNotifiedIDs)
	newlyUnavailableProducts := extractNewlyUnavailableProducts(currentSnapshot.Products, prevProductsByID, records)

	// -------------------------------------------------------------------------
	// 5-2. 알림 메시지 렌더링 — 분석 결과 → 텍스트 변환 (순수 포맷팅)
	// -------------------------------------------------------------------------
	// 위 분석 결과를 각각 알림 채널에 맞는 문자열로 포맷팅합니다.
	// supportsHTML 플래그에 따라 HTML 마크업 포함 여부가 달라지며, 각 렌더러는 입력 데이터가 없으면
	// 빈 문자열을 반환하여 최종 조합 단계에서 자연스럽게 생략됩니다.
	productDiffsMessage := renderProductDiffs(productDiffs, supportsHTML)
	newDuplicateRecordsMessage := renderDuplicateRecords(newDuplicateRecords, supportsHTML)
	newlyUnavailableProductsMessage := renderUnavailableProducts(newlyUnavailableProducts, supportsHTML)

	// -------------------------------------------------------------------------
	// 5-3. 상태 반영 — 분석 결과를 현재 스냅샷에 기록 (State Mutation)
	// -------------------------------------------------------------------------
	// extractNewDuplicateRecords가 산출한 '이번 회차까지 중복 알림이 발송된 상품 ID 목록'을 현재 스냅샷에 저장합니다.
	// 이 값은 다음 실행 시 prevDuplicateNotifiedIDs로 전달되어 동일 상품에 대한 중복 알림 재발송을 방지하는
	// State-Machine의 기억(Memory)으로 활용됩니다.
	currentSnapshot.DuplicateNotifiedIDs = updatedDuplicateNotifiedIDs

	// -------------------------------------------------------------------------
	// 5-4. 최종 알림 메시지 조합
	// -------------------------------------------------------------------------
	// 위 단계에서 생성된 세 종류의 부분 메시지(가격 변동·중복 등록·단종)를 하나의 알림 메시지로 합칩니다.
	message := buildNotificationMessage(t.RunBy(), currentSnapshot, productDiffsMessage, newDuplicateRecordsMessage, newlyUnavailableProductsMessage, supportsHTML)

	// -------------------------------------------------------------------------
	// 5-5. 스냅샷 저장 필요 여부 판단 — HasChanged (Deep Compare)
	// -------------------------------------------------------------------------
	// 현재 스냅샷과 이전 스냅샷을 깊게(Deep) 비교하여 내부 상태가 하나라도 바뀌었는지 확인합니다.
	// HasChanged는 다음 세 항목을 순서대로 검사합니다.
	//   1) 중복 알림 발송 이력(DuplicateNotifiedIDs) 변경 여부
	//   2) 상품 수(len) 변경 여부
	//   3) 개별 상품 필드(가격·할인율·최저가·품절 여부·수집 실패 횟수 등) 변경 여부
	hasChanged := currentSnapshot.HasChanged(prevSnapshot)

	// -------------------------------------------------------------------------
	// [경로 A] 변경 사항 있음 (hasChanged == true) → 스냅샷 저장 후 반환
	// -------------------------------------------------------------------------
	// '메시지 유무'와 무관하게 반드시 스냅샷을 저장해야 합니다.
	//
	// 상태는 바뀌었지만 알림 메시지가 없는 경우에도 저장을 건너뛰면 State-Machine 동기화 오류가 발생합니다.
	//   - 다음 실행에서 '변경된 상태'를 또다시 '새로운 변화'로 감지하여 알림이 중복 전송됩니다.
	//   - 삭제된 상품의 Ghost 데이터가 스토리지에 잔류하여 불필요한 알림의 원인이 됩니다.
	//
	// 따라서 메시지가 없을 때는 로그만 남기고 currentSnapshot을 반환하여 호출부가 알림 없이 스냅샷만 저장하도록 유도합니다.
	if hasChanged {
		if message == "" {
			var prevProductCount, prevDuplicateNotifiedCount int
			if prevSnapshot != nil {
				prevProductCount = len(prevSnapshot.Products)
				prevDuplicateNotifiedCount = len(prevSnapshot.DuplicateNotifiedIDs)
			}

			t.Log(component, applog.InfoLevel, "알림 발송 생략 및 상태 갱신: 알림 조건 미충족", nil, applog.Fields{
				"is_initial_run":             prevSnapshot == nil,
				"current_product_count":      len(currentSnapshot.Products),
				"prev_product_count":         prevProductCount,
				"current_duplicate_notified": len(currentSnapshot.DuplicateNotifiedIDs),
				"prev_duplicate_notified":    prevDuplicateNotifiedCount,
			})
		}

		return message, currentSnapshot, nil
	}

	// -------------------------------------------------------------------------
	// [경로 B] 변경 사항 없음 (hasChanged == false) → 저장 생략 후 반환
	// -------------------------------------------------------------------------
	// 이전 스냅샷과 완전히 동일하므로 스토리지 쓰기가 불필요합니다.
	// 두 번째 반환값을 nil로 설정하여 호출부에 "스냅샷 갱신 불필요" 신호를 전달합니다.
	return message, nil, nil
}
