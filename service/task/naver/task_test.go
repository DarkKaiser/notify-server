package naver

import (
	"testing"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/service/task"
	"github.com/darkkaiser/notify-server/service/task/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNaverWatchNewPerformancesCommandData_Validate(t *testing.T) {
	t.Run("정상적인 데이터", func(t *testing.T) {
		data := &naverWatchNewPerformancesCommandData{
			Query: "뮤지컬",
		}

		err := data.validate()
		assert.NoError(t, err, "정상적인 데이터는 검증을 통과해야 합니다")
	})

	t.Run("Query가 비어있는 경우", func(t *testing.T) {
		data := &naverWatchNewPerformancesCommandData{
			Query: "",
		}

		err := data.validate()
		assert.Error(t, err, "Query가 비어있으면 에러가 발생해야 합니다")
		assert.Contains(t, err.Error(), "query", "적절한 에러 메시지를 반환해야 합니다")
	})
}

func TestNaverPerformance_String(t *testing.T) {
	t.Run("HTML 메시지 포맷", func(t *testing.T) {
		performance := &naverPerformance{
			Title:     "테스트 공연",
			Place:     "테스트 극장",
			Thumbnail: "https://example.com/thumb.jpg",
		}

		result := performance.String(true, "")

		assert.Contains(t, result, "테스트 공연", "공연 제목이 포함되어야 합니다")
		assert.Contains(t, result, "테스트 극장", "공연 장소가 포함되어야 합니다")
		assert.Contains(t, result, "<b>", "HTML 태그가 포함되어야 합니다")
	})

	t.Run("텍스트 메시지 포맷", func(t *testing.T) {
		performance := &naverPerformance{
			Title:     "테스트 공연",
			Place:     "테스트 극장",
			Thumbnail: "https://example.com/thumb.jpg",
		}

		result := performance.String(false, "")

		assert.Contains(t, result, "테스트 공연", "공연 제목이 포함되어야 합니다")
		assert.Contains(t, result, "테스트 극장", "공연 장소가 포함되어야 합니다")
		assert.NotContains(t, result, "<b>", "HTML 태그가 포함되지 않아야 합니다")
	})

	t.Run("마크 표시", func(t *testing.T) {
		performance := &naverPerformance{
			Title: "테스트 공연",
			Place: "테스트 극장",
		}

		result := performance.String(false, " 🆕")

		assert.Contains(t, result, "🆕", "마크가 포함되어야 합니다")
	})
}

func TestNaverTask_FilterPerformances(t *testing.T) {
	t.Run("제목 필터링 - 포함 키워드", func(t *testing.T) {
		// filter 함수는 task_utils.go에 정의되어 있으므로 별도 테스트
		includedKeywords := []string{"뮤지컬"}
		excludedKeywords := []string{}

		result := task.Filter("뮤지컬 오페라의 유령", includedKeywords, excludedKeywords)
		assert.True(t, result, "포함 키워드가 있으면 true를 반환해야 합니다")
	})

	t.Run("제목 필터링 - 제외 키워드", func(t *testing.T) {
		includedKeywords := []string{"뮤지컬"}
		excludedKeywords := []string{"아동"}

		result := task.Filter("뮤지컬 아동극", includedKeywords, excludedKeywords)
		assert.False(t, result, "제외 키워드가 있으면 false를 반환해야 합니다")
	})

	t.Run("장소 필터링", func(t *testing.T) {
		includedKeywords := []string{"서울"}
		excludedKeywords := []string{}

		result := task.Filter("서울 예술의전당", includedKeywords, excludedKeywords)
		assert.True(t, result, "포함 키워드가 있으면 true를 반환해야 합니다")
	})
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
		tTask := &naverTask{
			Task: task.NewBaseTask(TidNaver, TcidNaverWatchNewPerformances, "test_instance", "test_notifier", task.RunByScheduler),
			appConfig: &config.AppConfig{
				Tasks: []config.TaskConfig{
					{
						ID: string(TidNaver),
						Commands: []config.CommandConfig{
							{
								ID: string(TcidNaverWatchNewPerformances),
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
		taskResultData := &naverWatchNewPerformancesResultData{}
		message, changedData, err := tTask.executeWatchNewPerformances(
			&naverWatchNewPerformancesCommandData{Query: "뮤지컬"},
			taskResultData,
			false,
		)

		require.NoError(t, err, "에러가 발생하지 않아야 합니다")
		assert.Contains(t, message, "뮤지컬 오페라의 유령", "메시지에 공연 제목이 포함되어야 합니다")

		require.NotNil(t, changedData, "변경된 데이터가 반환되어야 합니다")

		// 데이터 검증
		resultData, ok := changedData.(*naverWatchNewPerformancesResultData)
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
		tTask := &naverTask{
			Task: task.NewBaseTask(TidNaver, TcidNaverWatchNewPerformances, "test_instance", "test_notifier", task.RunByScheduler),
			appConfig: &config.AppConfig{
				Tasks: []config.TaskConfig{
					{
						ID: string(TidNaver),
						Commands: []config.CommandConfig{
							{
								ID: string(TcidNaverWatchNewPerformances),
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
		taskResultData := &naverWatchNewPerformancesResultData{}
		commandData := &naverWatchNewPerformancesCommandData{
			Query: "공연",
		}
		commandData.Filters.Title.IncludedKeywords = "뮤지컬"

		message, changedData, err := tTask.executeWatchNewPerformances(
			commandData,
			taskResultData,
			false,
		)

		require.NoError(t, err)
		assert.Contains(t, message, "뮤지컬 오페라의 유령", "필터링된 공연은 포함되어야 합니다")
		assert.NotContains(t, message, "연극 햄릿", "필터링되지 않은 공연은 포함되지 않아야 합니다")

		require.NotNil(t, changedData)
		resultData := changedData.(*naverWatchNewPerformancesResultData)
		assert.Equal(t, 1, len(resultData.Performances), "1개의 공연만 추출되어야 합니다")
	})
}
