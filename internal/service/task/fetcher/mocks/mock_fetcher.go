// Package mocks는 fetcher 패키지의 테스트를 위한 Mock 구현체들을 제공합니다.
//
// # Mock 구현체 선택 가이드
//
// 이 패키지는 두 가지 주요 Mock 구현체를 제공합니다:
//
// 1. MockFetcher (testify/mock 기반)
//   - 용도: 정교한 Mock 검증이 필요한 단위 테스트
//   - 장점: 메서드 호출 횟수, 인자 검증 등 상세한 Mock 동작 제어 가능
//   - 단점: 설정이 다소 복잡함
//   - 사용 예:
//     mockFetcher := mocks.NewMockFetcher()
//     mockFetcher.On("Get", mock.Anything, "https://example.com").Return(resp, nil)
//
// 2. MockHTTPFetcher (수동 구현)
//   - 용도: 통합 테스트, 벤치마크, 복잡한 시나리오 시뮬레이션
//   - 장점: URL별 응답/에러/지연 설정 가능, 간단한 설정, Thread-Safe
//   - 단점: 호출 검증 기능이 제한적 (기본적인 호출 횟수 확인 등은 지원)
//   - 사용 예:
//     mockFetcher := mocks.NewMockHTTPFetcher()
//     mockFetcher.SetResponse("https://example.com", []byte("response"))
//     mockFetcher.SetDelay("https://slow.com", 100*time.Millisecond)
//
// # 동시성 안전성
//
//   - MockFetcher: testify/mock 패키지가 내부적으로 동시성을 처리합니다.
//   - MockHTTPFetcher: sync.Mutex를 사용하여 완벽한 동시성 안전성을 보장합니다.
//   - MockReadCloser: atomic 연산을 사용하여 Close() 호출에 대한 동시성 안전성을 보장합니다.
package mocks

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/stretchr/testify/mock"
)

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ fetcher.Fetcher = (*MockFetcher)(nil)
var _ fetcher.Fetcher = (*MockHTTPFetcher)(nil)
var _ io.ReadCloser = (*MockReadCloser)(nil)

// ----------------------------------------------------------------------------
// MockFetcher (testify/mock 기반)
// ----------------------------------------------------------------------------

// MockFetcher Fetcher 인터페이스의 Mock 구현체 (Testify 사용)
type MockFetcher struct {
	mock.Mock
}

// NewMockFetcher 새로운 MockFetcher 인스턴스를 생성합니다.
func NewMockFetcher() *MockFetcher {
	return &MockFetcher{}
}

func (m *MockFetcher) Get(ctx context.Context, url string) (*http.Response, error) {
	args := m.Called(ctx, url)
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
//
// 이 함수는 테스트에서 간단한 HTTP 응답을 생성할 때 사용됩니다.
// Body는 io.NopCloser로 래핑되어 Close() 호출 시 아무 동작도 하지 않습니다.
func NewMockResponse(body string, statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}

// NewMockResponseWithJSON 주어진 JSON body와 status code를 가진 새로운 http.Response를 생성합니다.
//
// Content-Type 헤더가 자동으로 "application/json"으로 설정됩니다.
func NewMockResponseWithJSON(jsonBody string, statusCode int) *http.Response {
	resp := NewMockResponse(jsonBody, statusCode)
	resp.Header.Set("Content-Type", "application/json")
	return resp
}

// ----------------------------------------------------------------------------
// MockHTTPFetcher (수동 구현, Thread-Safe)
// ----------------------------------------------------------------------------

type mockResponse struct {
	body       []byte
	statusCode int
	header     http.Header
}

// MockHTTPFetcher 테스트용 Mock Fetcher (sync.Mutex 기반)
// 복잡한 동작(응답 지연, 에러 주입, 상태 코드 등)을 시뮬레이션하기 위해 사용됩니다.
type MockHTTPFetcher struct {
	mu            sync.Mutex
	responses     map[string]mockResponse
	errors        map[string]error
	delays        map[string]time.Duration
	requestedURLs []string
}

// NewMockHTTPFetcher 새로운 MockHTTPFetcher를 생성합니다.
func NewMockHTTPFetcher() *MockHTTPFetcher {
	return &MockHTTPFetcher{
		responses:     make(map[string]mockResponse),
		errors:        make(map[string]error),
		delays:        make(map[string]time.Duration),
		requestedURLs: make([]string, 0),
	}
}

// SetResponse 특정 URL에 대한 성공 응답(200 OK)을 설정합니다.
func (m *MockHTTPFetcher) SetResponse(url string, body []byte) {
	m.SetResponseWithStatus(url, body, http.StatusOK)
}

// SetResponseWithStatus 특정 URL에 대한 응답 Body와 Status Code를 설정합니다.
func (m *MockHTTPFetcher) SetResponseWithStatus(url string, body []byte, statusCode int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	resp := m.responses[url]
	resp.body = body
	resp.statusCode = statusCode
	if resp.header == nil {
		resp.header = make(http.Header)
	}
	m.responses[url] = resp
}

// SetHeader 특정 URL 응답에 헤더를 설정합니다.
// SetResponse나 SetResponseWithStatus가 먼저 호출되어 있어야 합니다.
// (호출되지 않았다면 기본 200 OK 응답으로 초기화된 후 헤더가 설정됩니다)
func (m *MockHTTPFetcher) SetHeader(url string, key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	resp, exists := m.responses[url]
	if !exists {
		resp = mockResponse{
			statusCode: http.StatusOK,
			header:     make(http.Header),
		}
	}
	if resp.header == nil {
		resp.header = make(http.Header)
	}
	resp.header.Set(key, value)
	m.responses[url] = resp
}

// SetError 특정 URL에 대한 에러를 설정합니다.
func (m *MockHTTPFetcher) SetError(url string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[url] = err
}

// SetDelay 특정 URL 요청 시 응답 지연 시간을 설정합니다.
func (m *MockHTTPFetcher) SetDelay(url string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delays[url] = d
}

// Get Mock HTTP Get 요청을 수행합니다.
// 내부적으로 Do를 호출하여 동작 일관성을 유지합니다.
func (m *MockHTTPFetcher) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return m.Do(req)
}

