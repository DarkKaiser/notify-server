package naver

import (
	"testing"

	"github.com/darkkaiser/notify-server/config"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
	"github.com/darkkaiser/notify-server/service/task/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTask_InvalidCommand(t *testing.T) {
	mockFetcher := testutil.NewMockHTTPFetcher()
	req := &tasksvc.SubmitRequest{
		TaskID:    ID,
		CommandID: "InvalidCommandID",
	}
	appConfig := &config.AppConfig{}

	_, err := createTask("test_instance", req, appConfig, mockFetcher)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "지원하지 않는 명령입니다")
}

func TestNaverTask_RunWatchNewPerformances(t *testing.T) {
	t.Run("정상적인 공연 정보 파싱", func(t *testing.T) {
		// Mock Fetcher 설정
		mockFetcher := testutil.NewMockHTTPFetcher()

		// Page 1 Response
		page1URL := "https://m.search.naver.com/p/csearch/content/nqapirender.nhn?key=kbList&pkid=269&where=nexearch&u7=1&u8=all&u3=&u1=%EB%AE%A4%EC%A7%80%EC%BB%AC&u2=all&u4=ingplan&u6=N&u5=date"
		mockJSON1 := `{"html": "<ul><li><div class=\"item\"><div class=\"title_box\"><strong class=\"name\">뮤지컬 오페라의 유령</strong><span class=\"sub_text\">샤롯데씨어터</span></div><div class=\"thumb\"><img src=\"https://example.com/phantom.jpg\"></div></div></li></ul>"}`
		mockFetcher.SetResponse(page1URL, []byte(mockJSON1))

		// Page 2 Response (Empty)
		page2URL := "https://m.search.naver.com/p/csearch/content/nqapirender.nhn?key=kbList&pkid=269&where=nexearch&u7=2&u8=all&u3=&u1=%EB%AE%A4%EC%A7%80%EC%BB%AC&u2=all&u4=ingplan&u6=N&u5=date"
		mockJSON2 := `{"html": ""}`
		mockFetcher.SetResponse(page2URL, []byte(mockJSON2))

		// Task 설정
		tTask := &task{
			Task: tasksvc.NewBaseTask(ID, WatchNewPerformancesCommand, "test_instance", "test_notifier", tasksvc.RunByScheduler),
			appConfig: &config.AppConfig{
				Tasks: []config.TaskConfig{
					{
						ID: string(ID),
						Commands: []config.CommandConfig{
							{
								ID: string(WatchNewPerformancesCommand),
								Data: map[string]interface{}{
									"query": "뮤지컬",
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
			},
		}
		tTask.SetFetcher(mockFetcher)

		// 초기 실행 (이전 데이터 없음)
		taskResultData := &watchNewPerformancesSnapshot{}
		message, changedData, err := tTask.executeWatchNewPerformances(
			&watchNewPerformancesCommandConfig{Query: "뮤지컬"},
			taskResultData,
			false,
		)

		require.NoError(t, err, "에러가 발생하지 않아야 합니다")
		assert.Contains(t, message, "뮤지컬 오페라의 유령", "메시지에 공연 제목이 포함되어야 합니다")

		require.NotNil(t, changedData, "변경된 데이터가 반환되어야 합니다")

		// 데이터 검증
		resultData, ok := changedData.(*watchNewPerformancesSnapshot)
		require.True(t, ok, "반환된 데이터 타입이 올바라야 합니다")
		assert.Equal(t, 1, len(resultData.Performances), "1개의 공연 정보가 추출되어야 합니다")
		assert.Equal(t, "뮤지컬 오페라의 유령", resultData.Performances[0].Title, "공연 제목이 일치해야 합니다")
	})

	t.Run("필터링 테스트", func(t *testing.T) {
		// Mock Fetcher 설정
		mockFetcher := testutil.NewMockHTTPFetcher()

		// Page 1 Response
		// Query: "공연" -> encoded: %EA%B3%B5%EC%97%B0
		page1URL := "https://m.search.naver.com/p/csearch/content/nqapirender.nhn?key=kbList&pkid=269&where=nexearch&u7=1&u8=all&u3=&u1=%EA%B3%B5%EC%97%B0&u2=all&u4=ingplan&u6=N&u5=date"
		mockJSON1 := `{"html": "<ul><li><div class=\"item\"><div class=\"title_box\"><strong class=\"name\">뮤지컬 오페라의 유령</strong><span class=\"sub_text\">샤롯데씨어터</span></div><div class=\"thumb\"><img src=\"https://example.com/phantom.jpg\"></div></div></li><li><div class=\"item\"><div class=\"title_box\"><strong class=\"name\">연극 햄릿</strong><span class=\"sub_text\">국립극장</span></div><div class=\"thumb\"><img src=\"https://example.com/hamlet.jpg\"></div></div></li></ul>"}`
		mockFetcher.SetResponse(page1URL, []byte(mockJSON1))

		// Page 2 Response (Empty)
		page2URL := "https://m.search.naver.com/p/csearch/content/nqapirender.nhn?key=kbList&pkid=269&where=nexearch&u7=2&u8=all&u3=&u1=%EA%B3%B5%EC%97%B0&u2=all&u4=ingplan&u6=N&u5=date"
		mockJSON2 := `{"html": ""}`
		mockFetcher.SetResponse(page2URL, []byte(mockJSON2))

		// Task 설정 (필터 적용)
		tTask := &task{
			Task: tasksvc.NewBaseTask(ID, WatchNewPerformancesCommand, "test_instance", "test_notifier", tasksvc.RunByScheduler),
			appConfig: &config.AppConfig{
				Tasks: []config.TaskConfig{
					{
						ID: string(ID),
						Commands: []config.CommandConfig{
							{
								ID: string(WatchNewPerformancesCommand),
								Data: map[string]interface{}{
									"query": "공연",
									"filters": map[string]interface{}{
										"title": map[string]interface{}{
											"included_keywords": "뮤지컬",
											"excluded_keywords": "",
										},
									},
								},
							},
						},
					},
				},
			},
		}
		tTask.SetFetcher(mockFetcher)

		// 실행
		taskResultData := &watchNewPerformancesSnapshot{}
		commandConfig := &watchNewPerformancesCommandConfig{
			Query: "공연",
		}
		commandConfig.Filters.Title.IncludedKeywords = "뮤지컬"

		message, changedData, err := tTask.executeWatchNewPerformances(
			commandConfig,
			taskResultData,
			false,
		)

		require.NoError(t, err)
		assert.Contains(t, message, "뮤지컬 오페라의 유령", "필터링된 공연은 포함되어야 합니다")
		assert.NotContains(t, message, "연극 햄릿", "필터링되지 않은 공연은 포함되지 않아야 합니다")

		require.NotNil(t, changedData)
		resultData := changedData.(*watchNewPerformancesSnapshot)
		assert.Equal(t, 1, len(resultData.Performances), "1개의 공연만 추출되어야 합니다")
	})
}
