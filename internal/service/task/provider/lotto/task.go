package lotto

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/darkkaiser/notify-server/pkg/validation"
)

// component Task 서비스의 Lotto Provider 로깅용 컴포넌트 이름
const component = "task.provider.lotto"

const (
	// TaskID 로또 당첨번호 예측 서비스와 연동되는 Task의 고유 식별자입니다.
	TaskID contract.TaskID = "LOTTO"

	// PredictionCommand 로또 당첨번호 예측을 수행하는 Command의 고유 식별자입니다.
	// 이 Command는 외부 Java 프로그램(JAR)을 실행하여 로또 당첨번호를 예측하고,
	// 예측 결과를 텔레그램 등을 통해 전송합니다.
	PredictionCommand contract.TaskCommandID = "Prediction"
)

const (
	// predictionJarName 로또 당첨번호 예측을 수행하는 외부 Java 프로그램(JAR)의 파일명입니다.
	//
	// 이 상수는 PredictionCommand 실행 시 사용되며, Task 초기화 단계(newTask)에서 해당 파일의
	// 존재 여부를 검증합니다. JAR 파일은 반드시 Task 설정의 app_path로 지정된 디렉터리 내에
	// 위치해야 하며, 그렇지 않을 경우 Task 생성이 실패합니다.
	predictionJarName = "lottoprediction-1.0.0.jar"
)

// lookPath os/exec 패키지의 LookPath 함수를 참조합니다.
// 이 변수는 단위 테스트 시 외부 실행 파일(예: java)의 존재 여부를 확인하는 로직을 가상화(Mocking)하기 위해 사용됩니다.
var lookPath = exec.LookPath

type taskSettings struct {
	AppPath string `json:"app_path"`
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ provider.Validator = (*taskSettings)(nil)

func (s *taskSettings) Validate() error {
	s.AppPath = strings.TrimSpace(s.AppPath)
	if s.AppPath == "" {
		return ErrAppPathMissing
	}

	// 절대 경로로 변환하여 실행 위치(CWD)에 독립적으로 만듭니다.
	absPath, err := filepath.Abs(s.AppPath)
	if err != nil {
		return newErrAppPathAbsFailed(err)
	}
	s.AppPath = absPath

	if err := validation.ValidateDir(s.AppPath); err != nil {
		return newErrAppPathDirValidationFailed(err)
	}

	return nil
}

func init() {
	provider.MustRegister(TaskID, &provider.TaskConfig{
		Commands: []*provider.TaskCommandConfig{
			{
				ID: PredictionCommand,

				AllowMultiple: false,

				NewSnapshot: func() any { return &predictionSnapshot{} },
			},
		},
		NewTask: newTask,
	})
}

func newTask(params provider.NewTaskParams) (provider.Task, error) {
	if params.Request.TaskID != TaskID {
		return nil, provider.NewErrTaskNotSupported(params.Request.TaskID)
	}

	taskSettings, err := provider.FindTaskSettings[taskSettings](params.AppConfig, params.Request.TaskID)
	if err != nil {
		return nil, err
	}

	// 로또 당첨번호 예측을 담당하는 외부 JAR 파일이 지정된 경로에 존재하는지 확인합니다.
	predictionJarPath := filepath.Join(taskSettings.AppPath, predictionJarName)
	if err := validation.ValidateFile(predictionJarPath); err != nil {
		return nil, newErrJarFileNotFound(err, predictionJarName)
	}

	// JAR 파일을 실행하기 위해 시스템에 Java가 설치되어 있는지 확인합니다.
	if _, err := lookPath("java"); err != nil {
		return nil, newErrJavaNotFound(err)
	}

	lottoTask := &task{
		Base: provider.NewBase(params, false),

		appPath: taskSettings.AppPath,

		executor: &defaultCommandExecutor{
			dir: taskSettings.AppPath,
		},
	}

	// Command에 따른 실행 함수를 미리 바인딩합니다.
	switch params.Request.CommandID {
	case PredictionCommand:
		lottoTask.SetExecute(func(ctx context.Context, _ any, _ bool) (string, any, error) {
			return lottoTask.executePrediction(ctx)
		})

	default:
		return nil, provider.NewErrCommandNotSupported(params.Request.CommandID, []contract.TaskCommandID{PredictionCommand})
	}

	return lottoTask, nil
}

type task struct {
	*provider.Base

	appPath string

	executor commandExecutor
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ provider.Task = (*task)(nil)
