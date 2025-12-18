package naver

import (
	"github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
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
		commandConfig, err := findCommandConfig(appConfig, req.TaskID, req.CommandID)
		if err != nil {
			return nil, err
		}

		naverTask.SetExecute(func(previousSnapshot interface{}, supportsHTML bool) (string, interface{}, error) {
			prevSnapshot, ok := previousSnapshot.(*watchNewPerformancesSnapshot)
			if ok == false {
				return "", nil, tasksvc.NewErrTypeAssertionFailed("prevSnapshot", &watchNewPerformancesSnapshot{}, previousSnapshot)
			}

			return naverTask.executeWatchNewPerformances(commandConfig, prevSnapshot, supportsHTML)
		})
	default:
		return nil, tasksvc.NewErrCommandNotSupported(req.CommandID)
	}

	return naverTask, nil
}

func findCommandConfig(appConfig *config.AppConfig, taskID tasksvc.ID, commandID tasksvc.CommandID) (*watchNewPerformancesCommandConfig, error) {
	var commandConfig *watchNewPerformancesCommandConfig

	for _, t := range appConfig.Tasks {
		if taskID == tasksvc.ID(t.ID) {
			for _, c := range t.Commands {
				if commandID == tasksvc.CommandID(c.ID) {
					cfg := &watchNewPerformancesCommandConfig{}
					if err := tasksvc.DecodeMap(cfg, c.Data); err != nil {
						return nil, apperrors.Wrap(err, apperrors.InvalidInput, tasksvc.ErrInvalidCommandData.Error())
					}
					if err := cfg.validate(); err != nil {
						return nil, apperrors.Wrap(err, apperrors.InvalidInput, tasksvc.ErrInvalidCommandData.Error())
					}
					commandConfig = cfg
					break
				}
			}
			break
		}
	}

	if commandConfig == nil {
		return nil, tasksvc.ErrCommandConfigNotFound
	}

	return commandConfig, nil
}

type task struct {
	tasksvc.Task
}
