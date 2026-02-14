package navershopping

import (
	"context"
	"strings"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
)

const (
	// TaskID 네이버쇼핑(https://shopping.naver.com/) 서비스와 연동되는 Task의 고유 식별자입니다.
	TaskID contract.TaskID = "NS"

	// WatchPriceAnyCommand 네이버쇼핑 상품의 가격 변화를 감시하는 Command의 고유 식별자입니다.
	// 이 Command는 와일드카드 패턴(*)을 사용하여 여러 상품을 동시에 추적할 수 있으며,
	// 네이버 쇼핑 API를 통해 가격 변동을 확인하고 변화가 감지되면 알림을 전송합니다.
	WatchPriceAnyCommand = contract.TaskCommandID(watchPriceAnyCommandPrefix + "*")
)

type taskSettings struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ provider.Validator = (*taskSettings)(nil)

func (s *taskSettings) Validate() error {
	s.ClientID = strings.TrimSpace(s.ClientID)
	if s.ClientID == "" {
		return ErrClientIDMissing
	}

	s.ClientSecret = strings.TrimSpace(s.ClientSecret)
	if s.ClientSecret == "" {
		return ErrClientSecretMissing
	}

	return nil
}

func init() {
	provider.MustRegister(TaskID, &provider.TaskConfig{
		Commands: []*provider.TaskCommandConfig{
			{
				ID: WatchPriceAnyCommand,

				AllowMultiple: true,

				NewSnapshot: func() any { return &watchPriceSnapshot{} },
			},
		},
		NewTask: newTask,
	})
}

func newTask(params provider.NewTaskParams) (provider.Task, error) {
	if params.Request.TaskID != TaskID {
		return nil, provider.NewErrTaskNotSupported(params.Request.TaskID)
	}

	taskSettings, err := provider.FindTaskSettings[taskSettings](params.AppConfig, params.Request.TaskID)
	if err != nil {
		return nil, err
	}

	naverShoppingTask := &task{
		Base: provider.NewBase(params, true),

		appConfig: params.AppConfig,

		clientID:     taskSettings.ClientID,
		clientSecret: taskSettings.ClientSecret,
	}

	// Command에 따른 실행 함수를 미리 바인딩합니다.
	if strings.HasPrefix(string(params.Request.CommandID), watchPriceAnyCommandPrefix) {
		commandSettings, err := provider.FindCommandSettings[watchPriceSettings](params.AppConfig, params.Request.TaskID, params.Request.CommandID)
		if err != nil {
			return nil, err
		}

		naverShoppingTask.SetExecute(func(ctx context.Context, previousSnapshot any, supportsHTML bool) (string, any, error) {
			prevSnapshot, ok := previousSnapshot.(*watchPriceSnapshot)
			if !ok {
				return "", nil, provider.NewErrTypeAssertionFailed(&watchPriceSnapshot{}, previousSnapshot)
			}

			return naverShoppingTask.executeWatchPrice(ctx, commandSettings, prevSnapshot, supportsHTML)
		})
	} else {
		return nil, provider.NewErrCommandNotSupported(params.Request.CommandID, []contract.TaskCommandID{WatchPriceAnyCommand})
	}

	return naverShoppingTask, nil
}

type task struct {
	*provider.Base

	appConfig *config.AppConfig

	clientID     string
	clientSecret string
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ provider.Task = (*task)(nil)
