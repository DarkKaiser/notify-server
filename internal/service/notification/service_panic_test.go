package notification

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/darkkaiser/notify-server/internal/service/task"
	taskmocks "github.com/darkkaiser/notify-server/internal/service/task/mocks"
	"github.com/stretchr/testify/assert"
)

// PanicMockNotifierHandler Run 메서드에서 패닉을 발생시키는 Mock Notifier
type PanicMockNotifierHandler struct {
	mocks.MockNotifierHandler
	PanicOnRun bool
}

func (m *PanicMockNotifierHandler) Run(ctx context.Context) {
	if m.PanicOnRun {
		panic("Simulated Panic in Notifier Run")
	}
	m.MockNotifierHandler.Run(ctx)
}

func TestService_Start_PanicRecovery(t *testing.T) {
	// Setup
	cfg := &config.AppConfig{
		Notifier: config.NotifierConfig{
			DefaultNotifierID: "normal_notifier",
		},
	}
	executor := &taskmocks.MockExecutor{}

	// 패닉을 발생시키는 Notifier와 정상적인 Notifier 준비
	panicNotifier := &PanicMockNotifierHandler{
		MockNotifierHandler: mocks.MockNotifierHandler{
			IDValue: "panic_notifier",
		},
		PanicOnRun: true,
	}
	normalNotifier := &mocks.MockNotifierHandler{
		IDValue: "normal_notifier",
	}

	factory := &mocks.MockNotifierFactory{
		CreateNotifiersFunc: func(cfg *config.AppConfig, executor task.Executor) ([]notifier.NotifierHandler, error) {
			return []notifier.NotifierHandler{panicNotifier, normalNotifier}, nil
		},
	}

	service := NewService(cfg, executor, factory)

	// Test
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Service Termination WaitGroup (Service Start uses this to signal service stop)
	serviceStopWG := &sync.WaitGroup{}
	serviceStopWG.Add(1)

	// Start Service
	err := service.Start(ctx, serviceStopWG)
	assert.NoError(t, err)

	// service.Start launches goroutines for notifiers.
	// One of them will panic immediately.
	// We wait a bit to ensure panic happens and is recovered.
	time.Sleep(100 * time.Millisecond)

	// Verify Service is still running
	assert.NoError(t, service.Health())

	// Verify normal notifier is reportedly running (Mock doesn't really track running state unless we instrument it,
	// but the fact that the test process didn't crash is the main verification).

	// Terminate Service
	cancel()
	serviceStopWG.Wait()
}
