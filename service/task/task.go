package task

import (
	"fmt"
	"sync"
	"time"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	log "github.com/sirupsen/logrus"
)

// TaskRunFunc
type TaskRunFunc func(interface{}, bool) (string, interface{}, error)

type Task struct {
	ID         ID
	CommandID  CommandID
	InstanceID InstanceID

	NotifierID string

	Canceled bool

	RunBy   RunBy
	RunTime time.Time

	RunFn TaskRunFunc

	Storage TaskResultStorage

	Fetcher Fetcher
}

type TaskHandler interface {
	GetID() ID
	GetCommandID() CommandID
	GetInstanceID() InstanceID

	GetNotifierID() string

	Cancel()
	IsCanceled() bool

	ElapsedTimeAfterRun() int64

	SetStorage(storage TaskResultStorage)

	// Run 작업 실행 메서드입니다. TaskContext를 통해 메타데이터를 전달받습니다.
	Run(taskCtx TaskContext, notificationSender NotificationSender, taskStopWaiter *sync.WaitGroup, taskDoneC chan<- InstanceID)
}

func (t *Task) GetID() ID {
	return t.ID
}

func (t *Task) GetCommandID() CommandID {
	return t.CommandID
}

func (t *Task) GetInstanceID() InstanceID {
	return t.InstanceID
}

func (t *Task) GetNotifierID() string {
	return t.NotifierID
}

func (t *Task) Cancel() {
	t.Canceled = true
}

func (t *Task) IsCanceled() bool {
	return t.Canceled
}

func (t *Task) ElapsedTimeAfterRun() int64 {
	return int64(time.Since(t.RunTime).Seconds())
}

func (t *Task) SetStorage(storage TaskResultStorage) {
	t.Storage = storage
}

func (t *Task) Run(taskCtx TaskContext, notificationSender NotificationSender, taskStopWaiter *sync.WaitGroup, taskDoneC chan<- InstanceID) {
	const errString = "작업 진행중 오류가 발생하여 작업이 실패하였습니다.😱"

	defer taskStopWaiter.Done()
	defer func() {
		taskDoneC <- t.InstanceID
	}()

	t.RunTime = time.Now()

	if t.RunFn == nil {
		m := fmt.Sprintf("%s\n\n☑ runFn()이 초기화되지 않았습니다.", errString)

		applog.WithComponentAndFields("task.executor", log.Fields{
			"task_id":    t.GetID(),
			"command_id": t.GetCommandID(),
		}).Error(m)

		t.notifyError(notificationSender, m, taskCtx)

		return
	}

	// TaskResultData를 초기화하고 읽어들인다.
	var taskResultData interface{}
	searchResult, cfgErr := findConfig(t.GetID(), t.GetCommandID())
	if cfgErr == nil {
		taskResultData = searchResult.Command.NewTaskResultDataFn()
	}
	if taskResultData == nil {
		m := fmt.Sprintf("%s\n\n☑ 작업결과데이터 생성이 실패하였습니다.", errString)

		applog.WithComponentAndFields("task.executor", log.Fields{
			"task_id":    t.GetID(),
			"command_id": t.GetCommandID(),
		}).Error(m)

		t.notifyError(notificationSender, m, taskCtx)

		return
	}

	// Storage가 초기화되지 않았을 경우에 대한 방어 로직
	if t.Storage == nil {
		// 하위 호환성을 위해 nil이면 에러 로깅 후 종료하거나 기본 파일 스토리지를 쓸 수도 있지만,
		// 리팩토링의 목적상 명시적으로 에러 처리합니다.
		m := fmt.Sprintf("%s\n\n☑ Storage가 초기화되지 않았습니다.", errString)
		applog.WithComponentAndFields("task.executor", log.Fields{
			"task_id":    t.GetID(),
			"command_id": t.GetCommandID(),
		}).Error(m)
		t.notifyError(notificationSender, m, taskCtx)
		return
	}

	err := t.Storage.Load(t.GetID(), t.GetCommandID(), taskResultData)
	if err != nil {
		m := fmt.Sprintf("이전 작업결과데이터 로딩이 실패하였습니다.😱\n\n☑ %s\n\n빈 작업결과데이터를 이용하여 작업을 계속 진행합니다.", err)

		applog.WithComponentAndFields("task.executor", log.Fields{
			"task_id":    t.GetID(),
			"command_id": t.GetCommandID(),
			"error":      err,
		}).Warn(m)

		t.notify(notificationSender, m, taskCtx)
	}

	if message, changedTaskResultData, err := t.RunFn(taskResultData, notificationSender.SupportsHTML(t.NotifierID)); t.IsCanceled() == false {
		if err == nil {
			if len(message) > 0 {
				t.notify(notificationSender, message, taskCtx)
			}

			if changedTaskResultData != nil {
				if err := t.Storage.Save(t.GetID(), t.GetCommandID(), changedTaskResultData); err != nil {
					m := fmt.Sprintf("작업이 끝난 작업결과데이터의 저장이 실패하였습니다.😱\n\n☑ %s", err)

					applog.WithComponentAndFields("task.executor", log.Fields{
						"task_id":    t.GetID(),
						"command_id": t.GetCommandID(),
						"error":      err,
					}).Warn(m)

					t.notifyError(notificationSender, m, taskCtx)
				}
			}
		} else {
			m := fmt.Sprintf("%s\n\n☑ %s", errString, err)

			applog.WithComponentAndFields("task.executor", log.Fields{
				"task_id":    t.GetID(),
				"command_id": t.GetCommandID(),
				"error":      err,
			}).Error(m)

			t.notifyError(notificationSender, m, taskCtx)

			return
		}
	}
}

func (t *Task) notify(notificationSender NotificationSender, m string, taskCtx TaskContext) bool {
	return notificationSender.Notify(taskCtx, t.GetNotifierID(), m)
}

func (t *Task) notifyError(notificationSender NotificationSender, m string, taskCtx TaskContext) bool {
	return notificationSender.Notify(taskCtx.WithError(), t.GetNotifierID(), m)
}
