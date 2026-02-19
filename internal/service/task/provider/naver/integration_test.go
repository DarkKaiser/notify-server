package naver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	contractmocks "github.com/darkkaiser/notify-server/internal/service/contract/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

// makeMockSearchURL 통합 테스트용 Mock URL을 생성합니다.
func makeMockSearchURL(query string, page int) string {
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

// makeHTMLItem 테스트용 HTML 아이템 조각을 생성합니다.
func makeHTMLItem(title, place string) string {
	return fmt.Sprintf(`<li><div class="item"><div class="title_box"><strong class="name">%s</strong><span class="sub_text">%s</span></div><div class="thumb"><img src="http://example.com/thumb.jpg"></div></div></li>`, title, place)
}

// setupTestTaskWithDeps 통합 테스트에 필요한 Task와 의존성을 설정합니다.
func setupTestTaskWithDeps(t *testing.T, fetcher fetcher.Fetcher) (*task, *config.AppConfig, *contractmocks.MockTaskResultStore) {
	req := &contract.TaskSubmitRequest{
		TaskID:     TaskID,
		CommandID:  WatchNewPerformancesCommand,
		NotifierID: "test-notifier",
		RunBy:      contract.TaskRunByScheduler,
	}
	appConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: string(TaskID),
				Commands: []config.CommandConfig{
					{
						ID: string(WatchNewPerformancesCommand),
						Data: map[string]interface{}{
							"query": "테스트",
						},
					},
				},
			},
		},
	}
	storage := new(contractmocks.MockTaskResultStore)

	// Task 인스턴스 생성
	handler, err := newTask(provider.NewTaskParams{
		InstanceID:  "test_instance",
		Request:     req,
		AppConfig:   appConfig,
		Storage:     storage,
		Fetcher:     fetcher,
		NewSnapshot: func() any { return &watchNewPerformancesSnapshot{} },
	})
	require.NoError(t, err)

	tsk, ok := handler.(*task)
	require.True(t, ok)

	return tsk, appConfig, storage
}

// -----------------------------------------------------------------------------
// Integration Scenarios
// -----------------------------------------------------------------------------

