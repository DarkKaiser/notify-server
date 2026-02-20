package naver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/pkg/mark"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Helper Functions & Types
// =============================================================================

// makeHTMLHelper helps creating HTML content for tests
func makeHTMLHelper(items ...string) string {
	if len(items) == 0 {
		return `<div class="api_no_result">검색결과가 없습니다</div>`
	}
	return fmt.Sprintf("<ul>%s</ul>", strings.Join(items, ""))
}

func makeItemHelper(title, place string) string {
	// Use absolute URL for thumbnail to simplify test expectations and avoid resolution logic dependency
	return fmt.Sprintf(`<li><div class="item"><div class="title_box"><strong class="name">%s</strong><span class="sub_text">%s</span></div><div class="thumb"><img src="https://example.com/thumb.jpg"></div></div></li>`, title, place)
}

func makeJSONResponse(htmlContent string) string {
	m := map[string]string{"html": htmlContent}
	b, _ := json.Marshal(m)
	return string(b)
}

// =============================================================================
// Unit Tests: Configuration & settings
// =============================================================================

func TestNaverWatchNewPerformancesSettings_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  *watchNewPerformancesSettings
		wantErr string
	}{
		{"Valid Config", &watchNewPerformancesSettings{Query: "Test"}, ""},
		{"Missing Query", &watchNewPerformancesSettings{}, "query가 입력되지 않았거나 공백입니다"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.config.Validate()
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNaverWatchNewPerformancesSettings_ApplyDefaults(t *testing.T) {
	t.Parallel()

	config := &watchNewPerformancesSettings{}
	config.ApplyDefaults()
	assert.Equal(t, 50, config.MaxPages)
	assert.Equal(t, 100, config.PageFetchDelay)
}

// =============================================================================
// Unit Tests: Parsing Logic
// =============================================================================

func TestParsePerformancesFromHTML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		html          string
		expectedItems int
		wantErr       bool
	}{
		{
			name:          "Single Item",
			html:          makeHTMLHelper(makeItemHelper("Title", "Place")),
			expectedItems: 1,
		},
		{
			name:          "Empty Result (Valid)",
			html:          makeHTMLHelper(),
			expectedItems: 0,
		},
		{
			name:    "Invalid HTML (Missing Title)",
			html:    `<ul><li><div class="item"></div></li></ul>`,
			wantErr: true,
		},
		{
			name:          "Empty Page (No Result Tag)",
			html:          "<html><body></body></html>",
			expectedItems: 0,
			wantErr:       true, // Expect error for completely empty body (structure changed)
		},
	}

	taskInstance := &task{
		Base: provider.NewBase(provider.NewTaskParams{
			Request: &contract.TaskSubmitRequest{TaskID: "T"},
			Fetcher: mocks.NewMockHTTPFetcher(),
		}, true),
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			items, _, err := taskInstance.parsePerformancesFromHTML(context.Background(), tt.html, "http://example.com", 1)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, items, tt.expectedItems)
			}
		})
	}

	// Filter Logic Test
	t.Run("Filtering", func(t *testing.T) {
		html := makeHTMLHelper(makeItemHelper("A", "Seoul"), makeItemHelper("B", "Busan"))
		items, rawCount, err := taskInstance.parsePerformancesFromHTML(context.Background(), html, "http://example.com", 1)
		require.NoError(t, err)
		assert.Equal(t, 2, rawCount)
		assert.Equal(t, 2, len(items))
	})
}

func TestBuildSearchAPIURL(t *testing.T) {
	t.Parallel()

	urlStr := buildPerformanceSearchURL("Query", 2)
	u, err := url.Parse(urlStr)
	require.NoError(t, err)
	q := u.Query()
	assert.Equal(t, "Query", q.Get("u1"))
	assert.Equal(t, "2", q.Get("u7"))
	assert.Equal(t, "kbList", q.Get("key"))
}

// =============================================================================
// Unit Tests: Rendering & Reporting
// =============================================================================

