// Package lotto 로또 당첨 번호 예측 프로그램과 연동하여
// 예측된 당첨 예상 번호를 알림으로 발송하는 작업을 수행하는 패키지입니다.
package lotto

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/darkkaiser/notify-server/internal/config"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/darkkaiser/notify-server/pkg/validation"
)

const (
	// TaskID
	TaskID contract.TaskID = "LOTTO"

	// CommandID
	PredictionCommand contract.TaskCommandID = "Prediction" // 로또 번호 예측

	// jarFileName PredictionCommand 수행 시 실행되는 JAR 파일명
	jarFileName = "lottoprediction-1.0.0.jar"
)

var execLookPath = exec.LookPath

type taskSettings struct {
	AppPath string `json:"app_path"`
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ provider.Validator = (*taskSettings)(nil)

func (s *taskSettings) Validate() error {
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

	if err := validation.ValidateDir(s.AppPath); err != nil {
		return apperrors.Wrap(err, apperrors.InvalidInput, "'app_path'로 지정된 경로가 존재하지 않거나 유효하지 않습니다")
	}

	return nil
}

func init() {
	provider.Register(TaskID, &provider.Config{
		Commands: []*provider.CommandConfig{
			{
				ID: PredictionCommand,

				AllowMultiple: false,

				NewSnapshot: func() interface{} { return &predictionSnapshot{} },
			},
		},
		NewTask: newTask,
	})
}

func newTask(instanceID contract.TaskInstanceID, req *contract.TaskSubmitRequest, appConfig *config.AppConfig, storage contract.TaskResultStore, _ fetcher.Fetcher, newSnapshot provider.NewSnapshotFunc) (provider.Task, error) {
	if req.TaskID != TaskID {
		return nil, provider.ErrTaskNotSupported
	}

	settings, err := provider.FindTaskSettings[taskSettings](appConfig, req.TaskID)
	if err != nil {
		return nil, err
	}

	// JAR 파일 존재 여부 검증
	// 실제 실행 시점의 에러를 방지하기 위해 미리 확인합니다.
	jarPath := filepath.Join(settings.AppPath, jarFileName)
	if err := validation.ValidateFile(jarPath); err != nil {
		return nil, apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("로또 당첨번호 예측 프로그램(%s)을 찾을 수 없습니다", jarFileName))
	}

	// Java 실행 가능 여부 검증
	if _, err := execLookPath("java"); err != nil {
		return nil, apperrors.Wrap(err, apperrors.System, "호스트 시스템에서 Java 런타임(JRE) 환경을 감지할 수 없습니다. PATH 설정을 확인해 주십시오")
	}

	lottoTask := &task{
		Base: provider.NewBase(provider.BaseParams{
			ID:          req.TaskID,
			CommandID:   req.CommandID,
			InstanceID:  instanceID,
			NotifierID:  req.NotifierID,
			RunBy:       req.RunBy,
			Storage:     storage,
			Scraper:     nil,
			NewSnapshot: newSnapshot,
		}),

		appPath: settings.AppPath,

		executor: &defaultCommandExecutor{},
	}

	// CommandID에 따른 실행 함수를 미리 바인딩합니다.
	switch req.CommandID {
	case PredictionCommand:
		lottoTask.SetExecute(func(ctx context.Context, _ interface{}, _ bool) (string, interface{}, error) {
			return lottoTask.executePrediction()
		})
	default:
		return nil, provider.NewErrCommandNotSupported(req.CommandID)
	}

	return lottoTask, nil
}

type task struct {
	*provider.Base

	appPath string

	executor commandExecutor
}
