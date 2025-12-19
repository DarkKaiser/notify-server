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

	nsTask := &task{
		Task: tasksvc.NewBaseTask(req.TaskID, req.CommandID, instanceID, req.NotifierID, req.RunBy),

		appConfig: appConfig,

		clientID:     settings.ClientID,
		clientSecret: settings.ClientSecret,
	}

	nsTask.SetFetcher(fetcher)

	// CommandID에 따른 실행 함수를 미리 바인딩합니다 (Fail Fast)
	if strings.HasPrefix(string(req.CommandID), watchPriceCommandIDPrefix) {
		nsTask.SetExecute(func(previousSnapshot interface{}, supportsHTML bool) (string, interface{}, error) {
			for _, t := range nsTask.appConfig.Tasks {
				if nsTask.GetID() == tasksvc.ID(t.ID) {
					for _, c := range t.Commands {
						if nsTask.GetCommandID() == tasksvc.CommandID(c.ID) {
							commandConfig := &watchPriceCommandConfig{}
							if err := tasksvc.DecodeMap(commandConfig, c.Data); err != nil {
								return "", nil, apperrors.Wrap(err, apperrors.InvalidInput, "작업 커맨드 데이터가 유효하지 않습니다")
							}
							if err := commandConfig.validate(); err != nil {
								return "", nil, apperrors.Wrap(err, apperrors.InvalidInput, "작업 커맨드 데이터가 유효하지 않습니다")
							}

							originTaskResultData, ok := previousSnapshot.(*watchPriceSnapshot)
							if ok == false {
								return "", nil, tasksvc.NewErrTypeAssertionFailed("TaskResultData", &watchPriceSnapshot{}, previousSnapshot)
							}

							return nsTask.executeWatchPrice(commandConfig, originTaskResultData, supportsHTML)
						}
					}
					break
				}
			}
			return "", nil, apperrors.New(apperrors.Internal, "Command configuration not found")
		})
	} else {
		return nil, apperrors.New(apperrors.InvalidInput, "지원하지 않는 명령입니다: "+string(req.CommandID))
	}

	return nsTask, nil
}

type task struct {
	tasksvc.Task

	appConfig *config.AppConfig

	clientID     string
	clientSecret string
}
