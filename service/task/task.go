package task

import (
	"fmt"
	"sync"
	"time"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	log "github.com/sirupsen/logrus"
)

const (
	msgTaskExecutionFailed          = "작업 진행중 오류가 발생하여 작업이 실패하였습니다.😱"
	msgRunFnNotInitialized          = "runFn()이 초기화되지 않았습니다."
	msgTaskResultDataCreationFailed = "작업결과데이터 생성이 실패하였습니다."
	msgStorageNotInitialized        = "Storage가 초기화되지 않았습니다."
	msgPreviousDataLoadFailed       = "이전 작업결과데이터 로딩이 실패하였습니다.😱\n\n☑ %s\n\n빈 작업결과데이터를 이용하여 작업을 계속 진행합니다."
	msgCurrentDataSaveFailed        = "작업이 끝난 작업결과데이터의 저장이 실패하였습니다.😱\n\n☑ %s"
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
	defer taskStopWaiter.Done()
	defer func() {
		taskDoneC <- t.InstanceID
	}()

	t.RunTime = time.Now()

	if t.RunFn == nil {
		t.logAndNotifyError(taskCtx, notificationSender, msgRunFnNotInitialized)
		return
	}

	// TaskResultData를 초기화하고 읽어들인다.
	var taskResultData interface{}
	searchResult, cfgErr := findConfig(t.GetID(), t.GetCommandID())
	if cfgErr == nil {
		taskResultData = searchResult.Command.NewTaskResultDataFn()
	}
	if taskResultData == nil {
		t.logAndNotifyError(taskCtx, notificationSender, msgTaskResultDataCreationFailed)
		return
	}

	// Storage가 초기화되지 않았을 경우에 대한 방어 로직
	if t.Storage == nil {
		t.logAndNotifyError(taskCtx, notificationSender, msgStorageNotInitialized)
		return
	}

	err := t.Storage.Load(t.GetID(), t.GetCommandID(), taskResultData)
	if err != nil {
		m := fmt.Sprintf(msgPreviousDataLoadFailed, err)

		applog.WithComponentAndFields("task.executor", log.Fields{
			"task_id":    t.GetID(),
			"command_id": t.GetCommandID(),
			"error":      err,
		}).Warn(m)

		t.notify(taskCtx, notificationSender, m)
	}

	if message, changedTaskResultData, err := t.RunFn(taskResultData, notificationSender.SupportsHTML(t.NotifierID)); t.IsCanceled() == false {
		if err == nil {
			if len(message) > 0 {
				t.notify(taskCtx, notificationSender, message)
			}

			if changedTaskResultData != nil {
				if err := t.Storage.Save(t.GetID(), t.GetCommandID(), changedTaskResultData); err != nil {
					m := fmt.Sprintf(msgCurrentDataSaveFailed, err)

					applog.WithComponentAndFields("task.executor", log.Fields{
						"task_id":    t.GetID(),
						"command_id": t.GetCommandID(),
						"error":      err,
					}).Warn(m)

					t.notifyError(taskCtx, notificationSender, m)
				}
			}
		} else {
			t.logAndNotifyError(taskCtx, notificationSender, err.Error())
			return
		}
	}
}

func (t *Task) notify(taskCtx TaskContext, notificationSender NotificationSender, message string) bool {
	return notificationSender.Notify(taskCtx, t.GetNotifierID(), message)
}

func (t *Task) notifyError(taskCtx TaskContext, notificationSender NotificationSender, message string) bool {
	return notificationSender.Notify(taskCtx.WithError(), t.GetNotifierID(), message)
}

func (t *Task) logAndNotifyError(taskCtx TaskContext, notificationSender NotificationSender, cause string) {
	message := fmt.Sprintf("%s\n\n☑ %s", msgTaskExecutionFailed, cause)

	applog.WithComponentAndFields("task.executor", log.Fields{
		"task_id":    t.GetID(),
		"command_id": t.GetCommandID(),
	}).Error(message)

	t.notifyError(taskCtx, notificationSender, message)
}
