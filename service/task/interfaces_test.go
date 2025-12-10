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
func (m *MockNotificationSender) Notify(notifierID string, message string, taskCtx TaskContext) bool {
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
