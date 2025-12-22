package kurly

import (
	"github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/maputil"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
)

const (
	// TaskID
	ID tasksvc.ID = "KURLY" // 마켓컬리 (https://www.kurly.com/)

	// CommandID
	WatchProductPriceCommand tasksvc.CommandID = "WatchProductPrice" // 상품 가격 변화 감시
)

func init() {
	tasksvc.Register(ID, &tasksvc.Config{
		Commands: []*tasksvc.CommandConfig{{
			ID: WatchProductPriceCommand,

			AllowMultiple: true,

			NewSnapshot: func() interface{} { return &watchProductPriceSnapshot{} },
		}},

		NewTask: newTask,
	})
}

func newTask(instanceID tasksvc.InstanceID, req *tasksvc.SubmitRequest, appConfig *config.AppConfig) (tasksvc.Handler, error) {
	fetcher := tasksvc.NewRetryFetcherFromConfig(appConfig.HTTPRetry.MaxRetries, appConfig.HTTPRetry.RetryDelay)
	return createTask(instanceID, req, appConfig, fetcher)
}

func createTask(instanceID tasksvc.InstanceID, req *tasksvc.SubmitRequest, appConfig *config.AppConfig, fetcher tasksvc.Fetcher) (tasksvc.Handler, error) {
	if req.TaskID != ID {
		return nil, tasksvc.ErrTaskNotSupported
	}

	kurlyTask := &task{
		Task: tasksvc.NewBaseTask(req.TaskID, req.CommandID, instanceID, req.NotifierID, req.RunBy),

		appConfig: appConfig,
	}

	kurlyTask.SetFetcher(fetcher)

	// CommandID에 따른 실행 함수를 미리 바인딩합니다 (Fail Fast)
	switch req.CommandID {
	case WatchProductPriceCommand:
		commandSettings, err := findCommandSettings(appConfig, req.TaskID, req.CommandID)
		if err != nil {
			return nil, err
		}

		kurlyTask.SetExecute(func(previousSnapshot interface{}, supportsHTML bool) (string, interface{}, error) {
			prevSnapshot, ok := previousSnapshot.(*watchProductPriceSnapshot)
			if !ok {
				return "", nil, tasksvc.NewErrTypeAssertionFailed("prevSnapshot", &watchProductPriceSnapshot{}, previousSnapshot)
			}

			return kurlyTask.executeWatchProductPrice(commandSettings, prevSnapshot, supportsHTML)
		})
	default:
		return nil, tasksvc.NewErrCommandNotSupported(req.CommandID)
	}

	return kurlyTask, nil
}

func findCommandSettings(appConfig *config.AppConfig, taskID tasksvc.ID, commandID tasksvc.CommandID) (*watchProductPriceSettings, error) {
	var commandSettings *watchProductPriceSettings

	for _, t := range appConfig.Tasks {
		if taskID == tasksvc.ID(t.ID) {
			for _, c := range t.Commands {
				if commandID == tasksvc.CommandID(c.ID) {
					settings := &watchProductPriceSettings{}
					if err := maputil.Decode(c.Data, settings); err != nil {
						return nil, apperrors.Wrap(err, apperrors.InvalidInput, tasksvc.ErrInvalidCommandSettings.Error())
					}
					if err := settings.validate(); err != nil {
						return nil, apperrors.Wrap(err, apperrors.InvalidInput, tasksvc.ErrInvalidCommandSettings.Error())
					}
					commandSettings = settings
					break
				}
			}
			break
		}
	}

	if commandSettings == nil {
		return nil, tasksvc.ErrCommandSettingsNotFound
	}

	return commandSettings, nil
}

type task struct {
	tasksvc.Task

	appConfig *config.AppConfig
}
