package naver

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/service/task"
)

func BenchmarkNaverTask_RunWatchNewPerformances(b *testing.B) {
	// 1. Mock 설정
	mockFetcher := task.NewMockHTTPFetcher()
	query := "뮤지컬"
	encodedQuery := url.QueryEscape(query)
	// Naver Task는 페이지 인덱스를 1부터 증가시키며 데이터를 가져옴
	// 여기서는 1페이지만 가져오고 종료되도록 설정 (데이터가 없으면 종료됨)

	// 검색 결과 JSON (공연 정보 포함)
	searchResultJSON := `{
		"total": 1,
		"html": "<ul class=\"list_news\"> <li class=\"bx\"> <div class=\"item\"> <div class=\"title_box\"> <strong class=\"name\">Test Performance</strong> <span class=\"sub_text\">Test Place</span> </div> <div class=\"thumb\"> <img src=\"http://example.com/thumb.jpg\"> </div> </div> </li> </ul>"
	}`

	// 첫 번째 페이지 요청에 대한 응답 설정
	url1 := fmt.Sprintf("https://m.search.naver.com/p/csearch/content/nqapirender.nhn?key=kbList&pkid=269&where=nexearch&u7=1&u8=all&u3=&u1=%s&u2=all&u4=ingplan&u6=N&u5=date", encodedQuery)
	mockFetcher.SetResponse(url1, []byte(searchResultJSON))

	// 두 번째 페이지 요청 (빈 데이터 -> 종료)
	emptyResultJSON := `{
		"total": 1,
		"html": ""
	}`
	url2 := fmt.Sprintf("https://m.search.naver.com/p/csearch/content/nqapirender.nhn?key=kbList&pkid=269&where=nexearch&u7=2&u8=all&u3=&u1=%s&u2=all&u4=ingplan&u6=N&u5=date", encodedQuery)
	mockFetcher.SetResponse(url2, []byte(emptyResultJSON))

	// 2. Task 초기화
	appConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: string(TidNaver),
				Commands: []config.CommandConfig{
					{
						ID: string(TcidNaverWatchNewPerformances),
						Data: map[string]interface{}{
							"query": query,
							"filters": map[string]interface{}{
								"title": map[string]interface{}{
									"included_keywords": "",
									"excluded_keywords": "",
								},
								"place": map[string]interface{}{
									"included_keywords": "",
									"excluded_keywords": "",
								},
							},
						},
					},
				},
			},
		},
	}

	tTask := &naverTask{
		Task: task.Task{
			ID:         TidNaver,
			CommandID:  TcidNaverWatchNewPerformances,
			NotifierID: "test-notifier",
			Fetcher:    mockFetcher,
		},
		appConfig: appConfig,
	}

	// 3. 테스트 데이터 준비
	commandData := &naverWatchNewPerformancesCommandData{
		Query: query,
	}

	resultData := &naverWatchNewPerformancesResultData{
		Performances: make([]*naverPerformance, 0),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 벤치마크 실행
		_, _, err := tTask.executeWatchNewPerformances(commandData, resultData, true)
		if err != nil {
			b.Fatalf("Task run failed: %v", err)
		}
	}
}
