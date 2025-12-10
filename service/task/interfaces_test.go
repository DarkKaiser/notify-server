package task

import (
	"sync"
	"testing"
)

// Compile-time checks to ensure types implement the interfaces.
var (
	// MockNotificationSender가 NotificationSender 인터페이스를 구현하는지 확인
	_ NotificationSender = (*MockNotificationSender)(nil)
)

func TestInterfaces(t *testing.T) {
	// 인터페이스 정의 자체를 테스트할 수는 없지만,
	// 주요 구현체들이 인터페이스를 준수하는지는 컴파일 타임 검사를 통해 보장합니다.
	// 위 var 블록에서 컴파일 에러가 발생하지 않는다면 테스트는 성공한 것으로 간주합니다.
}

// MockNotificationSender 테스트용 NotificationSender 구현체입니다.
type MockNotificationSender struct {
	mu sync.Mutex

	// 호출 기록
	NotifyToDefaultCalls           []string
	NotifyWithTaskContextCalls     []NotifyWithTaskContextCall
	SupportsHTMLMessageCalls       []string
	SupportsHTMLMessageReturnValue bool
}

// NotifyWithTaskContextCall NotifyWithTaskContext 호출 정보를 저장합니다.
type NotifyWithTaskContextCall struct {
	NotifierID  string
	Message     string
	TaskContext TaskContext
}

// NewMockNotificationSender 새로운 Mock 객체를 생성합니다.
func NewMockNotificationSender() *MockNotificationSender {
	return &MockNotificationSender{
		NotifyToDefaultCalls:           make([]string, 0),
		NotifyWithTaskContextCalls:     make([]NotifyWithTaskContextCall, 0),
		SupportsHTMLMessageCalls:       make([]string, 0),
		SupportsHTMLMessageReturnValue: true, // 기본값: HTML 지원
	}
}

// NotifyToDefault 기본 알림을 전송합니다 (Mock).
func (m *MockNotificationSender) NotifyToDefault(message string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NotifyToDefaultCalls = append(m.NotifyToDefaultCalls, message)
	return true
}

// NotifyWithTaskContext Task 컨텍스트와 함께 알림을 전송합니다 (Mock).
func (m *MockNotificationSender) NotifyWithTaskContext(notifierID string, message string, taskCtx TaskContext) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NotifyWithTaskContextCalls = append(m.NotifyWithTaskContextCalls, NotifyWithTaskContextCall{
		NotifierID:  notifierID,
		Message:     message,
		TaskContext: taskCtx,
	})
	return true
}

// SupportsHTMLMessage HTML 메시지 지원 여부를 반환합니다 (Mock).
func (m *MockNotificationSender) SupportsHTMLMessage(notifierID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SupportsHTMLMessageCalls = append(m.SupportsHTMLMessageCalls, notifierID)
	return m.SupportsHTMLMessageReturnValue
}

// Reset 모든 호출 기록을 초기화합니다.
func (m *MockNotificationSender) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NotifyToDefaultCalls = make([]string, 0)
	m.NotifyWithTaskContextCalls = make([]NotifyWithTaskContextCall, 0)
	m.SupportsHTMLMessageCalls = make([]string, 0)
}

// GetNotifyToDefaultCallCount NotifyToDefault 호출 횟수를 반환합니다.
func (m *MockNotificationSender) GetNotifyToDefaultCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.NotifyToDefaultCalls)
}

// GetNotifyWithTaskContextCallCount NotifyWithTaskContext 호출 횟수를 반환합니다.
func (m *MockNotificationSender) GetNotifyWithTaskContextCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.NotifyWithTaskContextCalls)
}

// GetSupportsHTMLMessageCallCount SupportsHTMLMessage 호출 횟수를 반환합니다.
func (m *MockNotificationSender) GetSupportsHTMLMessageCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.SupportsHTMLMessageCalls)
}
