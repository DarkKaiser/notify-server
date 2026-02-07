package naver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/stretchr/testify/require"
)

// generateLargeHTML 대량의 공연 데이터가 포함된 HTML을 생성합니다.
func generateLargeHTML(count int) string {
	var sb strings.Builder
	sb.WriteString("<ul class=\"list_news\">")
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

// setupBenchmarkTask 벤치마크 수행을 위한 Task와 Config를 초기화합니다.
func setupBenchmarkTask(b *testing.B, performanceCount int) (*task, *watchNewPerformancesSettings) {
	mockFetcher := mocks.NewMockHTTPFetcher()
	query := "뮤지컬"

	htmlContent := generateLargeHTML(performanceCount)
	htmlBytes, _ := json.Marshal(htmlContent)
	searchResultJSON := fmt.Sprintf(`{"total": %d, "html": %s}`, performanceCount, string(htmlBytes))

	// URL 생성 Helper
	makeURL := func(page int) string {
		v := url.Values{}
		v.Set("key", "kbList")
		v.Set("pkid", "269")
		v.Set("where", "nexearch")
		v.Set("u1", query) // Mock uses raw query? No, Encode() escapes it.
		v.Set("u2", "all")
		v.Set("u3", "")
		v.Set("u4", "ingplan")
		v.Set("u5", "date")
		v.Set("u6", "N")
		v.Set("u7", fmt.Sprintf("%d", page))
		v.Set("u8", "all")
		return "https://m.search.naver.com/p/csearch/content/nqapirender.nhn?" + v.Encode()
	}

	// 첫 페이지: 데이터 있음
	mockFetcher.SetResponse(makeURL(1), []byte(searchResultJSON))

	// 두 번째 페이지: 빈 데이터 (종료)
	mockFetcher.SetResponse(makeURL(2), []byte(`{"total": 0, "html": ""}`))

	tTask := &task{
		Base: provider.NewBase(TaskID, WatchNewPerformancesCommand, "test_instance", "test-notifier", contract.TaskRunByScheduler),
	}
	tTask.SetFetcher(mockFetcher)

	config := &watchNewPerformancesSettings{
		Query: query,
	}
	// Eager Initialization (중요: Panic 방지 및 Thread Safety 확보)
	err := config.validate()
	require.NoError(b, err)

	return tTask, config
}

func BenchmarkNaverTask_Execution(b *testing.B) {
	// 시나리오 1: 소규모 데이터 (일반적인 케이스)
	b.Run("SmallData_Serial", func(b *testing.B) {
		tTask, config := setupBenchmarkTask(b, 5)
		resultData := &watchNewPerformancesSnapshot{Performances: []*performance{}}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, err := tTask.executeWatchNewPerformances(context.Background(), config, resultData, true)
			if err != nil {
				b.Fatalf("Execution failed: %v", err)
			}
		}
	})

	// 시나리오 2: 대규모 데이터 (스트레스 테스트) - 파싱 및 메모리 할당 부하 측정
	b.Run("LargeData_Serial_100Items", func(b *testing.B) {
		tTask, config := setupBenchmarkTask(b, 100)
		resultData := &watchNewPerformancesSnapshot{Performances: []*performance{}}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, err := tTask.executeWatchNewPerformances(context.Background(), config, resultData, true)
			if err != nil {
				b.Fatalf("Execution failed: %v", err)
			}
		}
	})

	// 시나리오 3: 소규모 데이터 병렬 실행 (동시성/Race Condition 검증)
	b.Run("SmallData_Parallel", func(b *testing.B) {
		tTask, config := setupBenchmarkTask(b, 5)
		resultData := &watchNewPerformancesSnapshot{Performances: []*performance{}}

		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, _, err := tTask.executeWatchNewPerformances(context.Background(), config, resultData, true)
				if err != nil {
					b.Fatalf("Execution failed: %v", err)
				}
			}
		})
	})
}

// BenchmarkNaverTask_DiffOnly 는 네트워크/파싱을 제외한 순수 Diff 로직의 성능을 측정합니다.
func BenchmarkNaverTask_DiffOnly(b *testing.B) {
	// 데이터 준비
	count := 1000
	currentSnapshot := &watchNewPerformancesSnapshot{Performances: make([]*performance, count)}
	prevSnapshot := &watchNewPerformancesSnapshot{Performances: make([]*performance, count)}

	for i := 0; i < count; i++ {
		p := &performance{Title: fmt.Sprintf("Perf %d", i), Place: "Place"}
		currentSnapshot.Performances[i] = p
		// 절반은 같고 절반은 다르게 설정
		if i%2 == 0 {
			prevSnapshot.Performances[i] = p
		} else {
			prevSnapshot.Performances[i] = &performance{Title: "Old", Place: "Old"}
		}
	}

	tTask := &task{
		Base: provider.NewBase(TaskID, WatchNewPerformancesCommand, "test_instance", "test-notifier", contract.TaskRunByScheduler),
	}

	prevPerformancesSet := make(map[string]bool)
	for _, p := range prevSnapshot.Performances {
		prevPerformancesSet[p.Key()] = true
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// executeWatchNewPerformances 대신 내부 로직인 diffAndNotify를 직접 호출할 수도 있지만,
		// task 구조체에 메서드로 있으므로 export되지 않았다면 호출 불가.
		// diffAndNotify는 unexported이므로 동일 패키지 테스트에서는 호출 가능.
		_, _ = tTask.analyzeAndReport(currentSnapshot, prevPerformancesSet, true)
	}
}
