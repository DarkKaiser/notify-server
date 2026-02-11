// Package navershopping 네이버 쇼핑(Naver Shopping) 플랫폼과 연동하여
// 상품 정보를 수집하고 가격 변동을 모니터링하는 작업을 수행하는 패키지입니다.
package navershopping

import (
	"context"
	"strings"

	"github.com/darkkaiser/notify-server/internal/config"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/darkkaiser/notify-server/internal/service/task/scraper"
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

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ provider.Validator = (*taskSettings)(nil)

func (s *taskSettings) Validate() error {
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

func newTask(instanceID contract.TaskInstanceID, req *contract.TaskSubmitRequest, appConfig *config.AppConfig, storage contract.TaskResultStore, f fetcher.Fetcher, newSnapshot provider.NewSnapshotFunc) (provider.Task, error) {
	if req.TaskID != TaskID {
		return nil, provider.ErrTaskNotSupported
	}

	settings, err := provider.FindTaskSettings[taskSettings](appConfig, req.TaskID)
	if err != nil {
		return nil, err
	}

	naverShoppingTask := &task{
		Base: provider.NewBase(provider.BaseParams{
			ID:          req.TaskID,
			CommandID:   req.CommandID,
			InstanceID:  instanceID,
			NotifierID:  req.NotifierID,
			RunBy:       req.RunBy,
			Storage:     storage,
			Scraper:     scraper.New(f),
			NewSnapshot: newSnapshot,
		}),

		clientID:     settings.ClientID,
		clientSecret: settings.ClientSecret,

		appConfig: appConfig,
	}

	// CommandID에 따른 실행 함수를 미리 바인딩합니다.
	if strings.HasPrefix(string(req.CommandID), watchPriceAnyCommandPrefix) {
		commandSettings, err := provider.FindCommandSettings[watchPriceSettings](appConfig, req.TaskID, req.CommandID)
		if err != nil {
			return nil, err
		}

		naverShoppingTask.SetExecute(func(ctx context.Context, previousSnapshot any, supportsHTML bool) (string, any, error) {
			prevSnapshot, ok := previousSnapshot.(*watchPriceSnapshot)
			if !ok {
				return "", nil, provider.NewErrTypeAssertionFailed("prevSnapshot", &watchPriceSnapshot{}, previousSnapshot)
			}

			return naverShoppingTask.executeWatchPrice(ctx, commandSettings, prevSnapshot, supportsHTML)
		})
	} else {
		return nil, provider.NewErrCommandNotSupported(req.CommandID)
	}

	return naverShoppingTask, nil
}

type task struct {
	*provider.Base

	clientID     string
	clientSecret string

	appConfig *config.AppConfig
}
