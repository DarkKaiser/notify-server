package task

import (
	"bytes"
	"io"
	"net/http"
	"sync"

	"github.com/stretchr/testify/mock"
)

type MockTaskExecutor struct {
	mock.Mock
}

func (m *MockTaskExecutor) SubmitTask(req *SubmitRequest) error {
	args := m.Called(req)
	return args.Error(0)
}

func (m *MockTaskExecutor) CancelTask(instanceID InstanceID) error {
	args := m.Called(instanceID)
	return args.Error(0)
}

// MockNotificationSender is a mock implementation of NotificationSender interface
type MockTestifyNotificationSender struct {
	mock.Mock
}

func (m *MockTestifyNotificationSender) NotifyDefault(message string) bool {
	args := m.Called(message)
	return args.Bool(0)
}

func (m *MockTestifyNotificationSender) Notify(taskCtx TaskContext, notifierID string, message string) bool {
	args := m.Called(taskCtx, notifierID, message)
	// Return default true if return value not specified, or use args.Bool(0) if strict.
	// For most tests, we just want to verify call, return value matters less unless logic depends on it.
	// However, mock.Called returns Arguments, if I don't setup return, it might panic if accessing index 0?
	// Actually testify/mock returns zero values if not specified? No, it panics if expectation doesn't match return values count.
	// But usually we set .Return(true) etc.
	if len(args) > 0 {
		return args.Bool(0)
	}
	return true
}

func (m *MockTestifyNotificationSender) SupportsHTML(notifierID string) bool {
	args := m.Called(notifierID)
	if len(args) > 0 {
		return args.Bool(0)
	}
	return true
}

// MockNotificationSender 테스트용 NotificationSender 구현체입니다.
type MockNotificationSender struct {
	mu sync.Mutex

	// 호출 기록
	NotifyDefaultCalls      []string
	NotifyCalls             []NotifyCall
	SupportsHTMLCalls       []string
	SupportsHTMLReturnValue bool
}

// NotifyCall Notify 호출 정보를 저장합니다.
type NotifyCall struct {
	NotifierID  string
	Message     string
	TaskContext TaskContext
}

// NewMockNotificationSender 새로운 Mock 객체를 생성합니다.
func NewMockNotificationSender() *MockNotificationSender {
	return &MockNotificationSender{
		NotifyDefaultCalls:      make([]string, 0),
		NotifyCalls:             make([]NotifyCall, 0),
		SupportsHTMLCalls:       make([]string, 0),
		SupportsHTMLReturnValue: true, // 기본값: HTML 지원
	}
}

// NotifyDefault 기본 알림을 전송합니다 (Mock).
func (m *MockNotificationSender) NotifyDefault(message string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NotifyDefaultCalls = append(m.NotifyDefaultCalls, message)
	return true
}

// Notify Task 컨텍스트와 함께 알림을 전송합니다 (Mock).
func (m *MockNotificationSender) Notify(taskCtx TaskContext, notifierID string, message string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NotifyCalls = append(m.NotifyCalls, NotifyCall{
		NotifierID:  notifierID,
		Message:     message,
		TaskContext: taskCtx,
	})
	return true
}

// SupportsHTML HTML 메시지 지원 여부를 반환합니다 (Mock).
func (m *MockNotificationSender) SupportsHTML(notifierID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SupportsHTMLCalls = append(m.SupportsHTMLCalls, notifierID)
	return m.SupportsHTMLReturnValue
}

// Reset 모든 호출 기록을 초기화합니다.
func (m *MockNotificationSender) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NotifyDefaultCalls = make([]string, 0)
	m.NotifyCalls = make([]NotifyCall, 0)
	m.SupportsHTMLCalls = make([]string, 0)
}

// GetNotifyDefaultCallCount NotifyDefault 호출 횟수를 반환합니다.
func (m *MockNotificationSender) GetNotifyDefaultCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.NotifyDefaultCalls)
}

// GetNotifyCallCount Notify 호출 횟수를 반환합니다.
func (m *MockNotificationSender) GetNotifyCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.NotifyCalls)
}

// GetSupportsHTMLCallCount SupportsHTML 호출 횟수를 반환합니다.
func (m *MockNotificationSender) GetSupportsHTMLCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.SupportsHTMLCalls)
}

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

// MockTaskResultStorage 테스트용 Mock Storage
type MockTaskResultStorage struct {
	mock.Mock
}

func (m *MockTaskResultStorage) Get(taskID ID, commandID CommandID) (string, error) {
	args := m.Called(taskID, commandID)
	return args.String(0), args.Error(1)
}

func (m *MockTaskResultStorage) Save(taskID ID, commandID CommandID, data interface{}) error {
	args := m.Called(taskID, commandID, data)
	return args.Error(0)
}

func (m *MockTaskResultStorage) SetStorage(storage TaskResultStorage) {
	// Mock에서는 아무것도 하지 않음 or Mock 동작 정의
}

func (m *MockTaskResultStorage) Load(taskID ID, commandID CommandID, data interface{}) error {
	args := m.Called(taskID, commandID, data)
	return args.Error(0)
}

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
