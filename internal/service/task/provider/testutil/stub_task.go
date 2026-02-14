package testutil

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/contract"
)

// StubTask는 테스트용 Task (Shell/Stub) 구현체입니다.
// 실제 로직 없이 실행 요청을 대기하거나 취소 신호를 처리하는 용도로 사용됩니다.
type StubTask struct {
	id         contract.TaskID
	commandID  contract.TaskCommandID
	instanceID contract.TaskInstanceID

	// NotifierIDValue 설정된 경우 NotifierID() 메서드가 이 값을 반환합니다.
	// 기본값: "test-notifier"
	NotifierIDValue contract.NotifierID

	// RunFunc 설정된 경우 Run() 메서드 호출 시 실행됩니다.
	// 설정되지 않은 경우 기본적으로 블로킹(대기) 동작을 수행합니다.
	RunFunc func(ctx context.Context, ns contract.NotificationSender)

	// FixedElapsed 설정된 경우 Elapsed() 메서드가 항상 이 값을 반환합니다.
	FixedElapsed time.Duration

	// 상태 필드 (Thread-Safe)
	canceled   atomic.Bool
	cancelC    chan struct{}
	cancelOnce sync.Once
	runCalled  atomic.Int64
}

func NewStubTask(id contract.TaskID, cmdID contract.TaskCommandID, instID contract.TaskInstanceID) *StubTask {
	return &StubTask{
		id:         id,
		commandID:  cmdID,
		instanceID: instID,
		cancelC:    make(chan struct{}),
	}
}

func (s *StubTask) ID() contract.TaskID                 { return s.id }
func (s *StubTask) CommandID() contract.TaskCommandID   { return s.commandID }
func (s *StubTask) InstanceID() contract.TaskInstanceID { return s.instanceID }
func (s *StubTask) NotifierID() contract.NotifierID {
	if s.NotifierIDValue != "" {
		return s.NotifierIDValue
	}
	return contract.NotifierID("test-notifier")
}

func (s *StubTask) IsCanceled() bool {
	return s.canceled.Load()
}

func (s *StubTask) Elapsed() time.Duration {
	return s.FixedElapsed
}

// RunCount Run() 메서드가 호출된 횟수를 반환합니다.
func (s *StubTask) RunCount() int64 {
	return s.runCalled.Load()
}

func (s *StubTask) Run(ctx context.Context, ns contract.NotificationSender) {
	s.runCalled.Add(1)

	if s.RunFunc != nil {
		s.RunFunc(ctx, ns)
		return
	}

	// 기본 동작: 취소 또는 컨텍스트 종료 시까지 대기
	select {
	case <-ctx.Done():
	case <-s.cancelC:
	}
}

func (s *StubTask) Cancel() {
	s.cancelOnce.Do(func() {
		s.canceled.Store(true)
		close(s.cancelC)
	})
}
