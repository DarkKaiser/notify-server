package mocks

import (
	"context"
	"sync"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
)

// Interface Compliance Check
var _ notifier.Notifier = (*MockNotifier)(nil)

// NewMockNotifier 새로운 Mock 객체를 생성합니다.
func NewMockNotifier(id contract.NotifierID) *MockNotifier {
	return &MockNotifier{
		IDValue:           id,
		SupportsHTMLValue: true, // 기본값: HTML 지원
	}
}

// MockNotifier는 Notifier 인터페이스의 Mock 구현체입니다.
//
// 이 Mock은 알림 전송 동작을 테스트하는 데 사용되며,
// 실제 알림 전송 없이 호출 기록을 추적합니다.
type MockNotifier struct {
	IDValue           contract.NotifierID
	SupportsHTMLValue bool
	NotifyCalls       []MockNotifyCall
	Mu                sync.Mutex // NotifyCalls 동시성 보호
	DoneChannel       chan struct{}

	// Functional Options for Behavior Injection
	NotifyFunc func(contract.TaskContext, string) bool
	RunFunc    func(context.Context)
}

// MockNotifyCall은 Notify 메서드 호출 기록을 저장합니다.
type MockNotifyCall struct {
	Message string
	TaskCtx contract.TaskContext
}

// WithID ID를 설정합니다 (Fluent API).
func (m *MockNotifier) WithID(id contract.NotifierID) *MockNotifier {
	m.IDValue = id
	return m
}

// WithSupportsHTML HTML 지원 여부를 설정합니다 (Fluent API).
func (m *MockNotifier) WithSupportsHTML(supported bool) *MockNotifier {
	m.SupportsHTMLValue = supported
	return m
}

// WithNotifyFunc Notify 동작을 커스터마이징합니다 (Fluent API).
func (m *MockNotifier) WithNotifyFunc(fn func(contract.TaskContext, string) bool) *MockNotifier {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	m.NotifyFunc = fn
	return m
}

// WithRunFunc Run 동작을 커스터마이징합니다 (Fluent API).
func (m *MockNotifier) WithRunFunc(fn func(context.Context)) *MockNotifier {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	m.RunFunc = fn
	return m
}

// ID는 Notifier의 고유 식별자를 반환합니다.
func (m *MockNotifier) ID() contract.NotifierID {
	return m.IDValue
}

// Notify는 알림 메시지를 전송하고 호출 기록을 저장합니다.
// 동시성 안전을 위해 mutex로 보호됩니다.
func (m *MockNotifier) Notify(ctx contract.TaskContext, message string) bool {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	m.NotifyCalls = append(m.NotifyCalls, MockNotifyCall{
		Message: message,
		TaskCtx: ctx,
	})

	if m.NotifyFunc != nil {
		return m.NotifyFunc(ctx, message)
	}

	return true
}

// Run은 Notifier를 실행하고 context가 종료될 때까지 대기합니다.
func (m *MockNotifier) Run(notificationStopCtx context.Context) {
	m.Mu.Lock()
	runFn := m.RunFunc
	m.Mu.Unlock()

	if runFn != nil {
		runFn(notificationStopCtx)
		return
	}

	<-notificationStopCtx.Done()
}

// SupportsHTML은 HTML 형식 메시지 지원 여부를 반환합니다.
func (m *MockNotifier) SupportsHTML() bool {
	return m.SupportsHTMLValue
}

// Done은 Notifier 종료 채널을 반환합니다.
func (m *MockNotifier) Done() <-chan struct{} {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	if m.DoneChannel == nil {
		// 기본적으로 닫혀있지 않은(종료되지 않은) 채널 반환
		m.DoneChannel = make(chan struct{})
	}
	return m.DoneChannel
}

// Reset 상태를 초기화합니다.
func (m *MockNotifier) Reset() {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	m.NotifyCalls = nil
	// DoneChannel은 닫힌 상태인지 열린 상태인지 모호할 수 있으므로,
	// Reset 시 nil로 설정하여 다음 호출 시 새로 생성되게 함.
	m.DoneChannel = nil
	m.NotifyFunc = nil
	m.RunFunc = nil
}
