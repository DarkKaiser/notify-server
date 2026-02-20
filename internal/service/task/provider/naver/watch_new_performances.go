package naver

import (
	"context"
	"errors"
	"time"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutil"
)

// keywordMatchers 필터 설정을 반복 사용에 최적화된 Matcher 객체로 변환하여 캐시합니다.
//
// 설정에서 읽은 콤마 구분 키워드 문자열을 한 번만 파싱하여
// 공연마다 반복적으로 키워드 매칭을 수행할 때 사용됩니다.
type keywordMatchers struct {
	// TitleMatcher 공연 제목 필터링에 사용되는 Matcher입니다.
	TitleMatcher *strutil.KeywordMatcher

	// PlaceMatcher 공연 장소 필터링에 사용되는 Matcher입니다.
	PlaceMatcher *strutil.KeywordMatcher
}

// executeWatchNewPerformances 네이버에서 신규 공연 정보를 확인하고 변경 사항을 알립니다.
//
// 매개변수:
//   - ctx: 요청 취소 등을 위한 컨텍스트
//   - commandSettings: 검색어, 필터, 최대 페이지 수 등 작업 설정
//   - prevSnapshot: 이전 실행 시 저장된 공연 목록 스냅샷 (nil이면 최초 실행)
//   - supportsHTML: 알림 수신 채널의 HTML 지원 여부
//
// 반환값:
//   - message: 사용자에게 전송할 알림 메시지 (없으면 빈 문자열)
//   - newSnapshot: 변경사항이 감지된 경우 새로 수집된 공연 목록 스냅샷, 없으면 nil
//   - err: 실행 중 발생한 오류
func (t *task) executeWatchNewPerformances(ctx context.Context, commandSettings *watchNewPerformancesSettings, prevSnapshot *watchNewPerformancesSnapshot, supportsHTML bool) (message string, newSnapshot any, err error) {
	// 1단계: 설정된 조건(검색어, 필터 등)을 기반으로 네이버에서 최신 공연 목록을 수집합니다.
	// 수집 결과는 직전 스냅샷과의 비교 대상으로 활용됩니다.
	currentPerformances, err := t.fetchPerformances(ctx, commandSettings)
	if err != nil {
		return "", nil, err
	}

	currentSnapshot := &watchNewPerformancesSnapshot{
		Performances: currentPerformances,
	}

	// 2단계: 수집된 현재 스냅샷을 직전 스냅샷과 비교하여 변경 여부를 판단하고 알림 메시지를 구성합니다.
	//
	// 내부적으로 신규 공연 감지, 삭제 감지, 내용 변경(예: 썸네일) 감지를 수행하며,
	// 그 결과를 message(알림 메시지)와 hasChanges(스냅샷 갱신 필요 여부)로 분리하여 반환합니다.
	message, hasChanges := t.analyzeAndReport(currentSnapshot, prevSnapshot, supportsHTML)

	// 3단계: 스냅샷 갱신 여부에 따라 결과를 반환합니다.
	//
	// [변경 사항이 있는 경우 (hasChanges == true)]
	// 신규 공연 발견, 공연 삭제, 또는 내용 변경 중 하나 이상이 감지된 상태입니다.
	// 다음 실행 시 정확한 비교 기준이 될 수 있도록 현재 스냅샷(currentSnapshot)을 함께 반환합니다.
	// 반환된 currentSnapshot은 호출부에서 데이터베이스에 저장됩니다.
	if hasChanges {
		return message, currentSnapshot, nil
	}

	// [변경 사항이 없는 경우 (hasChanges == false)]
	// 스냅샷 갱신이 불필요하므로 nil을 반환하여 저장을 건너뜁니다.
	// analyzeAndReport 내부에서 수동 실행(TaskRunByUser) 시 현재 공연 목록을 message에 담아두므로,
	// 여기서는 그 값을 그대로 전달합니다. (스케줄러 실행 시에는 message가 빈 문자열입니다.)
	return message, nil, nil
}

