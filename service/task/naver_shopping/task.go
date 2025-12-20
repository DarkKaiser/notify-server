package naver_shopping

import (
	"strings"

	"github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
)

const (
	// TaskID
	ID tasksvc.ID = "NS" // 네이버쇼핑(https://shopping.naver.com/)

	// CommandID
	WatchPriceAnyCommand = tasksvc.CommandID(watchPriceCommandIDPrefix + "*") // 네이버쇼핑 가격 확인
)

type taskSettings struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

func (c *taskSettings) validate() error {
	if c.ClientID == "" {
		return apperrors.New(apperrors.InvalidInput, "client_id가 입력되지 않았습니다")
	}
	if c.ClientSecret == "" {
		return apperrors.New(apperrors.InvalidInput, "client_secret이 입력되지 않았습니다")
	}
	return nil
}

func init() {
	tasksvc.Register(ID, &tasksvc.Config{
		Commands: []*tasksvc.CommandConfig{{
			ID: WatchPriceAnyCommand,

			AllowMultiple: true,

			NewSnapshot: func() interface{} { return &watchPriceSnapshot{} },
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

	settings := &taskSettings{}
	for _, t := range appConfig.Tasks {
		if req.TaskID == tasksvc.ID(t.ID) {
			if err := tasksvc.DecodeMap(settings, t.Data); err != nil {
				return nil, apperrors.Wrap(err, apperrors.InvalidInput, "작업 데이터가 유효하지 않습니다")
			}
			break
		}
	}
	if err := settings.validate(); err != nil {
		return nil, apperrors.Wrap(err, apperrors.InvalidInput, "작업 데이터가 유효하지 않습니다")
	}

	naverShoppingTask := &task{
		Task: tasksvc.NewBaseTask(req.TaskID, req.CommandID, instanceID, req.NotifierID, req.RunBy),

		appConfig: appConfig,

		clientID:     settings.ClientID,
		clientSecret: settings.ClientSecret,
	}

	naverShoppingTask.SetFetcher(fetcher)

	// CommandID에 따른 실행 함수를 미리 바인딩합니다 (Fail Fast)
	if strings.HasPrefix(string(req.CommandID), watchPriceCommandIDPrefix) {
		commandSettings, err := findCommandSettings(appConfig, req.TaskID, req.CommandID)
		if err != nil {
			return nil, err
		}

		naverShoppingTask.SetExecute(func(previousSnapshot interface{}, supportsHTML bool) (string, interface{}, error) {
			originTaskResultData, ok := previousSnapshot.(*watchPriceSnapshot)
			if !ok {
				return "", nil, tasksvc.NewErrTypeAssertionFailed("previousSnapshot", &watchPriceSnapshot{}, previousSnapshot)
			}

			return naverShoppingTask.executeWatchPrice(commandSettings, originTaskResultData, supportsHTML)
		})
	} else {
		return nil, apperrors.New(apperrors.InvalidInput, "지원하지 않는 명령입니다: "+string(req.CommandID))
	}

	return naverShoppingTask, nil
}

func findCommandSettings(appConfig *config.AppConfig, taskID tasksvc.ID, commandID tasksvc.CommandID) (*watchPriceSettings, error) {
	var commandSettings *watchPriceSettings

	for _, t := range appConfig.Tasks {
		if taskID == tasksvc.ID(t.ID) {
			for _, c := range t.Commands {
				if commandID == tasksvc.CommandID(c.ID) {
					settings := &watchPriceSettings{}
					if err := tasksvc.DecodeMap(settings, c.Data); err != nil {
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

	clientID     string
	clientSecret string
}
