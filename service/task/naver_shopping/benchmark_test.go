package naver_shopping

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/darkkaiser/notify-server/service/task"
	"github.com/darkkaiser/notify-server/service/task/testutil"
)

func BenchmarkNaverShoppingTask_RunWatchPrice(b *testing.B) {
	// 1. Mock 설정
	mockFetcher := testutil.NewMockHTTPFetcher()
	query := "아이폰"
	encodedQuery := url.QueryEscape(query)

	// 검색 결과 JSON (상품 정보 포함)
	// 100개의 아이템을 생성하여 파싱 부하를 시뮬레이션
	itemsJSON := ""
	for i := 0; i < 100; i++ {
		if i > 0 {
			itemsJSON += ","
		}
		itemsJSON += fmt.Sprintf(`{
			"title": "Test Product %d",
			"link": "http://example.com/product/%d",
			"lprice": "%d",
			"mallName": "Test Mall",
			"productId": "%d",
			"productType": "1"
		}`, i, i, 10000+i*100, i)
	}

	searchResultJSON := fmt.Sprintf(`{
		"total": 100,
		"start": 1,
		"display": 100,
		"items": [%s]
	}`, itemsJSON)

	// 첫 번째 페이지 요청에 대한 응답 설정
	url1 := fmt.Sprintf("%s?query=%s&display=100&start=1&sort=sim", naverShoppingSearchURL, encodedQuery)
	mockFetcher.SetResponse(url1, []byte(searchResultJSON))

	// 2. Task 초기화
	tTask := &naverShoppingTask{
		Task: task.NewBaseTask(ID, WatchPriceAnyCommand, "test_instance", "test-notifier", task.RunByUnknown),
	}
	tTask.SetFetcher(mockFetcher)

	// 3. 테스트 데이터 준비
	commandConfig := &watchPriceConfig{
		Query: query,
		Filters: struct {
			IncludedKeywords string `json:"included_keywords"`
			ExcludedKeywords string `json:"excluded_keywords"`
			PriceLessThan    int    `json:"price_less_than"`
		}{
			PriceLessThan: 20000,
		},
	}

	resultData := &naverShoppingWatchPriceResultData{
		Products: make([]*naverShoppingProduct, 0),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 벤치마크 실행
		_, _, err := tTask.executeWatchPrice(commandConfig, resultData, true)
		if err != nil {
			b.Fatalf("Task run failed: %v", err)
		}
	}
}
