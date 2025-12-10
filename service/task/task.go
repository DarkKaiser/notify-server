package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	log "github.com/sirupsen/logrus"
)

// supportedTasks
type NewTaskFunc func(InstanceID, *RunRequest, *config.AppConfig) (TaskHandler, error)
type NewTaskResultDataFunc func() interface{}

var supportedTasks = make(map[ID]*TaskConfig)

func RegisterTask(taskID ID, config *TaskConfig) {
	supportedTasks[taskID] = config
}

type TaskConfig struct {
	CommandConfigs []*TaskCommandConfig

	NewTaskFn NewTaskFunc
}

type TaskCommandConfig struct {
	TaskCommandID CommandID

	AllowMultipleInstances bool

	NewTaskResultDataFn NewTaskResultDataFunc
}

func (c *TaskCommandConfig) equalsTaskCommandID(taskCommandID CommandID) bool {
	return c.TaskCommandID.Match(taskCommandID)
}

func findConfigFromSupportedTask(taskID ID, taskCommandID CommandID) (*TaskConfig, *TaskCommandConfig, error) {
	taskConfig, exists := supportedTasks[taskID]
	if exists == true {
		for _, commandConfig := range taskConfig.CommandConfigs {
			if commandConfig.equalsTaskCommandID(taskCommandID) == true {
				return taskConfig, commandConfig, nil
			}
		}

		return nil, nil, ErrCommandNotSupported
	}

	return nil, nil, ErrTaskNotSupported
}

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

	Run(notificationSender NotificationSender, taskStopWaiter *sync.WaitGroup, taskDoneC chan<- InstanceID)
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

func (t *Task) Run(notificationSender NotificationSender, taskStopWaiter *sync.WaitGroup, taskDoneC chan<- InstanceID) {
	const errString = "작업 진행중 오류가 발생하여 작업이 실패하였습니다.😱"

	defer taskStopWaiter.Done()
	defer func() {
		taskDoneC <- t.InstanceID
	}()

	t.RunTime = time.Now()

	var taskCtx = NewTaskContext().WithTask(t.GetID(), t.GetCommandID())

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
	if taskConfig, exists := supportedTasks[t.GetID()]; exists == true {
		for _, commandConfig := range taskConfig.CommandConfigs {
			if commandConfig.equalsTaskCommandID(t.GetCommandID()) == true {
				taskResultData = commandConfig.NewTaskResultDataFn()
				break
			}
		}
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
	err := t.readTaskResultDataFromFile(taskResultData)
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
				if err := t.writeTaskResultDataToFile(changedTaskResultData); err != nil {
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
	return notificationSender.NotifyWithTaskContext(t.GetNotifierID(), m, taskCtx)
}

func (t *Task) notifyError(notificationSender NotificationSender, m string, taskCtx TaskContext) bool {
	return notificationSender.NotifyWithTaskContext(t.GetNotifierID(), m, taskCtx.WithError())
}

func (t *Task) dataFileName() string {
	filename := fmt.Sprintf("%s-task-%s-%s.json", config.AppName, strutil.ToSnakeCase(string(t.GetID())), strutil.ToSnakeCase(string(t.GetCommandID())))
	return strings.ReplaceAll(filename, "_", "-")
}

func (t *Task) readTaskResultDataFromFile(v interface{}) error {
	data, err := os.ReadFile(t.dataFileName())
	if err != nil {
		// 아직 데이터 파일이 생성되기 전이라면 nil을 반환한다.
		var pathError *os.PathError
		if errors.As(err, &pathError) == true {
			return nil
		}

		return apperrors.Wrap(err, apperrors.ErrInternal, "작업 결과 데이터 파일을 읽는데 실패했습니다")
	}

	return json.Unmarshal(data, v)
}

func (t *Task) writeTaskResultDataToFile(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return apperrors.Wrap(err, apperrors.ErrInternal, "작업 결과 데이터 마샬링에 실패했습니다")
	}

	if err := os.WriteFile(t.dataFileName(), data, os.FileMode(0644)); err != nil {
		return apperrors.Wrap(err, apperrors.ErrInternal, "작업 결과 데이터 파일 쓰기에 실패했습니다")
	}

	return nil
}