func TestRenderPerformanceDiffs_Integration(t *testing.T) {
	t.Parallel()

	// Simple integration check to ensure it works with the rest of the package
	diffs := []performanceDiff{
		{Type: performanceEventNew, Performance: &performance{Title: "T1", Place: "P1"}},
	}
	msg := renderPerformanceDiffs(diffs, false)
	assert.Contains(t, msg, mark.New)
	assert.Contains(t, msg, "T1")
}

// =============================================================================
// Integration Tests: Execution Flow
// =============================================================================

func TestTask_ExecuteWatchNewPerformances_Flow(t *testing.T) {
	t.Parallel()

	// Table driven test for the main execution flow
	tests := []struct {
		name           string
		runBy          contract.TaskRunBy
		settings       *watchNewPerformancesSettings
		prevSnapshot   *watchNewPerformancesSnapshot
		mockPages      []string // HTML content for pages 1, 2, ...
		mockDelay      time.Duration
		mockError      error
		expectMessage  []string
		expectEmptyMsg bool
		expectError    string
		validateSnap   func(*testing.T, *watchNewPerformancesSnapshot)
	}{
		{
			name:     "Base Execution Flow (Success)",
			runBy:    contract.TaskRunByScheduler,
			settings: &watchNewPerformancesSettings{Query: "Test", MaxPages: 1}, // Match mocked pages count
			mockPages: []string{
				makeHTMLHelper(makeItemHelper("New1", "Seoul")),
			},
			validateSnap: func(t *testing.T, s *watchNewPerformancesSnapshot) {
				require.NotNil(t, s)
				assert.Len(t, s.Performances, 1)
			},
		},
		{
			name:     "Pagination & Limit",
			runBy:    contract.TaskRunByScheduler,
			settings: &watchNewPerformancesSettings{Query: "Test", MaxPages: 2},
			mockPages: []string{
				makeHTMLHelper(makeItemHelper("P1", "L1")), // Page 1
				makeHTMLHelper(makeItemHelper("P2", "L2")), // Page 2
				makeHTMLHelper(makeItemHelper("P3", "L3")), // Page 3 (Should not be fetched)
			},
			expectMessage: []string{"P1", "P2"}, // P3 should be missing
			validateSnap: func(t *testing.T, s *watchNewPerformancesSnapshot) {
				require.NotNil(t, s)
				assert.Len(t, s.Performances, 2)
			},
		},
		{
			name:     "Duplicates Across Pages",
			runBy:    contract.TaskRunByScheduler,
			settings: &watchNewPerformancesSettings{Query: "Test", MaxPages: 2},
			mockPages: []string{
				makeHTMLHelper(makeItemHelper("A", "L1")),                            // Page 1
				makeHTMLHelper(makeItemHelper("A", "L1"), makeItemHelper("B", "L2")), // Page 2 (A is result of shifting)
			},
			validateSnap: func(t *testing.T, s *watchNewPerformancesSnapshot) {
				require.NotNil(t, s, "Snapshot should not be nil")
				assert.Len(t, s.Performances, 2, "Duplicate A should be removed, B should be kept.")
			},
		},
		{
			name:        "Network Error",
			runBy:       contract.TaskRunByScheduler,
			settings:    &watchNewPerformancesSettings{Query: "Test"},
			mockError:   fmt.Errorf("connection timeout"),
			expectError: "connection timeout",
		},
		{
			name:  "Filtering (Include/Exclude)",
			runBy: contract.TaskRunByScheduler,
			settings: func() *watchNewPerformancesSettings {
				s := &watchNewPerformancesSettings{Query: "Test", MaxPages: 1}
				s.Filters.Title.IncludedKeywords = "Keep"
				s.Filters.Title.ExcludedKeywords = "Drop"
				return s
			}(),
			mockPages: []string{
				makeHTMLHelper(
					makeItemHelper("Keep This", "L1"),
					makeItemHelper("Drop This", "L2"),
					makeItemHelper("Ignore This", "L3"),
				),
			},
			expectMessage: []string{"Keep This"},
			validateSnap: func(t *testing.T, s *watchNewPerformancesSnapshot) {
				require.NotNil(t, s)
				assert.Len(t, s.Performances, 1)
				assert.Equal(t, "Keep This", s.Performances[0].Title)
			},
		},
		{
			name:     "Zero Result Safety Check",
			runBy:    contract.TaskRunByScheduler,
			settings: &watchNewPerformancesSettings{Query: "Test", MaxPages: 1},
			prevSnapshot: &watchNewPerformancesSnapshot{
				Performances: []*performance{{Title: "Existing", Place: "Seoul"}},
			},
			mockPages: []string{
				makeHTMLHelper(), // Returns empty list (0 items)
			},
			expectEmptyMsg: true, // Should not notify "All deleted"
			validateSnap: func(t *testing.T, s *watchNewPerformancesSnapshot) {
				assert.Nil(t, s, "Snapshot should NOT be updated (safety guard for spurious empty result)")
			},
		},
		{
			name:        "JSON Parsing Error (Invalid Format)",
			runBy:       contract.TaskRunByScheduler,
			settings:    &watchNewPerformancesSettings{Query: "Test", MaxPages: 1},
			mockPages:   []string{"{invalid_json_but_this_gets_bypassed_in_helper_so_we_must_inject_raw}"}, // Actually handled via mockError below for direct fetcher control
			mockError:   fmt.Errorf("invalid json format"),                                                 // fetchPerformances 단의 에러 모사
			expectError: "invalid json format",
		},
		{
			name:        "Context Cancelled (Timeout/User Cancel)",
			runBy:       contract.TaskRunByScheduler,
			settings:    &watchNewPerformancesSettings{Query: "Test", MaxPages: 1},
			mockError:   context.Canceled, // context 취소 에러 모사
			expectError: context.Canceled.Error(),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockFetcher := mocks.NewMockHTTPFetcher()

			// Setup Mocks
			if tt.mockError != nil {
				// Simulating error on first page
				u := buildPerformanceSearchURL(tt.settings.Query, 1)
				mockFetcher.SetError(u, tt.mockError)
			} else {
				for i, pageContent := range tt.mockPages {
					u := buildPerformanceSearchURL(tt.settings.Query, i+1)
					if tt.mockDelay > 0 {
						mockFetcher.SetDelay(u, tt.mockDelay)
					}
					mockFetcher.SetResponse(u, []byte(makeJSONResponse(pageContent)))
				}
			}

			// Task Instance
			taskInstance := &task{
				Base: provider.NewBase(provider.NewTaskParams{
					Request: &contract.TaskSubmitRequest{
						TaskID: "N", CommandID: "W", NotifierID: "N", RunBy: tt.runBy,
					},
					Fetcher:     mockFetcher,
					NewSnapshot: func() any { return &watchNewPerformancesSnapshot{} },
				}, true),
			}

			if tt.settings.MaxPages == 0 {
				tt.settings.MaxPages = 10
			}
			if tt.settings.PageFetchDelay == 0 {
				tt.settings.PageFetchDelay = 1
			}

			// Execution
			msg, snap, err := taskInstance.executeWatchNewPerformances(context.Background(), tt.settings, tt.prevSnapshot, false)

			// Verification
			if tt.expectError != "" {
				assert.ErrorContains(t, err, tt.expectError)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectEmptyMsg {
				assert.Empty(t, msg)
			} else if len(tt.expectMessage) > 0 {
				for _, m := range tt.expectMessage {
					assert.Contains(t, msg, m)
				}
			}

			if tt.validateSnap != nil {
				var s *watchNewPerformancesSnapshot
				if snap != nil {
					s = snap.(*watchNewPerformancesSnapshot)
				}
				tt.validateSnap(t, s)
			}
		})
	}
}

