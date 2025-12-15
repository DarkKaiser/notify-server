package lotto

import (
	"strings"

	"github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/validation"
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

func newTask(instanceID tasksvc.InstanceID, req *tasksvc.SubmitRequest, appConfig *config.AppConfig) (tasksvc.Handler, error) {
	return createTask(instanceID, req, appConfig, &defaultCommandExecutor{})
}

func createTask(instanceID tasksvc.InstanceID, req *tasksvc.SubmitRequest, appConfig *config.AppConfig, executor commandExecutor) (tasksvc.Handler, error) {
	if req.TaskID != ID {
		return nil, apperrors.New(tasksvc.ErrTaskNotFound, "등록되지 않은 작업입니다.😱")
	}

	var appPath string
	found := false
	for _, t := range appConfig.Tasks {
		if req.TaskID == tasksvc.ID(t.ID) {
			taskConfig := &taskConfig{}
			if err := tasksvc.DecodeMap(taskConfig, t.Data); err != nil {
				return nil, apperrors.Wrap(err, apperrors.ErrInvalidInput, "작업 데이터가 유효하지 않습니다")
			}

			appPath = strings.TrimSpace(taskConfig.AppPath)
			if appPath == "" {
				return nil, apperrors.New(apperrors.ErrInvalidInput, "Lotto Task의 AppPath 설정이 비어있습니다")
			}
			if err := validation.ValidateFileExists(appPath, false); err != nil {
				return nil, apperrors.Wrap(err, apperrors.ErrInvalidInput, "AppPath 경로가 유효하지 않습니다")
			}

			found = true
			break
		}
	}

	if !found {
		return nil, apperrors.New(tasksvc.ErrTaskNotFound, "Lotto 작업을 위한 설정을 찾을 수 없습니다.")
	}

	lottoTask := &task{
		Task: tasksvc.NewBaseTask(req.TaskID, req.CommandID, instanceID, req.NotifierID, req.RunBy),

		appPath: appPath,

		executor: executor,
	}

	// CommandID에 따른 실행 함수를 미리 바인딩합니다 (Fail Fast)
	switch req.CommandID {
	case PredictionCommand:
		lottoTask.SetExecute(func(_ interface{}, _ bool) (string, interface{}, error) {
			return lottoTask.executePrediction()
		})
	default:
		return nil, apperrors.New(apperrors.ErrInvalidInput, "지원하지 않는 명령입니다: "+string(req.CommandID))
	}

	return lottoTask, nil
}

type task struct {
	tasksvc.Task

	appPath string

	executor commandExecutor
}
