package naver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/pkg/mark"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Benchmark Setup & Helpers
// -----------------------------------------------------------------------------

// generateLargeHTML 대량의 공연 데이터가 포함된 HTML을 생성합니다.
// 벤치마크 테스트에서 동적으로 데이터를 생성하여 메모리 할당 및 파싱 부하를 시뮬레이션합니다.
func generateLargeHTML(count int) string {
	var sb strings.Builder
	// 실제 네이버 HTML 구조와 유사하게 작성하여 파싱 부하를 현실적으로 만듭니다.
	sb.WriteString(`<ul class="list_news">`)
	for i := 0; i < count; i++ {
		sb.WriteString(fmt.Sprintf(`
			<li class="bx">
				<div class="item">
					<div class="title_box">
						<strong class="name">Performance %d</strong>
						<span class="sub_text">Place %d</span>
					</div>
					<div class="thumb">
						<img src="http://example.com/thumb%d.jpg">
					</div>
				</div>
			</li>`, i, i, i))
	}
	sb.WriteString("</ul>")
	return sb.String()
}

// setupBenchmarkTask 벤치마크 수행을 위한 Task와 설정을 초기화합니다.
// b.Helper()를 통해 벤치마크 리포트에서 이 함수의 호출 스택을 제외합니다.
func setupBenchmarkTask(b *testing.B, performanceCount int) (*task, *watchNewPerformancesSettings) {
	b.Helper()

	mockFetcher := mocks.NewMockHTTPFetcher()
	query := "뮤지컬"

	// 대량의 HTML 데이터 생성
	htmlContent := generateLargeHTML(performanceCount)
	// JSON 응답 시뮬레이션 (API는 HTML을 JSON 필드에 담아 반환함)
	htmlBytes, _ := json.Marshal(htmlContent)
	searchResultJSON := fmt.Sprintf(`{"total": %d, "html": %s}`, performanceCount, string(htmlBytes))

	// URL 생성 Helper
	makeURL := func(page int) string {
		v := url.Values{}
		v.Set("key", "kbList")
		v.Set("pkid", "269")
		v.Set("where", "nexearch")
		v.Set("u1", query)
		v.Set("u2", "all")
		v.Set("u3", "")
		v.Set("u4", "ingplan")
		v.Set("u5", "date")
		v.Set("u6", "N")
		v.Set("u7", fmt.Sprintf("%d", page))
		v.Set("u8", "all")
		return "https://m.search.naver.com/p/csearch/content/nqapirender.nhn?" + v.Encode()
	}

	// Mock 응답 설정
	// 1페이지: 요청한 수만큼의 데이터 반환
	mockFetcher.SetResponse(makeURL(1), []byte(searchResultJSON))
	// 2페이지: 빈 데이터 (수집 종료 시그널)
	mockFetcher.SetResponse(makeURL(2), []byte(`{"total": 0, "html": ""}`))

	tTask := &task{
		Base: provider.NewBase(provider.NewTaskParams{
			Request: &contract.TaskSubmitRequest{
				TaskID:     TaskID,
				CommandID:  WatchNewPerformancesCommand,
				NotifierID: "benchmark-notifier",
				RunBy:      contract.TaskRunByScheduler,
			},
			InstanceID: "benchmark_instance",
			Fetcher:    mockFetcher,
			NewSnapshot: func() interface{} {
				return &watchNewPerformancesSnapshot{}
			},
		}, true),
	}

	config := &watchNewPerformancesSettings{
		Query:          query,
		PageFetchDelay: 0, // 벤치마크에서는 딜레이 제거
	}
	err := config.Validate()
	require.NoError(b, err)

	return tTask, config
}

// -----------------------------------------------------------------------------
// Component Benchmarks (Micro-benchmarks)
// -----------------------------------------------------------------------------

// BenchmarkPerformance_Key Key 생성 로직의 성능을 측정합니다.
// Key는 맵의 키로 사용되므로 매우 빈번하게 호출되며, 할당 최적화가 중요합니다.
func BenchmarkPerformance_Key(b *testing.B) {
	p := &performance{Title: "뮤지컬 오페라의 유령", Place: "샤롯데씨어터"}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = p.key()
	}
}

// BenchmarkRenderPerformance 단일 공연 항목을 렌더링하는 성능을 측정합니다.
// HTML 모드와 Text 모드를 각각 측정합니다.
func BenchmarkRenderPerformance(b *testing.B) {
	p := &performance{
		Title:     "Performance Title",
		Place:     "Performance Place",
		Thumbnail: "http://example.com/image.jpg",
	}

	b.Run("Text", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = renderPerformance(p, false, mark.New)
		}
	})

	b.Run("HTML", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = renderPerformance(p, true, mark.New)
		}
	})
}

