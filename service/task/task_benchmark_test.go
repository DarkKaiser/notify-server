package task

import (
	"fmt"
	"testing"

	"github.com/darkkaiser/notify-server/g"
)

func BenchmarkNaverTask_RunWatchNewPerformances(b *testing.B) {
	// Setup Mock Fetcher
	mockFetcher := NewMockHTTPFetcher()

	// Create realistic JSON response
	jsonContent := `{
		"html": "<ul><li><div class=\"item\"><div class=\"thumb\"><img src=\"https://example.com/thumb1.jpg\"></div><div class=\"title_box\"><strong class=\"name\">벤치마크 테스트 공연 1</strong><span class=\"sub_text\">테스트 극장 1</span></div></div></li><li><div class=\"item\"><div class=\"thumb\"><img src=\"https://example.com/thumb2.jpg\"></div><div class=\"title_box\"><strong class=\"name\">벤치마크 테스트 공연 2</strong><span class=\"sub_text\">테스트 극장 2</span></div></div></li><li><div class=\"item\"><div class=\"thumb\"><img src=\"https://example.com/thumb3.jpg\"></div><div class=\"title_box\"><strong class=\"name\">벤치마크 테스트 공연 3</strong><span class=\"sub_text\">테스트 극장 3</span></div></div></li></ul>"
	}`

	url1 := "https://m.search.naver.com/p/csearch/content/nqapirender.nhn?key=kbList&pkid=269&where=nexearch&u7=1&u8=all&u3=&u1=%EC%A0%84%EB%9D%BC%EB%8F%84&u2=all&u4=ingplan&u6=N&u5=date"
	mockFetcher.SetResponse(url1, []byte(jsonContent))

	// Empty response for page 2 (pagination end)
	url2 := "https://m.search.naver.com/p/csearch/content/nqapirender.nhn?key=kbList&pkid=269&where=nexearch&u7=2&u8=all&u3=&u1=%EC%A0%84%EB%9D%BC%EB%8F%84&u2=all&u4=ingplan&u6=N&u5=date"
	emptyJsonContent := `{"html": "<ul></ul>"}`
	mockFetcher.SetResponse(url2, []byte(emptyJsonContent))

	// Setup Task
	task := &naverTask{
		task: task{
			id:         TidNaver,
			commandID:  TcidNaverWatchNewPerformances,
			notifierID: "test-notifier",
			fetcher:    mockFetcher,
		},
		config: &g.AppConfig{},
	}

	commandData := &naverWatchNewPerformancesTaskCommandData{
		Query: "전라도",
	}

	resultData := &naverWatchNewPerformancesResultData{
		Performances: make([]*naverPerformance, 0),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, err := task.runWatchNewPerformances(commandData, resultData, true)
		if err != nil {
			b.Fatalf("Task run failed: %v", err)
		}
	}
}

func BenchmarkNaverShoppingTask_RunWatchPrice(b *testing.B) {
	// Setup Mock Fetcher
	mockFetcher := NewMockHTTPFetcher()

	// Create realistic JSON response (Naver Shopping API format)
	jsonContent := `{
		"total": 100,
		"items": [
			{
				"title": "벤치마크 테스트 상품 1",
				"link": "https://example.com/1",
				"lprice": "15000",
				"hprice": "20000",
				"mallName": "테스트 쇼핑몰 1",
				"productId": "1",
				"productType": "1"
			},
			{
				"title": "벤치마크 테스트 상품 2",
				"link": "https://example.com/2",
				"lprice": "25000",
				"hprice": "30000",
				"mallName": "테스트 쇼핑몰 2",
				"productId": "2",
				"productType": "1"
			},
			{
				"title": "벤치마크 테스트 상품 3",
				"link": "https://example.com/3",
				"lprice": "35000",
				"hprice": "40000",
				"mallName": "테스트 쇼핑몰 3",
				"productId": "3",
				"productType": "1"
			}
		]
	}`

	url := "https://openapi.naver.com/v1/search/shop.json?query=%EB%B2%A4%EC%B9%98%EB%A7%88%ED%81%AC&display=100&start=1&sort=sim"
	mockFetcher.SetResponse(url, []byte(jsonContent))

	// Setup Task
	task := &naverShoppingTask{
		task: task{
			id:         TidNaverShopping,
			commandID:  TcidNaverShoppingWatchPriceAny,
			notifierID: "test-notifier",
			fetcher:    mockFetcher,
		},
		config:       &g.AppConfig{},
		clientID:     "test-client-id",
		clientSecret: "test-client-secret",
	}

	commandData := &naverShoppingWatchPriceTaskCommandData{
		Query: "벤치마크",
	}
	commandData.Filters.PriceLessThan = 100000

	resultData := &naverShoppingWatchPriceResultData{
		Products: make([]*naverShoppingProduct, 0),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, err := task.runWatchPrice(commandData, resultData, true)
		if err != nil {
			b.Fatalf("Task run failed: %v", err)
		}
	}
}

func BenchmarkJDCTask_RunWatchNewOnlineEducation(b *testing.B) {
	// Setup Mock Fetcher
	mockFetcher := NewMockHTTPFetcher()

	// Create realistic HTML response for education list
	listHTML := `
<html>
<body>
	<div id="content">
		<ul class="prdt-list2">
			<li><a class="link" href="view?id=1">교육과정 1</a></li>
			<li><a class="link" href="view?id=2">교육과정 2</a></li>
		</ul>
	</div>
</body>
</html>`

	url1 := fmt.Sprintf("%sproduct/list?type=digital_edu", jdcBaseURL)
	url2 := fmt.Sprintf("%sproduct/list?type=untact_edu", jdcBaseURL)
	mockFetcher.SetResponse(url1, []byte(listHTML))
	mockFetcher.SetResponse(url2, []byte(listHTML))

	// Create realistic HTML response for course curriculum
	curriculumHTML := `
<html>
<body>
	<table class="prdt-tbl">
		<tbody>
			<tr>
				<td><a href="detail?id=1">벤치마크 테스트 강의 1</a><p>상세 설명 1</p></td>
				<td>2025-01-01 ~ 2025-01-31</td>
				<td>신청</td>
			</tr>
		</tbody>
	</table>
</body>
</html>`

	detailURL1 := fmt.Sprintf("%sproduct/view?id=1", jdcBaseURL)
	detailURL2 := fmt.Sprintf("%sproduct/view?id=2", jdcBaseURL)
	mockFetcher.SetResponse(detailURL1, []byte(curriculumHTML))
	mockFetcher.SetResponse(detailURL2, []byte(curriculumHTML))

	// Setup Task
	task := &jdcTask{
		task: task{
			id:         TidJdc,
			commandID:  TcidJdcWatchNewOnlineEducation,
			notifierID: "test-notifier",
			fetcher:    mockFetcher,
		},
	}

	resultData := &jdcWatchNewOnlineEducationResultData{
		OnlineEducationCourses: make([]*jdcOnlineEducationCourse, 0),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, err := task.runWatchNewOnlineEducation(resultData, true)
		if err != nil {
			b.Fatalf("Task run failed: %v", err)
		}
	}
}