// =============================================================================
// Unit Tests: Concurrency & Cancellation
// =============================================================================

func TestTask_CancelDuringFetch(t *testing.T) {
	t.Parallel()

	mockFetcher := mocks.NewMockHTTPFetcher()

	// Setup a delayed response
	u := buildPerformanceSearchURL("Cancel", 1)
	mockFetcher.SetDelay(u, 500*time.Millisecond)
	mockFetcher.SetResponse(u, []byte(makeJSONResponse(makeHTMLHelper(makeItemHelper("A", "B")))))

	taskInstance := &task{
		Base: provider.NewBase(provider.NewTaskParams{
			Request: &contract.TaskSubmitRequest{
				TaskID: "N", CommandID: "W", NotifierID: "N", RunBy: contract.TaskRunByUser,
			},
			Fetcher: mockFetcher,
		}, true),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error)
	go func() {
		_, err := taskInstance.fetchPerformances(ctx, &watchNewPerformancesSettings{Query: "Cancel", MaxPages: 5})
		errCh <- err
	}()

	time.Sleep(100 * time.Millisecond)
	taskInstance.Cancel() // Trigger Task Cancellation
	cancel()              // Cancel context

	select {
	case err := <-errCh:
		// Accept either context.Canceled or a wrapped error containing it
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled error, got %v", err)
		}
		assert.True(t, taskInstance.IsCanceled())
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for cancellation")
	}
}

