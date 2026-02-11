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
	ID         contract.TaskID
	CommandID  contract.TaskCommandID
	InstanceID contract.TaskInstanceID
	Canceled   bool
	CancelC    chan struct{}
	CancelOnce sync.Once
}

func NewStubTask(id contract.TaskID, cmdID contract.TaskCommandID, instID contract.TaskInstanceID) *StubTask {
	return &StubTask{
		ID:         id,
		CommandID:  cmdID,
		InstanceID: instID,
		CancelC:    make(chan struct{}),
	}
}

func (h *StubTask) GetID() contract.TaskID                 { return h.ID }
func (h *StubTask) GetCommandID() contract.TaskCommandID   { return h.CommandID }
func (h *StubTask) GetInstanceID() contract.TaskInstanceID { return h.InstanceID }
func (h *StubTask) GetNotifierID() contract.NotifierID {
	return contract.NotifierID("test-notifier")
}
func (h *StubTask) IsCanceled() bool                   { return h.Canceled }
func (h *StubTask) ElapsedTimeAfterRun() time.Duration { return 0 }

func (h *StubTask) Run(ctx context.Context, notificationSender contract.NotificationSender, taskStopWG *sync.WaitGroup, taskDoneC chan<- contract.TaskInstanceID) {
	defer taskStopWG.Done()

	select {
	case <-ctx.Done():
	case <-h.CancelC:
	}

	taskDoneC <- h.InstanceID
}

func (h *StubTask) Cancel() {
	h.CancelOnce.Do(func() {
		h.Canceled = true
		close(h.CancelC)
	})
}
