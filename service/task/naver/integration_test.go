package naver

import (
	"fmt"
	"testing"

	"github.com/darkkaiser/notify-server/service/task"
	"github.com/stretchr/testify/require"
)

func TestNaverTask_RunWatchNewPerformances_Integration(t *testing.T) {
	// 1. Mock 설정
	mockFetcher := task.NewMockHTTPFetcher()

	// 테스트용 JSON 응답 생성
	performanceTitle := "테스트 공연"
	performancePlace := "테스트 극장"
	performanceDate := "2025.11.28~2025.12.31"
	performanceURL := "https://example.com/performance/123"

	jsonContent := fmt.Sprintf(`{
		"html": "<ul><li><div class=\"item\"><div class=\"thumb\"><img src=\"https://example.com/thumb.jpg\"></div><div class=\"title_box\"><strong class=\"name\">%s</strong><span class=\"sub_text\">%s</span></div><div class=\"info_group\"><span class=\"date\">%s</span></div><a href=\"%s\"></a></div></li></ul>"
	}`, performanceTitle, performancePlace, performanceDate, performanceURL)

	url := "https://m.search.naver.com/p/csearch/content/nqapirender.nhn?key=kbList&pkid=269&where=nexearch&u7=1&u8=all&u3=&u1=%EC%A0%84%EB%9D%BC%EB%8F%84&u2=all&u4=ingplan&u6=N&u5=date"
	mockFetcher.SetResponse(url, []byte(jsonContent))

	// 페이지 2에 대한 빈 응답 (페이지네이션 종료)
	url2 := "https://m.search.naver.com/p/csearch/content/nqapirender.nhn?key=kbList&pkid=269&where=nexearch&u7=2&u8=all&u3=&u1=%EC%A0%84%EB%9D%BC%EB%8F%84&u2=all&u4=ingplan&u6=N&u5=date"
	mockFetcher.SetResponse(url2, []byte(`{"html": ""}`))
	// 2. Task 초기화
	tTask := &naverTask{
		Task: task.Task{
			ID:         TidNaver,
			CommandID:  TcidNaverWatchNewPerformances,
			NotifierID: "test-notifier",
			Fetcher:    mockFetcher,
		},
	}

	// 3. 테스트 데이터 준비
	commandData := &naverWatchNewPerformancesCommandData{
		Query: "전라도",
	}
	commandData.Filters.Title.IncludedKeywords = ""
	commandData.Filters.Title.ExcludedKeywords = ""
	commandData.Filters.Place.IncludedKeywords = ""
	commandData.Filters.Place.ExcludedKeywords = ""

	// 초기 결과 데이터 (비어있음)
	resultData := &naverWatchNewPerformancesResultData{
		Performances: make([]*naverPerformance, 0),
	}

	// 4. 실행
	message, newResultData, err := tTask.runWatchNewPerformances(commandData, resultData, true)

	// 5. 검증
	require.NoError(t, err)
	require.NotNil(t, newResultData)

	// 결과 데이터 타입 변환
	typedResultData, ok := newResultData.(*naverWatchNewPerformancesResultData)
	require.True(t, ok)
	require.Equal(t, 1, len(typedResultData.Performances))

	performance := typedResultData.Performances[0]
	require.Equal(t, performanceTitle, performance.Title)
	require.Equal(t, performancePlace, performance.Place)

	// 메시지 검증 (신규 공연 알림)
	require.Contains(t, message, "새로운 공연정보가 등록되었습니다")
	require.Contains(t, message, performanceTitle)
	require.Contains(t, message, "🆕")
}