func TestFetchPerformances_Context_Deadline(t *testing.T) {
	t.Parallel()

	mockFetcher := mocks.NewMockHTTPFetcher()
	u := buildPerformanceSearchURL("Timeout", 1)
	mockFetcher.SetDelay(u, 200*time.Millisecond)

	taskInstance := &task{
		Base: provider.NewBase(provider.NewTaskParams{
			Request: &contract.TaskSubmitRequest{TaskID: "N"},
			Fetcher: mocks.NewMockHTTPFetcher(), // Actual fetcher call will delay
		}, true),
	}
	// We need to inject the mock fetcher behavior that causes delay.
	// The mocks package usage here might need adjustment if SetDelay logic is strictly internal to HTTPFetcher mock.
	// Assuming NewMockHTTPFetcher respects SetDelay for any URL or specific URL.
	mockFetcher.SetDelay(u, 200*time.Millisecond)

	// Re-create task with the configured mockFetcher
	taskInstance = &task{
		Base: provider.NewBase(provider.NewTaskParams{
			Request: &contract.TaskSubmitRequest{TaskID: "N"},
			Fetcher: mockFetcher,
		}, true),
	}

	// Context with short deadline
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := taskInstance.fetchPerformances(ctx, &watchNewPerformancesSettings{Query: "Timeout", MaxPages: 1})

	// Should fail with context deadline exceeded
	assert.Error(t, err)
}

func TestFetchPerformances_AlreadyCanceled(t *testing.T) {
	t.Parallel()

	taskInstance := &task{
		Base: provider.NewBase(provider.NewTaskParams{
			Request: &contract.TaskSubmitRequest{
				TaskID: "N", CommandID: "W", NotifierID: "N", RunBy: contract.TaskRunByUser,
			},
			Fetcher: mocks.NewMockHTTPFetcher(),
		}, true),
	}

	// 강제 취소 부여
	taskInstance.Cancel()

	// fetchPerformances 호출 (루프 진입하자마자 IsCanceled에 잡혀야 함)
	items, err := taskInstance.fetchPerformances(context.Background(), &watchNewPerformancesSettings{Query: "X", MaxPages: 1})
	assert.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, items)
}

