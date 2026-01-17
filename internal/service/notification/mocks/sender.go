package mocks

import (
	"sync"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/contract"
)

// MockNotificationSender 테스트용 NotificationSender 구현체입니다.
type MockNotificationSender struct {
	Mu sync.Mutex

	NotifyCalled      bool
	LastNotifierID    contract.NotifierID
	LastTitle         string
	LastMessage       string
	LastErrorOccurred bool
	ShouldFail        bool

	NotifyDefaultCalled bool

	// FailError 실패 시 반환할 에러 (nil이면 기본 MockError 반환)
	FailError error

	// NotifyFunc 커스텀 알림 처리 함수 (테스트 동기화 등에 사용)
	NotifyFunc func(contract.TaskContext, contract.NotifierID, string) error

	// 호출 기록 (MockNotificationSender from task package)
	NotifyDefaultCalls      []string
	NotifyCalls             []NotifyCall
	CapturedContexts        []contract.TaskContext
	SupportsHTMLCalls       []contract.NotifierID
	SupportsHTMLReturnValue bool
	HealthCalls             int
}

// NotifyCall Notify 호출 정보를 저장합니다.
type NotifyCall struct {
	NotifierID  contract.NotifierID
	Message     string
	TaskContext contract.TaskContext
}

type MockError struct {
	Message string
}

func (e *MockError) Error() string {
	return e.Message
}

// NewMockNotificationSender 새로운 Mock 객체를 생성합니다.
func NewMockNotificationSender() *MockNotificationSender {
	return &MockNotificationSender{
		NotifyDefaultCalls:      make([]string, 0),
		NotifyCalls:             make([]NotifyCall, 0),
		CapturedContexts:        make([]contract.TaskContext, 0),
		SupportsHTMLCalls:       make([]contract.NotifierID, 0),
		SupportsHTMLReturnValue: true, // 기본값: HTML 지원
	}
}

// NotifyWithTitle 지정된 NotifierID로 제목이 포함된 알림 메시지를 발송합니다.
func (m *MockNotificationSender) NotifyWithTitle(notifierID contract.NotifierID, title string, message string, errorOccurred bool) error {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	m.NotifyCalled = true
	m.LastNotifierID = notifierID
	m.LastTitle = title
	m.LastMessage = message
	m.LastErrorOccurred = errorOccurred

	if m.ShouldFail {
		if m.FailError != nil {
			return m.FailError
		}
		return &MockError{Message: "mock failure"}
	}
	return nil
}

// NotifyDefault 기본 알림을 전송합니다.
func (m *MockNotificationSender) NotifyDefault(message string) error {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	// api 패키지 사용성
	m.NotifyDefaultCalled = true
	m.LastMessage = message

	// task 패키지 사용성
	m.NotifyDefaultCalls = append(m.NotifyDefaultCalls, message)

	if m.ShouldFail {
		if m.FailError != nil {
			return m.FailError
		}
		return &MockError{Message: "mock failure"}
	}
	return nil
}

// NotifyDefaultWithError 시스템 기본 알림 채널로 "오류" 성격의 알림 메시지를 발송합니다.
func (m *MockNotificationSender) NotifyDefaultWithError(message string) error {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	m.NotifyDefaultCalled = true
	m.LastMessage = message
	m.LastErrorOccurred = true

	if m.ShouldFail {
		if m.FailError != nil {
			return m.FailError
		}
		return &MockError{Message: "mock failure"}
	}
	return nil
}

// SupportsHTML HTML 메시지 지원 여부를 반환합니다.
func (m *MockNotificationSender) SupportsHTML(notifierID contract.NotifierID) bool {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	m.SupportsHTMLCalls = append(m.SupportsHTMLCalls, notifierID)
	return m.SupportsHTMLReturnValue
}

// WithNotify Notify 호출 시 반환할 에러를 설정합니다.
func (m *MockNotificationSender) WithNotify(err error) *MockNotificationSender {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	m.ShouldFail = err != nil
	m.FailError = err
	return m
}

