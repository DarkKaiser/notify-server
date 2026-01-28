package naver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/stretchr/testify/require"
)

// helper: URL 생성
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

// helper: HTML 조각 생성
func makeHTMLItem(title, place string) string {
	return fmt.Sprintf(`<li><div class="item"><div class="title_box"><strong class="name">%s</strong><span class="sub_text">%s</span></div><div class="thumb"><img src="http://example.com/thumb.jpg"></div></div></li>`, title, place)
}

func setupTestTask(t *testing.T, fetcher fetcher.Fetcher) (*task, *config.AppConfig) {
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
	handler, err := createTask("test_instance", req, appConfig, fetcher)
	require.NoError(t, err)
	tsk, ok := handler.(*task)
	require.True(t, ok)
	return tsk, appConfig
}

func TestNaverTask_Integration_Scenarios(t *testing.T) {
	query := "뮤지컬"

	tests := []struct {
		name           string
		configModifier func(*watchNewPerformancesSettings)
		mockSetup      func(*mocks.MockHTTPFetcher)
		validate       func(*testing.T, string, interface{}, error)
	}{
		{
			name: "Success_SinglePage",
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				html := fmt.Sprintf("<ul>%s</ul>", makeHTMLItem("Cats", "Broadway"))
				b, _ := json.Marshal(html)
				m.SetResponse(makeMockSearchURL(query, 1), []byte(fmt.Sprintf(`{"html": %s}`, string(b))))
				m.SetResponse(makeMockSearchURL(query, 2), []byte(`{"html": ""}`))
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
			name: "Success_MultiPage",
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				html1 := fmt.Sprintf("<ul>%s</ul>", makeHTMLItem("Item1", "Place1"))
				b1, _ := json.Marshal(html1)
				m.SetResponse(makeMockSearchURL(query, 1), []byte(fmt.Sprintf(`{"html": %s}`, string(b1))))

				html2 := fmt.Sprintf("<ul>%s</ul>", makeHTMLItem("Item2", "Place2"))
				b2, _ := json.Marshal(html2)
				m.SetResponse(makeMockSearchURL(query, 2), []byte(fmt.Sprintf(`{"html": %s}`, string(b2))))

				m.SetResponse(makeMockSearchURL(query, 3), []byte(`{"html": ""}`))
			},
			validate: func(t *testing.T, msg string, data interface{}, err error) {
				require.NoError(t, err)
				snapshot, ok := data.(*watchNewPerformancesSnapshot)
				require.True(t, ok)
				require.Len(t, snapshot.Performances, 2)
				require.Equal(t, "Item1", snapshot.Performances[0].Title)
				require.Equal(t, "Item2", snapshot.Performances[1].Title)
			},
		},
		{
			name: "MaxPages_Limit",
			configModifier: func(c *watchNewPerformancesSettings) {
				c.MaxPages = 2 // Limit to 2 pages
			},
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				html1 := fmt.Sprintf("<ul>%s</ul>", makeHTMLItem("P1", "L"))
				b1, _ := json.Marshal(html1)
				m.SetResponse(makeMockSearchURL(query, 1), []byte(fmt.Sprintf(`{"html": %s}`, string(b1))))

				html2 := fmt.Sprintf("<ul>%s</ul>", makeHTMLItem("P2", "L"))
				b2, _ := json.Marshal(html2)
				m.SetResponse(makeMockSearchURL(query, 2), []byte(fmt.Sprintf(`{"html": %s}`, string(b2))))

				html3 := fmt.Sprintf("<ul>%s</ul>", makeHTMLItem("P3", "L"))
				b3, _ := json.Marshal(html3)
				m.SetResponse(makeMockSearchURL(query, 3), []byte(fmt.Sprintf(`{"html": %s}`, string(b3))))
			},
			validate: func(t *testing.T, msg string, data interface{}, err error) {
				require.NoError(t, err)
				snapshot, ok := data.(*watchNewPerformancesSnapshot)
				require.True(t, ok)
				require.Len(t, snapshot.Performances, 2, "Should stop after 2 pages")
				require.Equal(t, "P1", snapshot.Performances[0].Title)
				require.Equal(t, "P2", snapshot.Performances[1].Title)
			},
		},
		{
			name: "Filtering_Combined",
			configModifier: func(c *watchNewPerformancesSettings) {
				c.Filters.Title.IncludedKeywords = "Cats"
				c.Filters.Place.ExcludedKeywords = "Seoul"
			},
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				items := []string{
					makeHTMLItem("Cats Musical", "Busan"),
					makeHTMLItem("Dogs Musical", "Busan"),
					makeHTMLItem("Cats Musical", "Seoul"),
				}
				html := fmt.Sprintf("<ul>%s%s%s</ul>", items[0], items[1], items[2])
				b, _ := json.Marshal(html)
				m.SetResponse(makeMockSearchURL(query, 1), []byte(fmt.Sprintf(`{"html": %s}`, string(b))))
				m.SetResponse(makeMockSearchURL(query, 2), []byte(`{"html": ""}`))
			},
			validate: func(t *testing.T, msg string, data interface{}, err error) {
				require.NoError(t, err)
				snapshot, ok := data.(*watchNewPerformancesSnapshot)
				require.True(t, ok)

				require.Len(t, snapshot.Performances, 1) // Only Cats/Busan remains
				require.Equal(t, "Cats Musical", snapshot.Performances[0].Title)
				require.Equal(t, "Busan", snapshot.Performances[0].Place)
			},
		},
		{
			name: "Deduplication_PaginationDrift",
			// Scenario:
			// Page 1: [Item A, Item B]
			// Page 2: [Item B, Item C] (Item B is duplicated due to pagination drift)
			// Expected: [Item A, Item B, Item C] (Total 3, not 4)
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				// Page 1
				html1 := fmt.Sprintf("<ul>%s%s</ul>", makeHTMLItem("Item A", "Place A"), makeHTMLItem("Item B", "Place B"))
				b1, _ := json.Marshal(html1)
				m.SetResponse(makeMockSearchURL(query, 1), []byte(fmt.Sprintf(`{"html": %s}`, string(b1))))

				// Page 2
				html2 := fmt.Sprintf("<ul>%s%s</ul>", makeHTMLItem("Item B", "Place B"), makeHTMLItem("Item C", "Place C"))
				b2, _ := json.Marshal(html2)
				m.SetResponse(makeMockSearchURL(query, 2), []byte(fmt.Sprintf(`{"html": %s}`, string(b2))))

				// Page 3 (Empty)
				m.SetResponse(makeMockSearchURL(query, 3), []byte(`{"html": ""}`))
			},
			validate: func(t *testing.T, msg string, data interface{}, err error) {
				require.NoError(t, err)
				snapshot, ok := data.(*watchNewPerformancesSnapshot)
				require.True(t, ok)

				require.Len(t, snapshot.Performances, 3, "Duplicate Item B should be removed")
				require.Equal(t, "Item A", snapshot.Performances[0].Title)
				require.Equal(t, "Item B", snapshot.Performances[1].Title)
				require.Equal(t, "Item C", snapshot.Performances[2].Title)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFetcher := mocks.NewMockHTTPFetcher()
			if tt.mockSetup != nil {
				tt.mockSetup(mockFetcher)
			}

			tTask, _ := setupTestTask(t, mockFetcher)

			// Setup Config
			cmdConfig := &watchNewPerformancesSettings{
				Query:          query,
				PageFetchDelay: 1, // Speed up tests
			}
			if tt.configModifier != nil {
				tt.configModifier(cmdConfig)
			}
			// Important: Eager Init
			require.NoError(t, cmdConfig.validate())

			// Run
			prev := &watchNewPerformancesSnapshot{Performances: []*performance{}}
			msg, data, err := tTask.executeWatchNewPerformances(context.Background(), cmdConfig, prev, true)

			// Validate
			if tt.validate != nil {
				tt.validate(t, msg, data, err)
			}
		})
	}
}

func TestNaverTask_RunWatchNewPerformances_PartialFailure(t *testing.T) {
	t.Skip("Partial Failure 허용 기능이 아직 적용되지 않았습니다.")
}
