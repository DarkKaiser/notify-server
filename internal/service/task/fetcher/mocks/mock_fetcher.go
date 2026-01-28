package mocks

import (
	"bytes"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/stretchr/testify/mock"
)

// Ensure MockFetcher and MockHTTPFetcher implement the Fetcher interface at compile-time.
var _ fetcher.Fetcher = (*MockFetcher)(nil)
var _ fetcher.Fetcher = (*MockHTTPFetcher)(nil)

// MockFetcher Fetcher 인터페이스의 Mock 구현체 (Testify 사용)
type MockFetcher struct {
	mock.Mock
}

func (m *MockFetcher) Get(url string) (*http.Response, error) {
	args := m.Called(url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

func (m *MockFetcher) Do(req *http.Request) (*http.Response, error) {
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
func NewMockResponseWithJSON(jsonBody string, statusCode int) *http.Response {
	resp := NewMockResponse(jsonBody, statusCode)
	resp.Header.Set("Content-Type", "application/json")
	return resp
}

// ----------------------------------------------------------------------------
// MockHTTPFetcher 테스트용 Mock Fetcher (sync.Mutex 기반)
// 복잡한 동작(응답 지연, 에러 주입 등)을 시뮬레이션하기 위해 사용됩니다.
// ----------------------------------------------------------------------------

type MockHTTPFetcher struct {
	mu            sync.Mutex
	Responses     map[string][]byte
	Errors        map[string]error
	Delays        map[string]time.Duration // URL별 지연 시간 설정
	RequestedURLs []string
}

// NewMockHTTPFetcher 새로운 MockHTTPFetcher를 생성합니다.
func NewMockHTTPFetcher() *MockHTTPFetcher {
	return &MockHTTPFetcher{
		Responses:     make(map[string][]byte),
		Errors:        make(map[string]error),
		Delays:        make(map[string]time.Duration),
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

// SetDelay 특정 URL 요청 시 응답 지연 시간을 설정합니다.
func (m *MockHTTPFetcher) SetDelay(url string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Delays[url] = d
}

// Get Mock HTTP Get 요청을 수행합니다.
// 내부적으로 Do를 호출하여 동작 일관성을 유지합니다.
func (m *MockHTTPFetcher) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return m.Do(req)
}

// Do Mock HTTP 요청을 수행합니다.
func (m *MockHTTPFetcher) Do(req *http.Request) (*http.Response, error) {
	url := req.URL.String()

	m.mu.Lock()

	// 호출 기록 저장
	m.RequestedURLs = append(m.RequestedURLs, url)

	// 에러 설정 확인
	err := m.Errors[url]

	// 응답 설정 확인
	responseBody, hasResponse := m.Responses[url]

	// 지연 설정 확인
	delay, hasDelay := m.Delays[url]

	m.mu.Unlock() // Lock 해제 후 Sleep (동시성 테스트 위해)

	if hasDelay {
		time.Sleep(delay)
	}

	if err != nil {
		return nil, err
	}

	if hasResponse {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(responseBody)),
		}, nil
	}

	// 설정되지 않은 URL은 404 반환
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(bytes.NewReader([]byte{})),
	}, nil
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
	m.Delays = make(map[string]time.Duration)
	m.RequestedURLs = make([]string, 0)
}

// MockReadCloser tracks calls to Close() in a concurrency-safe way.
type MockReadCloser struct {
	Data       *bytes.Buffer
	closeCount int64 // Atomic
}

// NewMockReadCloser 문자열 데이터를 가진 MockReadCloser를 생성합니다.
func NewMockReadCloser(data string) *MockReadCloser {
	return &MockReadCloser{
		Data: bytes.NewBufferString(data),
	}
}

func (m *MockReadCloser) Read(p []byte) (n int, err error) {
	return m.Data.Read(p)
}

func (m *MockReadCloser) Close() error {
	atomic.AddInt64(&m.closeCount, 1)
	return nil
}

// GetCloseCount returns the number of times Close() has been called.
func (m *MockReadCloser) GetCloseCount() int64 {
	return atomic.LoadInt64(&m.closeCount)
}
