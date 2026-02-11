// Package kurly 마켓컬리(Kurly) 플랫폼과 연동하여 상품 정보를 수집하고
// 가격 변동을 모니터링하는 작업을 수행하는 패키지입니다.
package kurly

import (
	"context"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/darkkaiser/notify-server/internal/service/task/scraper"
)

const (
	// TaskID
	TaskID contract.TaskID = "KURLY" // 마켓컬리 (https://www.kurly.com/)

	// CommandID
	WatchProductPriceCommand contract.TaskCommandID = "WatchProductPrice" // 상품 가격 변화 감시
)

func init() {
	provider.Register(TaskID, &provider.Config{
		Commands: []*provider.CommandConfig{
			{
				ID: WatchProductPriceCommand,

				AllowMultiple: true,

				NewSnapshot: func() interface{} { return &watchProductPriceSnapshot{} },
			},
		},
		NewTask: newTask,
	})
}

func newTask(instanceID contract.TaskInstanceID, req *contract.TaskSubmitRequest, appConfig *config.AppConfig, storage contract.TaskResultStore, f fetcher.Fetcher, newSnapshot provider.NewSnapshotFunc) (provider.Task, error) {
	if req.TaskID != TaskID {
		return nil, provider.ErrTaskNotSupported
	}

	kurlyTask := &task{
		Base: provider.NewBase(req.TaskID, req.CommandID, instanceID, req.NotifierID, req.RunBy, storage, scraper.New(f), newSnapshot),

		appConfig: appConfig,
	}

	// CommandID에 따른 실행 함수를 미리 바인딩합니다
	switch req.CommandID {
	case WatchProductPriceCommand:
		commandSettings, err := provider.FindCommandSettings[watchProductPriceSettings](appConfig, req.TaskID, req.CommandID)
		if err != nil {
			return nil, err
		}

		kurlyTask.SetExecute(func(ctx context.Context, previousSnapshot interface{}, supportsHTML bool) (string, interface{}, error) {
			prevSnapshot, ok := previousSnapshot.(*watchProductPriceSnapshot)
			if !ok {
				return "", nil, provider.NewErrTypeAssertionFailed("prevSnapshot", &watchProductPriceSnapshot{}, previousSnapshot)
			}

			// 설정된 CSV 파일에서 감시 대상 상품 목록을 읽어오는 Loader를 생성합니다.
			loader := &CSVWatchListLoader{
				FilePath: commandSettings.WatchProductsFile,
			}

			return kurlyTask.executeWatchProductPrice(ctx, loader, prevSnapshot, supportsHTML)
		})
	default:
		return nil, provider.NewErrCommandNotSupported(req.CommandID)
	}

	return kurlyTask, nil
}

type task struct {
	*provider.Base

	appConfig *config.AppConfig
}