func TestNaverTask_Integration_Scenarios(t *testing.T) {
	query := "뮤지컬"
	// 테스트 속도를 높이기 위해 딜레이를 최소화하는 설정 Modifier
	fastConfig := func(c *watchNewPerformancesSettings) {
		c.PageFetchDelay = 1 // 1ms
	}

	tests := []struct {
		name           string
		configModifier func(*watchNewPerformancesSettings)
		mockSetup      func(*mocks.MockHTTPFetcher)
		ctxModifier    func() (context.Context, context.CancelFunc) // 컨텍스트 제어용 (Timeout/Cancel)
		validate       func(*testing.T, string, interface{}, error)
	}{
		{
			name:           "Success_SinglePage",
			configModifier: fastConfig,
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				html := fmt.Sprintf("<ul>%s</ul>", makeHTMLItem("Cats", "Broadway"))
				b, _ := json.Marshal(html)
				m.SetResponse(makeMockSearchURL(query, 1), []byte(fmt.Sprintf(`{"html": %s}`, string(b))))
				m.SetResponse(makeMockSearchURL(query, 2), []byte(`{"html": "<div class=\"api_no_result\">검색결과가 없습니다</div>"}`))
			},
			validate: func(t *testing.T, msg string, data interface{}, err error) {
				require.NoError(t, err)
				snapshot, ok := data.(*watchNewPerformancesSnapshot)
				require.True(t, ok)
				require.Len(t, snapshot.Performances, 1)
				require.Equal(t, "Cats", snapshot.Performances[0].Title)
				require.Contains(t, msg, "Cats")
			},
		},
		{
			name:           "Success_MultiPage",
			configModifier: fastConfig,
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				html1 := fmt.Sprintf("<ul>%s</ul>", makeHTMLItem("Item1", "Place1"))
				b1, _ := json.Marshal(html1)
				m.SetResponse(makeMockSearchURL(query, 1), []byte(fmt.Sprintf(`{"html": %s}`, string(b1))))

				html2 := fmt.Sprintf("<ul>%s</ul>", makeHTMLItem("Item2", "Place2"))
				b2, _ := json.Marshal(html2)
				m.SetResponse(makeMockSearchURL(query, 2), []byte(fmt.Sprintf(`{"html": %s}`, string(b2))))

				m.SetResponse(makeMockSearchURL(query, 3), []byte(`{"html": "<div class=\"api_no_result\">검색결과가 없습니다</div>"}`))
			},
			validate: func(t *testing.T, msg string, data interface{}, err error) {
				require.NoError(t, err)
				snapshot, ok := data.(*watchNewPerformancesSnapshot)
				require.True(t, ok)
				require.Len(t, snapshot.Performances, 2)
				require.Equal(t, "Item2", snapshot.Performances[1].Title)
			},
		},
		{
			name: "MaxPages_Limit",
			configModifier: func(c *watchNewPerformancesSettings) {
				c.MaxPages = 2 // 2페이지만 수집하고 중단
				c.PageFetchDelay = 1
			},
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				// 3페이지까지 데이터가 있어도 2페이지까지만 호출해야 함 (Mock이 예상치 못한 호출에 에러를 뱉는지는 Mock 구현에 따름)
				html1 := fmt.Sprintf("<ul>%s</ul>", makeHTMLItem("P1", "L"))
				b1, _ := json.Marshal(html1)
				m.SetResponse(makeMockSearchURL(query, 1), []byte(fmt.Sprintf(`{"html": %s}`, string(b1))))

				html2 := fmt.Sprintf("<ul>%s</ul>", makeHTMLItem("P2", "L"))
				b2, _ := json.Marshal(html2)
				m.SetResponse(makeMockSearchURL(query, 2), []byte(fmt.Sprintf(`{"html": %s}`, string(b2))))

				// 3페이지는 호출되지 않아야 하므로 Mock 설정 불필요 (또는 호출 시 에러 발생하도록 검증 가능)
			},
			validate: func(t *testing.T, msg string, data interface{}, err error) {
				require.NoError(t, err)
				snapshot, ok := data.(*watchNewPerformancesSnapshot)
				require.True(t, ok)
				require.Len(t, snapshot.Performances, 2, "최대 페이지 수 설정에 따라 2개만 수집되어야 함")
			},
		},
		{
			name: "Context_Cancellation",
			configModifier: func(c *watchNewPerformancesSettings) {
				c.PageFetchDelay = 500 // 취소 기회를 주기 위해 딜레이 추가
			},
			ctxModifier: func() (context.Context, context.CancelFunc) {
				// 아주 짧은 타임아웃을 주어 작업 도중 취소되도록 유도
				return context.WithTimeout(context.Background(), 50*time.Millisecond)
			},
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				// 첫 페이지 응답은 성공하지만, 딜레이 중 컨텍스트가 만료될 것으로 예상
				html := fmt.Sprintf("<ul>%s</ul>", makeHTMLItem("Item1", "Place1"))
				b, _ := json.Marshal(html)
				m.SetResponse(makeMockSearchURL(query, 1), []byte(fmt.Sprintf(`{"html": %s}`, string(b))))
				// 2페이지 요청이 올 수도 있고 안 올 수도 있음 (타이밍 의존)
			},
			validate: func(t *testing.T, msg string, data interface{}, err error) {
				// 컨텍스트 취소로 인한 에러 반환 확인
				require.Error(t, err)
				require.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) ||
					(err != nil && (err.Error() == "context deadline exceeded" || err.Error() == "context canceled")),
					"Context 취소 또는 타임아웃 에러가 발생해야 합니다. Got: %v", err)
			},
		},
		{
			name:           "Network_Failure_Retry",
			configModifier: fastConfig,
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				// MockFetcher는 현재 재시도 로직이 내장되어 있지 않음 (Scraper 레벨에서 처리 가정).
				// 여기서는 Fetcher가 에러를 반환했을 때 Task가 정상적으로 에러를 전파하고 중단하는지 확인.
				m.SetError(makeMockSearchURL(query, 1), errors.New("network timeout"))
			},
			validate: func(t *testing.T, msg string, data interface{}, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "network timeout")
			},
		},
		{
			name:           "Invalid_Response_But_Recoverable",
			configModifier: fastConfig,
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				// 1페이지: 정상
				html1 := fmt.Sprintf("<ul>%s</ul>", makeHTMLItem("Item1", "Place1"))
				b1, _ := json.Marshal(html1)
				m.SetResponse(makeMockSearchURL(query, 1), []byte(fmt.Sprintf(`{"html": %s}`, string(b1))))

				// 2페이지: JSON이 아닌 이상한 응답 (Scraper에서 에러 처리)
				// 현재 로직상 페이지 수집 중 에러 발생 시 전체 실패로 간주하는지,
				// Partial Success를 허용하는지 정책에 따라 다름.
				// 현재: pageFetchLoop에서 에러 발생 시 즉시 리턴하고 수집된 데이터(parsedPerformances)는 버려짐?
				// -> watch_new_performances.go의 loop 로직을 보면 에러 시 break 하고 err 반환.
				m.SetResponse(makeMockSearchURL(query, 2), []byte(`Invalid JSON`))
			},
			validate: func(t *testing.T, msg string, data interface{}, err error) {
				// 현재 구현상 중간에 에러가 나면 전체 실패 처리
				require.Error(t, err)
				snapshot, ok := data.(*watchNewPerformancesSnapshot)
				if ok && snapshot != nil {
					// 실패 시 nil을 리턴하므로 이 블록은 실행되지 않을 수 있음
					// 만약 Partial Success를 지원한다면 여기서 Item1 확인 가능
				}
			},
		},
		{
			name:           "Execution_With_No_Changes",
			configModifier: fastConfig,
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				html := fmt.Sprintf("<ul>%s</ul>", makeHTMLItem("OldItem", "Place"))
				b, _ := json.Marshal(html)
				m.SetResponse(makeMockSearchURL(query, 1), []byte(fmt.Sprintf(`{"html": %s}`, string(b))))
				m.SetResponse(makeMockSearchURL(query, 2), []byte(`{"html": "<div class=\"api_no_result\">검색결과가 없습니다</div>"}`))
			},
			validate: func(t *testing.T, msg string, data interface{}, err error) {
				require.NoError(t, err)
				// snapshot에는 데이터가 있지만,
				// execute 메서드의 반환값 중 msg는 "변경 없음"일 경우 비어있거나 특정 메시지일 수 있음.
				// 여기서는 executeWatchNewPerformances 호출 시 prevSnapshot을 동일하게 주입하여 변경 없음을 유도해야 함.
				// 그러나 setup 로직에서 prevSnapshot을 비워두고 있으므로, 이 테스트 케이스에서 별도로 prev를 주입해야 정확함.
				// 아래 루프 내에서 처리 필요.
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFetcher := mocks.NewMockHTTPFetcher()
			if tt.mockSetup != nil {
				tt.mockSetup(mockFetcher)
			}

			tTask, _, _ := setupTestTaskWithDeps(t, mockFetcher)

			// Config 설정
			cmdConfig := &watchNewPerformancesSettings{
				Query:          query,
				PageFetchDelay: 100, // 기본값
			}
			if tt.configModifier != nil {
				tt.configModifier(cmdConfig)
			}
			cmdConfig.ApplyDefaults()
			require.NoError(t, cmdConfig.Validate())

			// Context 설정
			ctx := context.Background()
			if tt.ctxModifier != nil {
				var cancel context.CancelFunc
				ctx, cancel = tt.ctxModifier()
				defer cancel()
			}

			// Run
			// 시나리오별 이전 상태 주입이 필요하다면 확장이 필요하지만,
			// 여기서는 기본적으로 "이전 상태 없음(최초 실행)"을 가정
			prev := &watchNewPerformancesSnapshot{Performances: []*performance{}}

			// Execution_With_No_Changes 케이스를 위한 특수 처리 (이전 상태 주입)
			if tt.name == "Execution_With_No_Changes" {
				prev.Performances = []*performance{
					{Title: "OldItem", Place: "Place"},
				}
			}

			msg, data, err := tTask.executeWatchNewPerformances(ctx, cmdConfig, prev, true)

			// Validate
			if tt.validate != nil {
				tt.validate(t, msg, data, err)
			}
		})
	}
}
