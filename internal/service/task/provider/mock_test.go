package provider

import (
	"bytes"
	"io"
	"net/http"
	"sync"

	notificationmocks "github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/stretchr/testify/mock"
)

// Deprecated: Use notificationmocks.MockNotificationSender instead.
// We keep this alias for compatibility if needed, but optimally should replace usages.
type MockNotificationSender = notificationmocks.MockNotificationSender

// TestMockFetcher Fetcher 인터페이스의 Mock 구현체 (Testify 사용)
// 여러 테스트 파일에서 공통으로 사용됩니다.
type TestMockFetcher struct {
	mock.Mock
}

func (m *TestMockFetcher) Get(url string) (*http.Response, error) {
	args := m.Called(url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

func (m *TestMockFetcher) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// NewMockResponse 주어진 body와 status code를 가진 새로운 http.Response를 생성합니다.
// Test Helper 함수
func NewMockResponse(body string, statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}

// NewMockResponseWithJSON 주어진 JSON body와 status code를 가진 새로운 http.Response를 생성합니다.
// Test Helper 함수
func NewMockResponseWithJSON(jsonBody string, statusCode int) *http.Response {
	resp := NewMockResponse(jsonBody, statusCode)
	resp.Header.Set("Content-Type", "application/json")
	return resp
}

// --------------- Copied for Internal Usage (Avoiding Cyclic Dependency) ----------------

// --------------- Copied for Internal Usage (Avoiding Cyclic Dependency) ----------------

// MockHTTPFetcher 테스트용 Mock Fetcher (sync.Mutex 기반)
type MockHTTPFetcher struct {
	mu sync.Mutex

	// URL별 응답 설정
	Responses map[string][]byte // URL -> 응답 바이트
	Errors    map[string]error  // URL -> 에러

	// 호출 기록
	RequestedURLs []string
}

// NewMockHTTPFetcher 새로운 MockHTTPFetcher를 생성합니다.
func NewMockHTTPFetcher() *MockHTTPFetcher {
	return &MockHTTPFetcher{
		Responses:     make(map[string][]byte),
		Errors:        make(map[string]error),
		RequestedURLs: make([]string, 0),
	}
}

// SetResponse 특정 URL에 대한 응답을 설정합니다.
func (m *MockHTTPFetcher) SetResponse(url string, response []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Responses[url] = response
}

// SetError 특정 URL에 대한 에러를 설정합니다.
func (m *MockHTTPFetcher) SetError(url string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Errors[url] = err
}

// Get Mock HTTP Get 요청을 수행합니다.
func (m *MockHTTPFetcher) Get(url string) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 호출 기록 저장
	m.RequestedURLs = append(m.RequestedURLs, url)

	// 에러가 설정되어 있으면 에러 반환
	if err, ok := m.Errors[url]; ok {
		return nil, err
	}

	// 응답이 설정되어 있으면 응답 반환
	if responseBody, ok := m.Responses[url]; ok {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(responseBody)),
		}, nil
	}

	// 설정되지 않은 URL은 404 반환 (또는 빈 응답)
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(bytes.NewReader([]byte{})),
	}, nil
}

// Do Mock HTTP 요청을 수행합니다.
func (m *MockHTTPFetcher) Do(req *http.Request) (*http.Response, error) {
	return m.Get(req.URL.String())
}

// GetRequestedURLs 요청된 URL 목록을 반환합니다.
func (m *MockHTTPFetcher) GetRequestedURLs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	urls := make([]string, len(m.RequestedURLs))
	copy(urls, m.RequestedURLs)
	return urls
}

// Reset 모든 설정과 기록을 초기화합니다.
func (m *MockHTTPFetcher) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Responses = make(map[string][]byte)
	m.Errors = make(map[string]error)
	m.RequestedURLs = make([]string, 0)
}
