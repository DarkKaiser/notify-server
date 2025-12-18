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

		appConfig: appConfig,
	}

	naverTask.SetFetcher(fetcher)

	// CommandID에 따른 실행 함수를 미리 바인딩합니다.
	switch req.CommandID {
	case WatchNewPerformancesCommand:
		naverTask.SetExecute(func(previousSnapshot interface{}, supportsHTML bool) (string, interface{}, error) {
			for _, t := range naverTask.appConfig.Tasks {
				if naverTask.GetID() == tasksvc.ID(t.ID) {
					for _, c := range t.Commands {
						if naverTask.GetCommandID() == tasksvc.CommandID(c.ID) {
							commandConfig := &watchNewPerformancesCommandConfig{}
							if err := tasksvc.DecodeMap(commandConfig, c.Data); err != nil {
								return "", nil, apperrors.Wrap(err, apperrors.InvalidInput, "작업 커맨드 데이터가 유효하지 않습니다")
							}
							if err := commandConfig.validate(); err != nil {
								return "", nil, apperrors.Wrap(err, apperrors.InvalidInput, "작업 커맨드 데이터가 유효하지 않습니다")
							}

							originTaskResultData, ok := previousSnapshot.(*watchNewPerformancesSnapshot)
							if ok == false {
								return "", nil, tasksvc.NewErrTypeAssertionFailed("TaskResultData", &watchNewPerformancesSnapshot{}, previousSnapshot)
							}

							return naverTask.executeWatchNewPerformances(commandConfig, originTaskResultData, supportsHTML)
						}
					}
					break
				}
			}
			return "", nil, apperrors.New(apperrors.Internal, "Command configuration not found")
		})
	default:
		return nil, tasksvc.NewErrCommandNotSupported(req.CommandID)
	}

	return naverTask, nil
}

type task struct {
	tasksvc.Task

	appConfig *config.AppConfig
}
