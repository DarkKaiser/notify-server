package fetcher_test

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUserAgentFetcher_Do(t *testing.T) {
	customUAs := []string{"Custom/1.0", "Custom/2.0"}

	tests := []struct {
		name          string
		userAgents    []string
		useRandomUA   bool
		existingUA    string
		mockSetup     func(*mocks.MockFetcher)
		checkResponse func(*testing.T, *http.Request)
		expectedError error
	}{
		{
			name:        "Existing UA is preserved (Random Enabled)",
			userAgents:  customUAs,
			useRandomUA: true,
			existingUA:  "Original/1.0",
			mockSetup: func(m *mocks.MockFetcher) {
				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Header.Get("User-Agent") == "Original/1.0"
				})).Return(&http.Response{StatusCode: 200}, nil)
			},
			checkResponse: func(t *testing.T, req *http.Request) {
				// 원본 요청이 변경되지 않았는지 확인 (Do 내부에서 clone하므로 여기선 헤더 확인)
				// Mock Matcher에서 이미 확인했으므로 여기선 패스 가능하나, 명시적 확인
			},
		},
		{
			name:        "Existing UA is preserved (Random Disabled)",
			userAgents:  customUAs,
			useRandomUA: false,
			existingUA:  "Original/1.0",
			mockSetup: func(m *mocks.MockFetcher) {
				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Header.Get("User-Agent") == "Original/1.0"
				})).Return(&http.Response{StatusCode: 200}, nil)
			},
		},
		{
			name:        "No UA, Random Enabled -> Inject Custom UA",
			userAgents:  customUAs,
			useRandomUA: true,
			existingUA:  "",
			mockSetup: func(m *mocks.MockFetcher) {
				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					ua := req.Header.Get("User-Agent")
					return ua == "Custom/1.0" || ua == "Custom/2.0"
				})).Return(&http.Response{StatusCode: 200}, nil)
			},
		},
		{
			name:        "No UA, Random Disabled -> No Injection",
			userAgents:  customUAs,
			useRandomUA: false,
			existingUA:  "",
			mockSetup: func(m *mocks.MockFetcher) {
				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Header.Get("User-Agent") == ""
				})).Return(&http.Response{StatusCode: 200}, nil)
			},
		},
		{
			name:        "No UA, No Custom List, Random Enabled -> Inject Common UA",
			userAgents:  nil, // Use defaults
			useRandomUA: true,
			existingUA:  "",
			mockSetup: func(m *mocks.MockFetcher) {
				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Header.Get("User-Agent") != ""
				})).Return(&http.Response{StatusCode: 200}, nil)
			},
		},
		{
			name:        "Delegate Error Propagation",
			userAgents:  customUAs,
			useRandomUA: true,
			existingUA:  "",
			mockSetup: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Return(nil, errors.New("delegate error"))
			},
			expectedError: errors.New("delegate error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFetcher := new(mocks.MockFetcher)
			if tt.mockSetup != nil {
				tt.mockSetup(mockFetcher)
			}

			f := fetcher.NewUserAgentFetcher(mockFetcher, tt.userAgents, tt.useRandomUA)

			req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
			if tt.existingUA != "" {
				req.Header.Set("User-Agent", tt.existingUA)
			}

			_, err := f.Do(req)

			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}

			mockFetcher.AssertExpectations(t)
		})
	}
}

func TestUserAgentFetcher_RandomSelection(t *testing.T) {
	mockFetcher := new(mocks.MockFetcher)
	customUAs := []string{"UA1", "UA2", "UA3"}

	// Mock 동작 설정: 호출될 때마다 User-Agent 수집
	capturedUAs := make([]string, 0)
	var mu sync.Mutex

	mockFetcher.On("Do", mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(0).(*http.Request)
		mu.Lock()
		capturedUAs = append(capturedUAs, req.Header.Get("User-Agent"))
		mu.Unlock()
	}).Return(&http.Response{StatusCode: 200}, nil)

	f := fetcher.NewUserAgentFetcher(mockFetcher, customUAs, true)

	// 충분한 횟수 반복 실행
	iterations := 300
	for i := 0; i < iterations; i++ {
		req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
		f.Do(req)
	}

	// 검증
	counts := make(map[string]int)
	for _, ua := range capturedUAs {
		counts[ua]++
	}

	assert.Equal(t, len(customUAs), len(counts), "모든 User-Agent가 최소한 한번은 선택되어야 함")
	for _, ua := range customUAs {
		count := counts[ua]
		// 300번 시행에 3개 후보면 기대값 100.
		// 극단적인 확률 제외하고 최소 50번 이상은 나와야 정상 분포로 간주
		assert.GreaterOrEqual(t, count, 50, "User-Agent %s 선택 빈도가 너무 낮음 (%d)", ua, count)
	}
}

func TestUserAgentFetcher_Concurrency(t *testing.T) {
	mockFetcher := new(mocks.MockFetcher)
	mockFetcher.On("Do", mock.Anything).Return(&http.Response{StatusCode: 200}, nil)

	f := fetcher.NewUserAgentFetcher(mockFetcher, []string{"UA1", "UA2"}, true)

	var wg sync.WaitGroup
	count := 100
	wg.Add(count)

	for i := 0; i < count; i++ {
		go func() {
			defer wg.Done()
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
			f.Do(req)
		}()
	}

	wg.Wait()
	// Race detector가 실행 중에 경쟁 상태를 감지할 것임
}

func TestUserAgentFetcher_EmptyPool_RandomEnabled(t *testing.T) {
	// Custom list empty -> Should use Common User Agents
	mockFetcher := new(mocks.MockFetcher)
	mockFetcher.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Header.Get("User-Agent") != "" // Should have SOME UA
	})).Return(&http.Response{StatusCode: 200}, nil)

	f := fetcher.NewUserAgentFetcher(mockFetcher, []string{}, true) // Empty list
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	f.Do(req)

	mockFetcher.AssertExpectations(t)
}
