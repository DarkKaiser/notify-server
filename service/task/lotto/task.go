package lotto

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/maputil"
	"github.com/darkkaiser/notify-server/pkg/validation"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
)

const (
	// TaskID
	ID tasksvc.ID = "LOTTO"

	// CommandID
	PredictionCommand tasksvc.CommandID = "Prediction" // 로또 번호 예측

	// jarFileName PredictionCommand 수행 시 실행되는 JAR 파일명
	jarFileName = "lottoprediction-1.0.0.jar"
)

var execLookPath = exec.LookPath

type taskSettings struct {
	AppPath string `json:"app_path"`
}

func (s *taskSettings) validate() error {
	s.AppPath = strings.TrimSpace(s.AppPath)
	if s.AppPath == "" {
		return apperrors.New(apperrors.InvalidInput, "'app_path'가 입력되지 않았거나 공백입니다")
	}

	// 절대 경로로 변환하여 실행 위치(CWD)에 독립적으로 만듭니다.
	absPath, err := filepath.Abs(s.AppPath)
	if err != nil {
		return apperrors.Wrap(err, apperrors.InvalidInput, "'app_path'에 대한 절대 경로 변환 처리에 실패하였습니다")
	}
	s.AppPath = absPath

	if err := validation.ValidateFileExists(s.AppPath, false); err != nil {
		return apperrors.Wrap(err, apperrors.InvalidInput, "'app_path'로 지정된 경로가 존재하지 않거나 유효하지 않습니다")
	}

	return nil
}

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
		return nil, tasksvc.ErrTaskNotSupported
	}

	var appPath string

	found := false
	for _, t := range appConfig.Tasks {
		if req.TaskID == tasksvc.ID(t.ID) {
			settings, err := maputil.Decode[taskSettings](t.Data)
			if err != nil {
				return nil, apperrors.Wrap(err, apperrors.InvalidInput, tasksvc.ErrInvalidTaskSettings.Error())
			}
			if err := settings.validate(); err != nil {
				return nil, err
			}

			appPath = settings.AppPath

			// JAR 파일 존재 여부 검증
			// 실제 실행 시점의 에러를 방지하기 위해 미리 확인합니다.
			jarPath := filepath.Join(appPath, jarFileName)
			if err := validation.ValidateFileExists(jarPath, false); err != nil {
				return nil, apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("로또 당첨번호 예측 프로그램(%s)을 찾을 수 없습니다", jarFileName))
			}

			// Java 실행 가능 여부 검증
			if _, err := execLookPath("java"); err != nil {
				return nil, apperrors.Wrap(err, apperrors.System, "호스트 시스템에서 Java 런타임(JRE) 환경을 감지할 수 없습니다. PATH 설정을 확인해 주십시오")
			}

			found = true

			break
		}
	}
	if !found {
		return nil, tasksvc.ErrTaskSettingsNotFound
	}

	lottoTask := &task{
		Task: tasksvc.NewBaseTask(req.TaskID, req.CommandID, instanceID, req.NotifierID, req.RunBy),

		appPath: appPath,

		executor: executor,
	}

	// CommandID에 따른 실행 함수를 미리 바인딩합니다.
	switch req.CommandID {
	case PredictionCommand:
		lottoTask.SetExecute(func(_ interface{}, _ bool) (string, interface{}, error) {
			return lottoTask.executePrediction()
		})
	default:
		return nil, tasksvc.NewErrCommandNotSupported(req.CommandID)
	}

	return lottoTask, nil
}

type task struct {
	tasksvc.Task

	appPath string

	executor commandExecutor
}
