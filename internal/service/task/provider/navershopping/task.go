// Package navershopping 네이버 쇼핑(Naver Shopping) 플랫폼과 연동하여
// 상품 정보를 수집하고 가격 변동을 모니터링하는 작업을 수행하는 패키지입니다.
package navershopping

import (
	"strings"

	"github.com/darkkaiser/notify-server/internal/config"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/darkkaiser/notify-server/pkg/maputil"
)

const (
	// TaskID
	TaskID contract.TaskID = "NS" // 네이버쇼핑 (https://shopping.naver.com/)

	// CommandID
	WatchPriceAnyCommand = contract.TaskCommandID(watchPriceAnyCommandPrefix + "*") // 네이버쇼핑 가격 확인
)

type taskSettings struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

func (s *taskSettings) validate() error {
	s.ClientID = strings.TrimSpace(s.ClientID)
	if s.ClientID == "" {
		return apperrors.New(apperrors.InvalidInput, "client_id가 입력되지 않았거나 공백입니다")
	}
	s.ClientSecret = strings.TrimSpace(s.ClientSecret)
	if s.ClientSecret == "" {
		return apperrors.New(apperrors.InvalidInput, "client_secret이 입력되지 않았거나 공백입니다")
	}
	return nil
}

func init() {
	provider.Register(TaskID, &provider.Config{
		Commands: []*provider.CommandConfig{
			{
				ID: WatchPriceAnyCommand,

				AllowMultiple: true,

				NewSnapshot: func() interface{} { return &watchPriceSnapshot{} },
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

	var settings *taskSettings
	for _, t := range appConfig.Tasks {
		if req.TaskID == contract.TaskID(t.ID) {
			s, err := maputil.Decode[taskSettings](t.Data)
			if err != nil {
				return nil, apperrors.Wrap(err, apperrors.InvalidInput, provider.ErrInvalidTaskSettings.Error())
			}
			if err := s.validate(); err != nil {
				return nil, apperrors.Wrap(err, apperrors.InvalidInput, provider.ErrInvalidTaskSettings.Error())
			}

			settings = s

			break
		}
	}
	if settings == nil {
		return nil, provider.ErrTaskSettingsNotFound
	}

	naverShoppingTask := &task{
		Base: provider.NewBaseTask(req.TaskID, req.CommandID, instanceID, req.NotifierID, req.RunBy),

		clientID:     settings.ClientID,
		clientSecret: settings.ClientSecret,

		appConfig: appConfig,
	}

	naverShoppingTask.SetFetcher(notificationFetcher)

	// CommandID에 따른 실행 함수를 미리 바인딩합니다.
	if strings.HasPrefix(string(req.CommandID), watchPriceAnyCommandPrefix) {
		commandSettings, err := findCommandSettings(appConfig, req.TaskID, req.CommandID)
		if err != nil {
			return nil, err
		}

		naverShoppingTask.SetExecute(func(previousSnapshot interface{}, supportsHTML bool) (string, interface{}, error) {
			prevSnapshot, ok := previousSnapshot.(*watchPriceSnapshot)
			if !ok {
				return "", nil, provider.NewErrTypeAssertionFailed("prevSnapshot", &watchPriceSnapshot{}, previousSnapshot)
			}

			return naverShoppingTask.executeWatchPrice(commandSettings, prevSnapshot, supportsHTML)
		})
	} else {
		return nil, provider.NewErrCommandNotSupported(req.CommandID)
	}

	return naverShoppingTask, nil
}

func findCommandSettings(appConfig *config.AppConfig, taskID contract.TaskID, commandID contract.TaskCommandID) (*watchPriceSettings, error) {
	var commandSettings *watchPriceSettings

	for _, t := range appConfig.Tasks {
		if taskID == contract.TaskID(t.ID) {
			for _, c := range t.Commands {
				if commandID == contract.TaskCommandID(c.ID) {
					settings, err := maputil.Decode[watchPriceSettings](c.Data)
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

	clientID     string
	clientSecret string

	appConfig *config.AppConfig
}
