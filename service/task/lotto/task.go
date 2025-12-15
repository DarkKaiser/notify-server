package lotto

import (
	"os"
	"strings"

	appconfig "github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
)

const (
	// TaskID
	ID tasksvc.ID = "LOTTO"

	// CommandID
	PredictionCommand tasksvc.CommandID = "Prediction" // 로또 번호 예측 명령
)

type taskConfig struct {
	AppPath string `json:"app_path"`
}

type predictionSnapshot struct{}

func init() {
	tasksvc.Register(ID, &tasksvc.Config{
		Commands: []*tasksvc.CommandConfig{{
			ID: PredictionCommand,

			AllowMultiple: false,

			NewSnapshot: func() interface{} { return &predictionSnapshot{} },
		}},

		NewTask: newTask,
	})
}

func newTask(instanceID tasksvc.InstanceID, req *tasksvc.SubmitRequest, appConfig *appconfig.AppConfig) (tasksvc.Handler, error) {
	return createTask(instanceID, req, appConfig, &defaultCommandExecutor{})
}

func createTask(instanceID tasksvc.InstanceID, req *tasksvc.SubmitRequest, appConfig *appconfig.AppConfig, executor commandExecutor) (tasksvc.Handler, error) {
	if req.TaskID != ID {
		return nil, apperrors.New(tasksvc.ErrTaskNotFound, "등록되지 않은 작업입니다.😱")
	}

	var appPath string
	for _, t := range appConfig.Tasks {
		if req.TaskID == tasksvc.ID(t.ID) {
			taskConfig := &taskConfig{}
			if err := tasksvc.DecodeMap(taskConfig, t.Data); err != nil {
				return nil, apperrors.Wrap(err, apperrors.ErrInvalidInput, "작업 데이터가 유효하지 않습니다")
			}

			appPath = strings.Trim(taskConfig.AppPath, " ")
			if appPath == "" {
				return nil, apperrors.New(apperrors.ErrInvalidInput, "Lotto Task의 AppPath 설정이 비어있습니다")
			}
			if _, err := os.Stat(appPath); os.IsNotExist(err) {
				return nil, apperrors.New(apperrors.ErrInvalidInput, "설정된 AppPath 경로가 존재하지 않습니다: "+appPath)
			}

			break
		}
	}

	lottoTask := &task{
		Task: tasksvc.NewBaseTask(req.TaskID, req.CommandID, instanceID, req.NotifierID, req.RunBy),

		appPath: appPath,

		executor: executor,
	}

	lottoTask.SetExecute(func(previousSnapshot interface{}, supportsHTML bool) (string, interface{}, error) {
		switch lottoTask.GetCommandID() {
		case PredictionCommand:
			return lottoTask.executePrediction()
		}

		return "", nil, tasksvc.ErrCommandNotImplemented
	})

	return lottoTask, nil
}

type task struct {
	tasksvc.Task

	appPath string

	executor commandExecutor
}
