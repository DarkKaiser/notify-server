// Package naver 네이버 검색 API를 통해 공연 정보 등의 새로운 컨텐츠를
// 모니터링하고 알림을 발송하는 작업을 수행하는 패키지입니다.
package naver

import (
	"github.com/darkkaiser/notify-server/internal/config"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	tasksvc "github.com/darkkaiser/notify-server/internal/service/task"
	"github.com/darkkaiser/notify-server/pkg/maputil"
)

const (
	// TaskID
	ID tasksvc.ID = "NAVER"

	// CommandID
	WatchNewPerformancesCommand tasksvc.CommandID = "WatchNewPerformances" // 네이버 신규 공연정보 확인
)

func init() {
	tasksvc.Register(ID, &tasksvc.Config{
		Commands: []*tasksvc.CommandConfig{{
			ID: WatchNewPerformancesCommand,

			AllowMultiple: true,

			NewSnapshot: func() interface{} { return &watchNewPerformancesSnapshot{} },
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

	naverTask := &task{
		Task: tasksvc.NewBaseTask(req.TaskID, req.CommandID, instanceID, req.NotifierID, req.RunBy),
	}

	naverTask.SetFetcher(fetcher)

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
				return "", nil, tasksvc.NewErrTypeAssertionFailed("prevSnapshot", &watchNewPerformancesSnapshot{}, previousSnapshot)
			}

			return naverTask.executeWatchNewPerformances(commandSettings, prevSnapshot, supportsHTML)
		})
	default:
		return nil, tasksvc.NewErrCommandNotSupported(req.CommandID)
	}

	return naverTask, nil
}

func findCommandSettings(appConfig *config.AppConfig, taskID tasksvc.ID, commandID tasksvc.CommandID) (*watchNewPerformancesSettings, error) {
	var commandSettings *watchNewPerformancesSettings

	for _, t := range appConfig.Tasks {
		if taskID == tasksvc.ID(t.ID) {
			for _, c := range t.Commands {
				if commandID == tasksvc.CommandID(c.ID) {
					settings, err := maputil.Decode[watchNewPerformancesSettings](c.Data)
					if err != nil {
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
}
