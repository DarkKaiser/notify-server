package testutil

import (
	"context"
	"sync"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/contract"
)

// StubTask는 테스트용 Task (Shell/Stub) 구현체입니다.
// 실제 로직 없이 실행 요청을 대기하거나 취소 신호를 처리하는 용도로 사용됩니다.
type StubTask struct {
	id         contract.TaskID
	commandID  contract.TaskCommandID
	instanceID contract.TaskInstanceID
	Canceled   bool
	CancelC    chan struct{}
	CancelOnce sync.Once
}

func NewStubTask(id contract.TaskID, cmdID contract.TaskCommandID, instID contract.TaskInstanceID) *StubTask {
	return &StubTask{
		id:         id,
		commandID:  cmdID,
		instanceID: instID,
		CancelC:    make(chan struct{}),
	}
}

func (h *StubTask) ID() contract.TaskID                 { return h.id }
func (h *StubTask) CommandID() contract.TaskCommandID   { return h.commandID }
func (h *StubTask) InstanceID() contract.TaskInstanceID { return h.instanceID }
func (h *StubTask) NotifierID() contract.NotifierID {
	return contract.NotifierID("test-notifier")
}
func (h *StubTask) IsCanceled() bool       { return h.Canceled }
func (h *StubTask) Elapsed() time.Duration { return 0 }

func (s *StubTask) Run(ctx context.Context, ns contract.NotificationSender) {
	select {
	case <-ctx.Done():
	case <-s.CancelC:
	}
}

func (h *StubTask) Cancel() {
	h.CancelOnce.Do(func() {
		h.Canceled = true
		close(h.CancelC)
	})
}