// Do Mock HTTP 요청을 수행합니다.
//
// 기능:
// - Context 취소 감지
// - 요청 URL 기록
// - 설정된 지연(Delay) 시뮬레이션
// - 설정된 에러 반환
// - 설정된 응답(Body, Status, Header) 반환
func (m *MockHTTPFetcher) Do(req *http.Request) (*http.Response, error) {
	// 1. 요청 시작 시점 Context 취소 확인
	if err := req.Context().Err(); err != nil {
		return nil, err
	}

	url := req.URL.String()

	m.mu.Lock()
	// 호출 기록
	m.requestedURLs = append(m.requestedURLs, url)

	// 설정 조회
	errVal := m.errors[url]
	respVal, hasResponse := m.responses[url]
	delayVal, hasDelay := m.delays[url]
	m.mu.Unlock()

	// 2. 지연 시뮬레이션 (Context 취소 감지 포함)
	if hasDelay {
		select {
		case <-time.After(delayVal):
			// 지연 완료
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}

	// 3. 에러 반환
	if errVal != nil {
		return nil, errVal
	}

	// 4. 응답 반환
	if hasResponse {
		resp := &http.Response{
			StatusCode: respVal.statusCode,
			Body:       io.NopCloser(bytes.NewReader(respVal.body)),
			Header:     resultHeader(respVal.header),
		}
		return resp, nil
	}

	// 5. 설정되지 않은 URL은 404 Not Found 반환
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(bytes.NewReader([]byte{})),
		Header:     make(http.Header),
	}, nil
}

// resultHeader 헤더 맵을 복사하여 반환합니다 (맵 참조 문제 방지).
func resultHeader(h http.Header) http.Header {
	if h == nil {
		return make(http.Header)
	}
	return h.Clone()
}

// GetRequestedURLs 요청된 URL 목록을 반환합니다.
func (m *MockHTTPFetcher) GetRequestedURLs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	urls := make([]string, len(m.requestedURLs))
	copy(urls, m.requestedURLs)
	return urls
}

// GetCallCount 특정 URL이 호출된 횟수를 반환합니다.
func (m *MockHTTPFetcher) GetCallCount(url string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, reqURL := range m.requestedURLs {
		if reqURL == url {
			count++
		}
	}
	return count
}

// Reset 모든 설정과 기록을 초기화합니다.
func (m *MockHTTPFetcher) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.responses = make(map[string]mockResponse)
	m.errors = make(map[string]error)
	m.delays = make(map[string]time.Duration)
	m.requestedURLs = make([]string, 0)
}

// ----------------------------------------------------------------------------
// MockReadCloser (io.ReadCloser 구현체)
// ----------------------------------------------------------------------------

// MockReadCloser io.ReadCloser 인터페이스를 구현하며, Close() 호출 여부를 추적합니다.
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

// NewMockReadCloserBytes 바이트 슬라이스 데이터를 가진 MockReadCloser를 생성합니다.
func NewMockReadCloserBytes(data []byte) *MockReadCloser {
	return &MockReadCloser{
		Data: bytes.NewBuffer(data),
	}
}

func (m *MockReadCloser) Read(p []byte) (n int, err error) {
	return m.Data.Read(p)
}

func (m *MockReadCloser) Close() error {
	atomic.AddInt64(&m.closeCount, 1)
	return nil
}

// GetCloseCount Close() 메서드가 호출된 횟수를 반환합니다.
func (m *MockReadCloser) GetCloseCount() int64 {
	return atomic.LoadInt64(&m.closeCount)
}
