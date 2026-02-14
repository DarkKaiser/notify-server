package kurly

import (
	"context"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
)

const (
	// TaskID 마켓컬리(https://www.kurly.com/) 서비스와 연동되는 Task의 고유 식별자입니다.
	TaskID contract.TaskID = "KURLY"

	// WatchProductPriceCommand 마켓컬리 상품의 가격 변화를 감시하는 Command의 고유 식별자입니다.
	// 이 Command는 지정된 상품 목록을 주기적으로 스크래핑하여 가격 변동을 추적하고,
	// 변화가 감지되면 텔레그램 등을 통해 알림을 전송합니다.
	WatchProductPriceCommand contract.TaskCommandID = "WatchProductPrice"
)

func init() {
	provider.MustRegister(TaskID, &provider.TaskConfig{
		Commands: []*provider.TaskCommandConfig{
			{
				ID: WatchProductPriceCommand,

				AllowMultiple: true,

				NewSnapshot: func() any { return &watchProductPriceSnapshot{} },
			},
		},
		NewTask: newTask,
	})
}

func newTask(params provider.NewTaskParams) (provider.Task, error) {
	if params.Request.TaskID != TaskID {
		return nil, provider.NewErrTaskNotSupported(params.Request.TaskID)
	}

	kurlyTask := &task{
		Base: provider.NewBase(params, true),

		appConfig: params.AppConfig,
	}

	// Command에 따른 실행 함수를 미리 바인딩합니다
	switch params.Request.CommandID {
	case WatchProductPriceCommand:
		commandSettings, err := provider.FindCommandSettings[watchProductPriceSettings](params.AppConfig, params.Request.TaskID, params.Request.CommandID)
		if err != nil {
			return nil, err
		}

		kurlyTask.SetExecute(func(ctx context.Context, previousSnapshot any, supportsHTML bool) (string, any, error) {
			prevSnapshot, ok := previousSnapshot.(*watchProductPriceSnapshot)
			if !ok {
				return "", nil, provider.NewErrTypeAssertionFailed(&watchProductPriceSnapshot{}, previousSnapshot)
			}

			// CSV 파일에서 감시 대상 상품 목록을 읽어오는 Loader를 생성합니다.
			loader := &CSVWatchListLoader{
				FilePath: commandSettings.WatchProductsFile,
			}

			return kurlyTask.executeWatchProductPrice(ctx, loader, prevSnapshot, supportsHTML)
		})

	default:
		return nil, provider.NewErrCommandNotSupported(params.Request.CommandID, []contract.TaskCommandID{WatchProductPriceCommand})
	}

	return kurlyTask, nil
}

type task struct {
	*provider.Base

	appConfig *config.AppConfig
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ provider.Task = (*task)(nil)
