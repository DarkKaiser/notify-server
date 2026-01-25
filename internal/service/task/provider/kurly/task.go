// Package kurly 마켓컬리(Kurly) 플랫폼과 연동하여 상품 정보를 수집하고
// 가격 변동을 모니터링하는 작업을 수행하는 패키지입니다.
package kurly

import (
	"github.com/darkkaiser/notify-server/internal/config"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/darkkaiser/notify-server/pkg/maputil"
)

const (
	// TaskID
	TaskID contract.TaskID = "KURLY" // 마켓컬리 (https://www.kurly.com/)

	// CommandID
	WatchProductPriceCommand contract.TaskCommandID = "WatchProductPrice" // 상품 가격 변화 감시
)

func init() {
	provider.Register(TaskID, &provider.Config{
		Commands: []*provider.CommandConfig{
			{
				ID: WatchProductPriceCommand,

				AllowMultiple: true,

				NewSnapshot: func() interface{} { return &watchProductPriceSnapshot{} },
			},
		},
		NewTask: newTask,
	})
}

func newTask(instanceID contract.TaskInstanceID, req *contract.TaskSubmitRequest, appConfig *config.AppConfig) (provider.Task, error) {
	httpFetcher := fetcher.NewRetryFetcherFromConfig(appConfig.HTTPRetry.MaxRetries, appConfig.HTTPRetry.RetryDelay)
	return createTask(instanceID, req, appConfig, httpFetcher)
}

func createTask(instanceID contract.TaskInstanceID, req *contract.TaskSubmitRequest, appConfig *config.AppConfig, notificationFetcher fetcher.Fetcher) (provider.Task, error) {
	if req.TaskID != TaskID {
		return nil, provider.ErrTaskNotSupported
	}

	kurlyTask := &task{
		Base: provider.NewBase(req.TaskID, req.CommandID, instanceID, req.NotifierID, req.RunBy),

		appConfig: appConfig,
	}

	kurlyTask.SetFetcher(notificationFetcher)

	// CommandID에 따른 실행 함수를 미리 바인딩합니다
	switch req.CommandID {
	case WatchProductPriceCommand:
		commandSettings, err := findCommandSettings(appConfig, req.TaskID, req.CommandID)
		if err != nil {
			return nil, err
		}

		kurlyTask.SetExecute(func(previousSnapshot interface{}, supportsHTML bool) (string, interface{}, error) {
			prevSnapshot, ok := previousSnapshot.(*watchProductPriceSnapshot)
			if !ok {
				return "", nil, provider.NewErrTypeAssertionFailed("prevSnapshot", &watchProductPriceSnapshot{}, previousSnapshot)
			}

			// 설정된 CSV 파일에서 감시 대상 상품 목록을 읽어오는 Loader를 생성합니다.
			loader := &CSVWatchListLoader{
				FilePath: commandSettings.WatchProductsFile,
			}

			return kurlyTask.executeWatchProductPrice(loader, prevSnapshot, supportsHTML)
		})
	default:
		return nil, provider.NewErrCommandNotSupported(req.CommandID)
	}

	return kurlyTask, nil
}

func findCommandSettings(appConfig *config.AppConfig, taskID contract.TaskID, commandID contract.TaskCommandID) (*watchProductPriceSettings, error) {
	var commandSettings *watchProductPriceSettings

	for _, t := range appConfig.Tasks {
		if taskID == contract.TaskID(t.ID) {
			for _, c := range t.Commands {
				if commandID == contract.TaskCommandID(c.ID) {
					settings, err := maputil.Decode[watchProductPriceSettings](c.Data)
					if err != nil {
						return nil, apperrors.Wrap(err, apperrors.InvalidInput, provider.ErrInvalidCommandSettings.Error())
					}
					if err := settings.validate(); err != nil {
						return nil, apperrors.Wrap(err, apperrors.InvalidInput, provider.ErrInvalidCommandSettings.Error())
					}
					commandSettings = settings
					break
				}
			}
			break
		}
	}

	if commandSettings == nil {
		return nil, provider.ErrCommandSettingsNotFound
	}

	return commandSettings, nil
}

type task struct {
	provider.Base

	appConfig *config.AppConfig
}
