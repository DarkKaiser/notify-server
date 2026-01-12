package mocks

import (
	"sync"

	"github.com/darkkaiser/notify-server/internal/service/task"
)

// MockNotificationSender 테스트용 NotificationSender 구현체입니다.
type MockNotificationSender struct {
	mu sync.Mutex

	NotifyCalled      bool
	LastNotifierID    string
	LastTitle         string
	LastMessage       string
	LastErrorOccurred bool
	ShouldFail        bool

	NotifyDefaultCalled bool

	// 호출 기록 (MockNotificationSender from task package)
	NotifyDefaultCalls      []string
	NotifyCalls             []NotifyCall
	CapturedContexts        []task.TaskContext
	SupportsHTMLCalls       []string
	SupportsHTMLReturnValue bool
}

// NotifyCall Notify 호출 정보를 저장합니다.
type NotifyCall struct {
	NotifierID  string
	Message     string
	TaskContext task.TaskContext
}

// NewMockNotificationSender 새로운 Mock 객체를 생성합니다.
func NewMockNotificationSender() *MockNotificationSender {
	return &MockNotificationSender{
		NotifyDefaultCalls:      make([]string, 0),
		NotifyCalls:             make([]NotifyCall, 0),
		CapturedContexts:        make([]task.TaskContext, 0),
		SupportsHTMLCalls:       make([]string, 0),
		SupportsHTMLReturnValue: true, // 기본값: HTML 지원
	}
}

// NotifyWithTitle 지정된 NotifierID로 제목이 포함된 알림 메시지를 발송합니다.
func (m *MockNotificationSender) NotifyWithTitle(notifierID string, title string, message string, errorOccurred bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NotifyCalled = true
	m.LastNotifierID = notifierID
	m.LastTitle = title
	m.LastMessage = message
	m.LastErrorOccurred = errorOccurred
	return !m.ShouldFail
}

// NotifyDefault 기본 알림을 전송합니다.
func (m *MockNotificationSender) NotifyDefault(message string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// api 패키지 사용성
	m.NotifyDefaultCalled = true
	m.LastMessage = message

	// task 패키지 사용성
	m.NotifyDefaultCalls = append(m.NotifyDefaultCalls, message)
	return true
}

// NotifyDefaultWithError 시스템 기본 알림 채널로 "오류" 성격의 알림 메시지를 발송합니다.
func (m *MockNotificationSender) NotifyDefaultWithError(message string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NotifyDefaultCalled = true
	m.LastMessage = message
	m.LastErrorOccurred = true
	return true
}

// Notify Task 컨텍스트와 함께 알림을 전송합니다.
func (m *MockNotificationSender) Notify(taskCtx task.TaskContext, notifierID string, message string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NotifyCalls = append(m.NotifyCalls, NotifyCall{
		NotifierID:  notifierID,
		Message:     message,
		TaskContext: taskCtx,
	})
	m.CapturedContexts = append(m.CapturedContexts, taskCtx)
	return true
}

// SupportsHTML HTML 메시지 지원 여부를 반환합니다.
func (m *MockNotificationSender) SupportsHTML(notifierID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SupportsHTMLCalls = append(m.SupportsHTMLCalls, notifierID)
	return m.SupportsHTMLReturnValue
}

// Reset 상태를 초기화합니다.
func (m *MockNotificationSender) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NotifyCalled = false
	m.LastNotifierID = ""
	m.LastTitle = ""
	m.LastMessage = ""
	m.LastErrorOccurred = false
	m.ShouldFail = false
	m.NotifyDefaultCalled = false

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

// WasNotifyDefaultCalled NotifyDefault (또는 WithError)가 호출되었는지 반환합니다.
func (m *MockNotificationSender) WasNotifyDefaultCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.NotifyDefaultCalled
}