func TestFetchPerformances_TimerDrained(t *testing.T) {
	t.Parallel()

	mockFetcher := mocks.NewMockHTTPFetcher()
	u1 := buildPerformanceSearchURL("TimerDrain", 1)
	u2 := buildPerformanceSearchURL("TimerDrain", 2)
	u3 := buildPerformanceSearchURL("TimerDrain", 3)

	// 서버 처리가 설정된 delay보다 오래 걸린다고 가정 (이 경우 리셋 전 Stop 시 타이머 채널이 차 있어서 Drain 발생)
	// Page 1을 가져오는데 30ms가 걸리도록 지연
	mockFetcher.SetDelay(u1, 30*time.Millisecond)
	mockFetcher.SetResponse(u1, []byte(makeJSONResponse(makeHTMLHelper(makeItemHelper("A", "Seoul")))))

	// Page 2도 마찬가지 - (마지막 페이지 조건으로 루프 탈출 방지용)
	mockFetcher.SetDelay(u2, 30*time.Millisecond)
	mockFetcher.SetResponse(u2, []byte(makeJSONResponse(makeHTMLHelper(makeItemHelper("B", "Busan")))))

	// Page 3은 빈 리스트를 반환하여 종료
	mockFetcher.SetResponse(u3, []byte(makeJSONResponse(makeHTMLHelper())))

	taskInstance := &task{
		Base: provider.NewBase(provider.NewTaskParams{
			Request: &contract.TaskSubmitRequest{TaskID: "N"},
			Fetcher: mockFetcher,
		}, true),
	}

	// 의도적으로 지연 시간을 짧게 잡아 fetch(30ms) 시점에 이미 타이머가 만료되도록 함
	// 그리고 확실히 타이머 채널을 채우게 하기 위해 명시적으로 Sleep을 줍니다.
	time.Sleep(5 * time.Millisecond) // 확실히 타이머 만료 이벤트를 넘기게 유도

	settings := &watchNewPerformancesSettings{
		Query:          "TimerDrain",
		MaxPages:       5,
		PageFetchDelay: 1, // 1ms delay
	}

	items, err := taskInstance.fetchPerformances(context.Background(), settings)

	require.NoError(t, err)
	assert.Len(t, items, 2)
}

func TestFetchPerformances_RateLimiting(t *testing.T) {
	t.Parallel()

	mockFetcher := mocks.NewMockHTTPFetcher()
	pagesToFetch := 3
	query := "RateLimit"
	delayMs := 50

	// Setup mock responses for 3 pages
	for i := 1; i <= pagesToFetch; i++ {
		u := buildPerformanceSearchURL(query, i)
		mockFetcher.SetResponse(u, []byte(makeJSONResponse(makeHTMLHelper(makeItemHelper(fmt.Sprintf("Item%d", i), "Seoul")))))
	}
	// 4th page empty to stop
	u := buildPerformanceSearchURL(query, pagesToFetch+1)
	mockFetcher.SetResponse(u, []byte(makeJSONResponse(makeHTMLHelper())))

	taskInstance := &task{
		Base: provider.NewBase(provider.NewTaskParams{
			Request: &contract.TaskSubmitRequest{TaskID: "N"},
			Fetcher: mockFetcher,
		}, true),
	}

	settings := &watchNewPerformancesSettings{
		Query:          query,
		MaxPages:       10,
		PageFetchDelay: delayMs,
	}

	start := time.Now()
	items, err := taskInstance.fetchPerformances(context.Background(), settings)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Len(t, items, pagesToFetch)

	// Expected delay: (pagesToFetch - 1) * delayMs
	// 1st page: immediate
	// 2nd page: wait delay
	// 3rd page: wait delay
	// 4th page (empty): wait delay
	// Total waits: 3 times (between 1-2, 2-3, 3-4) ?
	// Let's check logic:
	// Loop 1(page 1): fetch -> wait
	// Loop 2(page 2): fetch -> wait
	// Loop 3(page 3): fetch -> wait
	// Loop 4(page 4): fetch (empty) -> break
	// Total 3 waits.
	expectedMinDuration := time.Duration((pagesToFetch)*delayMs) * time.Millisecond

	// Allow some margin for execution overhead, but it should definitely be more than the minimal delay
	assert.True(t, elapsed >= expectedMinDuration, "Execution time %v should be at least %v", elapsed, expectedMinDuration)
}

// =============================================================================
// Benchmarks Base
// =============================================================================

func BenchmarkTask_ParsePerformances(b *testing.B) {
	html := makeHTMLHelper()
	sb := strings.Builder{}
	for i := 0; i < 50; i++ {
		sb.WriteString(makeItemHelper(fmt.Sprintf("T%d", i), fmt.Sprintf("P%d", i)))
	}
	html = makeHTMLHelper(sb.String())

	taskInstance := &task{
		Base: provider.NewBase(provider.NewTaskParams{
			Request: &contract.TaskSubmitRequest{TaskID: "T"},
			Fetcher: mocks.NewMockHTTPFetcher(),
		}, true),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = taskInstance.parsePerformancesFromHTML(context.Background(), html, "http://url", 1)
	}
}