// BenchmarkRenderPerformanceDiffs 다수의 변경 사항을 메시지로 변환하는 성능을 측정합니다.
// 문자열 연결(concatenation) 비용을 확인합니다.
func BenchmarkRenderPerformanceDiffs(b *testing.B) {
	// 데이터 준비
	count := 100
	diffs := make([]performanceDiff, count)
	for i := 0; i < count; i++ {
		diffs[i] = performanceDiff{
			Type: performanceEventNew,
			Performance: &performance{
				Title: fmt.Sprintf("Performance %d", i),
				Place: "Seoul Arts Center",
			},
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = renderPerformanceDiffs(diffs, false)
	}
}

// BenchmarkParsePerformancesFromHTML HTML 파싱 로직의 성능을 측정합니다.
// goquery 파싱 및 DOM 탐색 비용이 주된 측정 대상입니다.
func BenchmarkParsePerformancesFromHTML(b *testing.B) {
	// 벤치마크 셋업: 미리 HTML 생성
	smallHTML := generateLargeHTML(10)
	largeHTML := generateLargeHTML(100)

	// Task 인스턴스 (메서드 호출용, Fetcher 사용 안함)
	tTask := &task{Base: provider.NewBase(provider.NewTaskParams{
		Request: &contract.TaskSubmitRequest{TaskID: "bench"},
		Fetcher: mocks.NewMockHTTPFetcher(),
	}, true)}
	ctx := context.Background()
	pageURL := "http://localhost"

	b.Run("Small_10Items", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _, _ = tTask.parsePerformancesFromHTML(ctx, smallHTML, pageURL, 1)
		}
	})

	b.Run("Large_100Items", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _, _ = tTask.parsePerformancesFromHTML(ctx, largeHTML, pageURL, 1)
		}
	})
}

// BenchmarkNaverTask_DiffOnly 네트워크/파싱을 제외한 순수 Diff 계산(Snapshot Comparison) 로직 성능 측정
func BenchmarkNaverTask_DiffOnly(b *testing.B) {
	count := 1000
	currentSnapshot := &watchNewPerformancesSnapshot{Performances: make([]*performance, count)}
	prevSnapshot := &watchNewPerformancesSnapshot{Performances: make([]*performance, count)}

	for i := 0; i < count; i++ {
		p := &performance{Title: fmt.Sprintf("Perf %d", i), Place: "Place"}
		currentSnapshot.Performances[i] = p
		// 50% 변경 시나리오
		if i%2 == 0 {
			prevSnapshot.Performances[i] = p
		} else {
			prevSnapshot.Performances[i] = &performance{Title: "Old", Place: "Old"}
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = currentSnapshot.Compare(prevSnapshot)
	}
}

// -----------------------------------------------------------------------------
// Integrated Benchmarks (Full Execution Flow)
// -----------------------------------------------------------------------------

// BenchmarkNaverTask_Execution 전체 실행 흐름(Fetch -> Parse -> Diff -> Report)을 측정합니다.
func BenchmarkNaverTask_Execution(b *testing.B) {
	// 1. 소규모 데이터 (일반적인 케이스)
	b.Run("SmallData_5Items_Serial", func(b *testing.B) {
		// 셋업 비용을 타이머에서 제외하기 위해 StopTimer/StartTimer 사용
		b.StopTimer()
		tTask, config := setupBenchmarkTask(b, 5)
		resultData := &watchNewPerformancesSnapshot{Performances: []*performance{}}
		b.StartTimer()

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// 매 반복마다 Context 등을 초기화할 필요가 있다면 여기서 수행
			_, _, err := tTask.executeWatchNewPerformances(context.Background(), config, resultData, true)
			if err != nil {
				b.Fatalf("Execution failed: %v", err)
			}
		}
	})

	// 2. 대규모 데이터 (스트레스 테스트)
	b.Run("LargeData_100Items_Serial", func(b *testing.B) {
		b.StopTimer()
		tTask, config := setupBenchmarkTask(b, 100)
		resultData := &watchNewPerformancesSnapshot{Performances: []*performance{}}
		b.StartTimer()

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _, err := tTask.executeWatchNewPerformances(context.Background(), config, resultData, true)
			if err != nil {
				b.Fatalf("Execution failed: %v", err)
			}
		}
	})

	// 3. 병렬 실행 (동시성 안전성 및 경합 상태 확인)
	b.Run("SmallData_5Items_Parallel", func(b *testing.B) {
		b.StopTimer()
		tTask, config := setupBenchmarkTask(b, 5)
		resultData := &watchNewPerformancesSnapshot{Performances: []*performance{}}
		b.StartTimer()

		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				// executeWatchNewPerformances 내부에서 공유 상태(resultData)를 수정하지 않는지 확인
				// (Snapshot은 Copy-on-Write 또는 매번 새로 생성하여 반환하므로 안전해야 함)
				_, _, err := tTask.executeWatchNewPerformances(context.Background(), config, resultData, true)
				if err != nil {
					b.Fatalf("Execution failed: %v", err)
				}
			}
		})
	})
}