// WithNotifyFunc Notify 호출 시 실행할 커스텀 함수를 설정합니다.
// 이 함수가 nil이 아니면 기본 로직 대신 실행됩니다.
// NOTE: 현재 MockNotificationSender 구조체에 함수 필드가 없으므로 추가가 필요할 수 있습니다.
// 하지만 기존 factory 리팩토링과 달리 여기서는 ShouldFail/FailError 필드를 주로 사용합니다.
// 복잡한 로직이 필요한 경우를 위해 NotifyFunc 필드를 추가하는 것이 좋습니다.
// 일단 WithNotify만 구현하고, 추후 필요시 Func 필드를 추가합니다.
// -> 사용자 요청 "전문가 수준"이므로 Func 필드를 추가하는게 맞습니다.
// 하지만 replace_file_content로 전체 구조체를 건드리기엔 범위가 큽니다.
// 일단 WithNotify(err)만으로도 많은 케이스 커버가 가능합니다. verify는 이미 CallHistory로 가능합니다.

// WithNotifyFunc Notify 호출 시 실행할 커스텀 함수를 설정합니다.
func (m *MockNotificationSender) WithNotifyFunc(fn func(contract.TaskContext, contract.NotifierID, string) error) *MockNotificationSender {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	m.NotifyFunc = fn
	return m
}

// VerifyNotifyCalled Notify가 정확히 expected 횟수만큼 호출되었는지 검증합니다.
func (m *MockNotificationSender) VerifyNotifyCalled(t *testing.T, expected int) {
	t.Helper()
	m.Mu.Lock()
	defer m.Mu.Unlock()

	if len(m.NotifyCalls) != expected {
		t.Errorf("MockNotificationSender.Notify called %d times, expected %d", len(m.NotifyCalls), expected)
	}
}

// VerifyNotifyDefaultCalled NotifyDefault가 정확히 expected 횟수만큼 호출되었는지 검증합니다.
func (m *MockNotificationSender) VerifyNotifyDefaultCalled(t *testing.T, expected int) {
	t.Helper()
	m.Mu.Lock()
	defer m.Mu.Unlock()

	if len(m.NotifyDefaultCalls) != expected {
		t.Errorf("MockNotificationSender.NotifyDefault called %d times, expected %d", len(m.NotifyDefaultCalls), expected)
	}
}

// Notify Task 컨텍스트와 함께 알림을 전송합니다.
func (m *MockNotificationSender) Notify(taskCtx contract.TaskContext, notifierID contract.NotifierID, message string) error {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	m.NotifyCalls = append(m.NotifyCalls, NotifyCall{
		NotifierID:  notifierID,
		Message:     message,
		TaskContext: taskCtx,
	})
	m.CapturedContexts = append(m.CapturedContexts, taskCtx)

	if m.NotifyFunc != nil {
		return m.NotifyFunc(taskCtx, notifierID, message)
	}

	if m.ShouldFail {
		if m.FailError != nil {
			return m.FailError
		}
		return &MockError{Message: "mock failure"}
	}
	return nil
}

// Health 서비스의 건강 상태를 확인합니다.
func (m *MockNotificationSender) Health() error {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	m.HealthCalls++

	if m.ShouldFail {
		if m.FailError != nil {
			return m.FailError
		}
		return &MockError{Message: "mock failure"}
	}
	return nil
}

// Reset 상태를 초기화합니다.
func (m *MockNotificationSender) Reset() {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	m.NotifyCalled = false
	m.LastNotifierID = ""
	m.LastTitle = ""
	m.LastMessage = ""
	m.LastErrorOccurred = false
	m.ShouldFail = false
	m.FailError = nil // FailError 초기화
	m.NotifyDefaultCalled = false

	m.NotifyDefaultCalls = make([]string, 0)
	m.NotifyCalls = make([]NotifyCall, 0)
	m.SupportsHTMLCalls = make([]contract.NotifierID, 0)
	m.HealthCalls = 0
}

// GetNotifyDefaultCallCount NotifyDefault 호출 횟수를 반환합니다.
func (m *MockNotificationSender) GetNotifyDefaultCallCount() int {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	return len(m.NotifyDefaultCalls)
}

// GetNotifyCallCount Notify 호출 횟수를 반환합니다.
func (m *MockNotificationSender) GetNotifyCallCount() int {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	return len(m.NotifyCalls)
}

// GetSupportsHTMLCallCount SupportsHTML 호출 횟수를 반환합니다.
func (m *MockNotificationSender) GetSupportsHTMLCallCount() int {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	return len(m.SupportsHTMLCalls)
}

// WasNotifyDefaultCalled NotifyDefault (또는 WithError)가 호출되었는지 반환합니다.
func (m *MockNotificationSender) WasNotifyDefaultCalled() bool {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	return m.NotifyDefaultCalled
}
