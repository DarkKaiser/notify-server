package task

import (
	"bytes"
	"io"
	"net/http"

	"github.com/stretchr/testify/mock"
)

type MockTaskExecutor struct {
	mock.Mock
}

func (m *MockTaskExecutor) Run(req *RunRequest) error {
	args := m.Called(req)
	return args.Error(0)
}

func (m *MockTaskExecutor) Cancel(instanceID InstanceID) error {
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

func (m *MockTestifyNotificationSender) Notify(notifierID string, message string, taskCtx TaskContext) bool {
	args := m.Called(notifierID, message, taskCtx)
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
