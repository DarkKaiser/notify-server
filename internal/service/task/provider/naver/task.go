// Package naver 네이버 검색 API를 통해 공연 정보 등의 새로운 컨텐츠를
// 모니터링하고 알림을 발송하는 작업을 수행하는 패키지입니다.
package naver

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
	TaskID contract.TaskID = "NAVER"

	// CommandID
	WatchNewPerformancesCommand contract.TaskCommandID = "WatchNewPerformances" // 네이버 신규 공연정보 확인
)

func init() {
	provider.Register(TaskID, &provider.Config{
		Commands: []*provider.CommandConfig{
			{
				ID: WatchNewPerformancesCommand,

				AllowMultiple: true,

				NewSnapshot: func() interface{} { return &watchNewPerformancesSnapshot{} },
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

	naverTask := &task{
		Base: provider.NewBaseTask(req.TaskID, req.CommandID, instanceID, req.NotifierID, req.RunBy),
	}

	naverTask.SetFetcher(notificationFetcher)

	// CommandID에 따른 실행 함수를 미리 바인딩합니다.
	switch req.CommandID {
	case WatchNewPerformancesCommand:
		commandSettings, err := findCommandSettings(appConfig, req.TaskID, req.CommandID)
		if err != nil {
			return nil, err
		}

		naverTask.SetExecute(func(previousSnapshot interface{}, supportsHTML bool) (string, interface{}, error) {
			prevSnapshot, ok := previousSnapshot.(*watchNewPerformancesSnapshot)
			if !ok {
				return "", nil, provider.NewErrTypeAssertionFailed("prevSnapshot", &watchNewPerformancesSnapshot{}, previousSnapshot)
			}

			return naverTask.executeWatchNewPerformances(commandSettings, prevSnapshot, supportsHTML)
		})
	default:
		return nil, provider.NewErrCommandNotSupported(req.CommandID)
	}

	return naverTask, nil
}

func findCommandSettings(appConfig *config.AppConfig, taskID contract.TaskID, commandID contract.TaskCommandID) (*watchNewPerformancesSettings, error) {
	var commandSettings *watchNewPerformancesSettings

	for _, t := range appConfig.Tasks {
		if taskID == contract.TaskID(t.ID) {
			for _, c := range t.Commands {
				if commandID == contract.TaskCommandID(c.ID) {
					settings, err := maputil.Decode[watchNewPerformancesSettings](c.Data)
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
}
