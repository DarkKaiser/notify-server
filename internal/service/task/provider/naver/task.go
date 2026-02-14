package naver

import (
	"context"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
)

const (
	// TaskID 네이버(https://www.naver.com/) 서비스와 연동되는 Task의 고유 식별자입니다.
	TaskID contract.TaskID = "NAVER"

	// WatchNewPerformancesCommand 네이버 신규 공연정보를 감시하는 Command의 고유 식별자입니다.
	// 이 Command는 네이버 공연 페이지를 주기적으로 스크래핑하여 새로운 공연 정보를 추적하고,
	// 신규 공연이 등록되면 텔레그램 등을 통해 알림을 전송합니다.
	WatchNewPerformancesCommand contract.TaskCommandID = "WatchNewPerformances"
)

func init() {
	provider.MustRegister(TaskID, &provider.TaskConfig{
		Commands: []*provider.TaskCommandConfig{
			{
				ID: WatchNewPerformancesCommand,

				AllowMultiple: true,

				NewSnapshot: func() any { return &watchNewPerformancesSnapshot{} },
			},
		},
		NewTask: newTask,
	})
}

func newTask(params provider.NewTaskParams) (provider.Task, error) {
	if params.Request.TaskID != TaskID {
		return nil, provider.NewErrTaskNotSupported(params.Request.TaskID)
	}

	naverTask := &task{
		Base: provider.NewBase(params, true),
	}

	// Command에 따른 실행 함수를 미리 바인딩합니다.
	switch params.Request.CommandID {
	case WatchNewPerformancesCommand:
		commandSettings, err := provider.FindCommandSettings[watchNewPerformancesSettings](params.AppConfig, params.Request.TaskID, params.Request.CommandID)
		if err != nil {
			return nil, err
		}

		naverTask.SetExecute(func(ctx context.Context, previousSnapshot any, supportsHTML bool) (string, any, error) {
			prevSnapshot, ok := previousSnapshot.(*watchNewPerformancesSnapshot)
			if !ok {
				return "", nil, provider.NewErrTypeAssertionFailed(&watchNewPerformancesSnapshot{}, previousSnapshot)
			}

			return naverTask.executeWatchNewPerformances(ctx, commandSettings, prevSnapshot, supportsHTML)
		})

	default:
		return nil, provider.NewErrCommandNotSupported(params.Request.CommandID, []contract.TaskCommandID{WatchNewPerformancesCommand})
	}

	return naverTask, nil
}

type task struct {
	*provider.Base
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ provider.Task = (*task)(nil)