// fetchPerformances 네이버 모바일 검색 페이지를 페이지 단위로 호출하여 조건에 맞는 공연 목록을 수집합니다.
//
// 매개변수:
//   - ctx: 요청 취소 등을 위한 컨텍스트
//   - commandSettings: 검색어, 필터, 최대 페이지 수 등 작업 설정
//
// 반환값:
//   - []*performance: 필터링 및 중복 제거를 거친 공연 목록
//   - error: 수집 중 발생한 오류 (작업 취소 시 context.Canceled)
func (t *task) fetchPerformances(ctx context.Context, commandSettings *watchNewPerformancesSettings) ([]*performance, error) {
	// =========================================================================
	// [초기화] 키워드 필터 Matcher 준비
	// =========================================================================

	// 필터 설정(콤마로 구분된 문자열)을 Matcher 객체로 변환하여 캐싱합니다.
	// API 응답의 각 공연 데이터를 순회할 때마다 매번 파싱하지 않기 위함입니다.
	matchers := &keywordMatchers{
		TitleMatcher: strutil.NewKeywordMatcher(
			strutil.SplitClean(commandSettings.Filters.Title.IncludedKeywords, ","),
			strutil.SplitClean(commandSettings.Filters.Title.ExcludedKeywords, ","),
		),
		PlaceMatcher: strutil.NewKeywordMatcher(
			strutil.SplitClean(commandSettings.Filters.Place.IncludedKeywords, ","),
			strutil.SplitClean(commandSettings.Filters.Place.ExcludedKeywords, ","),
		),
	}

	// =========================================================================
	// [초기화] 페이지 요청 간격 타이머 준비
	// =========================================================================

	// 페이지 수집 간격을 조절하기 위한 타이머를 생성합니다.
	// 반복문 내에서 time.After를 사용하면 매번 객체가 생성되어 GC 부하가 발생하므로,
	// 타이머를 하나만 생성하고 재사용(Reset)하는 최적화 패턴을 사용합니다.
	fetchDelay := time.Duration(commandSettings.PageFetchDelay) * time.Millisecond
	fetchDelayTimer := time.NewTimer(fetchDelay)

	// 초기 생성된 타이머는 즉시 정지시킵니다.
	// Stop()이 false를 반환하면 이미 만료되었을 수 있으므로, 채널을 비워(Drain) 안전하게 초기화합니다.
	if !fetchDelayTimer.Stop() {
		select {
		case <-fetchDelayTimer.C:
		default:
		}
	}
	defer fetchDelayTimer.Stop()

	// =========================================================================
	// [초기화] 루프 상태 변수 준비
	// =========================================================================

	// 필터링과 중복 제거를 모두 통과한 최종 공연 목록입니다.
	var collectedPerformances []*performance

	// 동일한 공연이 여러 페이지에 걸쳐 중복 노출되는 경우를 방지하기 위한 방문 기록 맵입니다.
	// 공연의 고유 Key를 맵의 키로 사용하며, 이미 수집된 공연은 true로 표시하여 이후 페이지에서 재수집되지 않도록 합니다.
	collectedKeys := make(map[string]bool)

	// 필터링이 적용되기 전, API로부터 수신한 원시(raw) 공연 데이터의 누적 총 개수입니다.
	// 최종 수집된 개수와 비교함으로써 필터링으로 제외된 항목의 수를 파악하고 로깅 및 모니터링 자료로 활용합니다.
	totalFetchedCount := 0

	// 네이버 API에 요청할 현재 페이지 번호입니다.
	// 네이버 API는 1-based 인덱스를 사용하므로 1로 초기화하며, 페이지를 순회할 때마다 1씩 증가합니다.
	currentPage := 1

	// =========================================================================
	// [메인 루프] 페이지 단위로 공연 데이터를 수집합니다.
	// =========================================================================
	for {
		// 작업 취소 요청이 들어왔는지 주기적으로 확인하여, 불필요한 리소스 낭비를 방지합니다.
		if t.IsCanceled() {
			t.Log(component, applog.InfoLevel, "수집 중단: 외부 취소 요청", nil, applog.Fields{
				"query":           commandSettings.Query,
				"page":            currentPage,
				"limit_max_pages": commandSettings.MaxPages,
				"collected_count": len(collectedPerformances),
				"fetched_count":   totalFetchedCount,
			})

			return nil, context.Canceled
		}

		// 설정된 최대 페이지 수를 초과하면, 데이터가 남아있더라도 강제로 수집을 중단합니다.
		// (무한 루프 방지 및 과도한 데이터 수집 제한)
		if currentPage > commandSettings.MaxPages {
			t.Log(component, applog.WarnLevel, "수집 조기 종료: 최대 페이지 제한 도달", nil, applog.Fields{
				"query":           commandSettings.Query,
				"page":            currentPage,
				"limit_max_pages": commandSettings.MaxPages,
				"collected_count": len(collectedPerformances),
				"fetched_count":   totalFetchedCount,
			})

			break
		}

		t.Log(component, applog.DebugLevel, "페이지 요청: 네이버 공연 검색", nil, applog.Fields{
			"query":           commandSettings.Query,
			"page":            currentPage,
			"collected_count": len(collectedPerformances),
		})

		// 실제 네이버 서버에 HTTP 요청을 보내 해당 페이지의 공연 목록을 가져옵니다.
		rawPerformances, rawCount, err := t.fetchPagePerformances(ctx, commandSettings.Query, currentPage)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				t.Log(component, applog.InfoLevel, "수집 중단: 외부 취소 요청", nil, applog.Fields{
					"query":           commandSettings.Query,
					"page":            currentPage,
					"limit_max_pages": commandSettings.MaxPages,
					"collected_count": len(collectedPerformances),
					"fetched_count":   totalFetchedCount,
					"error":           err.Error(),
				})

				return nil, context.Canceled
			}

			// [주의] context.Canceled로 교체하지 않고 ctx.Err()를 그대로 반환합니다.
			//
			// 왜 교체하면 안 되는가?
			//   base.go의 finalizeExecution()은 에러가 context.Canceled인 경우에만
			//   "사용자가 직접 취소한 것"으로 간주하여 에러 알림 전송을 의도적으로 생략합니다.
			//
			//   만약 여기서 ctx.Err()(예: context.DeadlineExceeded)를 context.Canceled로
			//   교체해버리면, 타임아웃으로 인한 수집 실패임에도 불구하고 에러 알림이 전송되지 않아
			//   사용자는 서비스가 조용히 실패하고 있다는 사실을 전혀 알 수 없게 됩니다.
			//
			// 올바른 동작:
			//   - context.Canceled  → 사용자가 명시적으로 취소한 경우: 알림 생략 (정상)
			//   - context.DeadlineExceeded → 타임아웃으로 실패한 경우:  알림 전송 (이상 상황)
			if ctxErr := ctx.Err(); ctxErr != nil {
				t.Log(component, applog.WarnLevel, "수집 중단: 컨텍스트 종료", ctxErr, applog.Fields{
					"query":           commandSettings.Query,
					"page":            currentPage,
					"limit_max_pages": commandSettings.MaxPages,
					"collected_count": len(collectedPerformances),
					"fetched_count":   totalFetchedCount,
					"error":           err.Error(),
				})

				return nil, ctxErr
			}

			return nil, err
		}

		// 이번 페이지에서 조회된 '필터링 전' 전체 항목 수를 누적합니다.
		// (실제 수집된 개수와 비교하여 필터링 비율 등을 파악하는 용도로 사용됩니다)
		totalFetchedCount += rawCount

		// 수집된 공연 목록을 순회하며 1차 필터링(키워드) 및 2차 필터링(중복 제거)을 수행합니다.
		// 모든 조건을 통과한 유효한 공연만 최종 결과 목록에 추가됩니다.
		for _, p := range rawPerformances {
			// 제목 및 장소 키워드 필터 적용: 조건에 맞지 않으면 건너뜁니다.
			if !matchers.TitleMatcher.Match(p.Title) || !matchers.PlaceMatcher.Match(p.Place) {
				continue
			}

			// 동일한 공연이 여러 페이지에 중복 노출될 수 있으므로 Key 기반으로 중복을 제거합니다.
			key := p.key()
			if collectedKeys[key] {
				continue
			}

			// 해당 공연의 Key를 방문 기록에 등록합니다.
			// 이후 페이지에서 동일한 Key를 가진 공연이 다시 등장하면 위의 중복 검사에서 걸러집니다.
			collectedKeys[key] = true

			// 키워드 필터와 중복 검사를 모두 통과한 유효한 공연이므로 최종 결과 목록에 추가합니다.
			collectedPerformances = append(collectedPerformances, p)
		}

		// 다음 루프에서 요청할 페이지 번호를 설정합니다.
		currentPage += 1

		// 이번 페이지에서 조회된 데이터가 없으면 마지막 페이지에 도달한 것으로 판단합니다.
		// 더 이상 수집할 데이터가 없으므로 루프를 종료합니다.
		if rawCount == 0 {
			t.Log(component, applog.DebugLevel, "수집 종료: 데이터 없음", nil, applog.Fields{
				"query":             commandSettings.Query,
				"last_visited_page": currentPage - 1,
				"collected_count":   len(collectedPerformances),
				"fetched_count":     totalFetchedCount,
			})

			break
		}

		// 다음 페이지 요청 전 설정된 시간만큼 대기합니다.
		// (네이버 서버 부하 방지 및 차단 회피 목적)
		//
		// 매번 새로운 타이머를 생성하는 대신 기존 타이머를 재사용(Reset)하여 GC 부하를 최소화합니다.
		if !fetchDelayTimer.Stop() {
			select {
			case <-fetchDelayTimer.C:
			default:
			}
		}
		fetchDelayTimer.Reset(fetchDelay)

		select {
		case <-ctx.Done():
			t.Log(component, applog.InfoLevel, "수집 중단: 외부 취소 요청", nil, applog.Fields{
				"query":             commandSettings.Query,
				"last_visited_page": currentPage - 1,
				"limit_max_pages":   commandSettings.MaxPages,
				"collected_count":   len(collectedPerformances),
				"fetched_count":     totalFetchedCount,
			})

			return nil, ctx.Err()

		case <-fetchDelayTimer.C: // 대기 시간이 만료되면 다음 루프(페이지)로 진행
		}
	}

	t.Log(component, applog.InfoLevel, "수집 완료: 정상 종료", nil, applog.Fields{
		"query":             commandSettings.Query,
		"last_visited_page": currentPage - 1,
		"limit_max_pages":   commandSettings.MaxPages,
		"collected_count":   len(collectedPerformances),
		"fetched_count":     totalFetchedCount,
	})

	return collectedPerformances, nil
}
