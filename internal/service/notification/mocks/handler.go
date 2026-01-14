package mocks

import (
	"context"
	"sync"

	"github.com/darkkaiser/notify-server/internal/service/notification/types"
	"github.com/darkkaiser/notify-server/internal/service/task"
)

// MockNotifierHandler는 NotifierHandler 인터페이스의 Mock 구현체입니다.
//
// 이 Mock은 알림 전송 동작을 테스트하는 데 사용되며,
// 실제 알림 전송 없이 호출 기록을 추적합니다.
type MockNotifierHandler struct {
	IDValue           types.NotifierID
	SupportsHTMLValue bool
	NotifyCalls       []MockNotifyCall
	Mu                sync.Mutex // NotifyCalls 동시성 보호
	DoneChannel       chan struct{}
}

// MockNotifyCall은 Notify 메서드 호출 기록을 저장합니다.
type MockNotifyCall struct {
	Message string
	TaskCtx task.TaskContext
}

// ID는 Notifier의 고유 식별자를 반환합니다.
func (m *MockNotifierHandler) ID() types.NotifierID {
	return m.IDValue
}

// Notify는 알림 메시지를 전송하고 호출 기록을 저장합니다.
// 동시성 안전을 위해 mutex로 보호됩니다.
func (m *MockNotifierHandler) Notify(taskCtx task.TaskContext, message string) bool {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	m.NotifyCalls = append(m.NotifyCalls, MockNotifyCall{
		Message: message,
		TaskCtx: taskCtx,
	})
	return true
}

// Run은 Notifier를 실행하고 context가 종료될 때까지 대기합니다.
func (m *MockNotifierHandler) Run(notificationStopCtx context.Context) {
	<-notificationStopCtx.Done()
}

// SupportsHTML은 HTML 형식 메시지 지원 여부를 반환합니다.
func (m *MockNotifierHandler) SupportsHTML() bool {
	return m.SupportsHTMLValue
}

// Done은 Notifier 종료 채널을 반환합니다.
func (m *MockNotifierHandler) Done() <-chan struct{} {
	if m.DoneChannel == nil {
		// 기본적으로 닫혀있지 않은(종료되지 않은) 채널 반환
		m.DoneChannel = make(chan struct{})
	}
	return m.DoneChannel
}
