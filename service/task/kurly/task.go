package kurly

import (
	"github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
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

	tTask := &task{
		Task:      tasksvc.NewBaseTask(req.TaskID, req.CommandID, instanceID, req.NotifierID, req.RunBy),
		appConfig: appConfig,
	}

	tTask.SetFetcher(fetcher)

	// CommandID에 따른 실행 함수를 미리 바인딩합니다 (Fail Fast)
	switch req.CommandID {
	case WatchProductPriceCommand:
		tTask.SetExecute(func(previousSnapshot interface{}, supportsHTML bool) (string, interface{}, error) {
			for _, t := range tTask.appConfig.Tasks {
				if tTask.GetID() == tasksvc.ID(t.ID) {
					for _, c := range t.Commands {
						if tTask.GetCommandID() == tasksvc.CommandID(c.ID) {
							settings := &watchProductPriceSettings{}
							if err := tasksvc.DecodeMap(settings, c.Data); err != nil {
								return "", nil, apperrors.Wrap(err, apperrors.InvalidInput, tasksvc.ErrInvalidCommandSettings.Error())
							}
							if err := settings.validate(); err != nil {
								return "", nil, apperrors.Wrap(err, apperrors.InvalidInput, tasksvc.ErrInvalidCommandSettings.Error())
							}

							originTaskResultData, ok := previousSnapshot.(*watchProductPriceSnapshot)
							if !ok {
								return "", nil, tasksvc.NewErrTypeAssertionFailed("TaskResultData", &watchProductPriceSnapshot{}, previousSnapshot)
							}

							return tTask.executeWatchProductPrice(settings, originTaskResultData, supportsHTML)
						}
					}
					break
				}
			}
			return "", nil, tasksvc.ErrCommandSettingsNotFound
		})
	default:
		return nil, tasksvc.NewErrCommandNotSupported(req.CommandID)
	}

	return tTask, nil
}

type task struct {
	tasksvc.Task

	appConfig *config.AppConfig
}
