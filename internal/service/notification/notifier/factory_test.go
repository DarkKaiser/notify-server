package notifier_test

import (
	"errors"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	notificationmocks "github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	taskmocks "github.com/darkkaiser/notify-server/internal/service/task/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Interface Verification
// =============================================================================

func TestFactory_InterfaceCompliance(t *testing.T) {
	t.Run("NewFactory returns Factory interface", func(t *testing.T) {
		f := notifier.NewFactory()
		require.NotNil(t, f)

		// Type verification (compile-time safety is already handled by return type,
		// but this runtime check acts as a sanity check)
		var _ notifier.Factory = f
	})

	t.Run("MockFactory implements Factory interface", func(t *testing.T) {
		var _ notifier.Factory = (*notificationmocks.MockFactory)(nil)
	})
}

// =============================================================================
// Functional Tests
// =============================================================================

func TestFactory_CreateNotifiers(t *testing.T) {
	// Common Test Helpers
	mockExecutor := &taskmocks.MockExecutor{}
	testConfig := &config.AppConfig{
		Notifier: config.NotifierConfig{
			DefaultNotifierID: "default",
		},
	}

	t.Run("Argument Propagation", func(t *testing.T) {
		// 테스트 목적: CreateNotifiers에 전달된 config와 executor가 프로세서에게 정확히 전달되는지 확인
		f := notifier.NewFactory()

		called := false
		f.RegisterProcessor(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
			assert.Equal(t, testConfig, cfg, "Config should differ")
			assert.Equal(t, mockExecutor, executor, "Executor should match")
			called = true
			return nil, nil // Return empty, no error
		})

		_, err := f.CreateNotifiers(testConfig, mockExecutor)
		require.NoError(t, err)
		assert.True(t, called, "Processor should have been called")
	})

	t.Run("Aggregation of Handlers", func(t *testing.T) {
		// 테스트 목적: 여러 프로세서에서 생성된 핸들러들이 하나의 슬라이스로 잘 합쳐지는지 확인
		f := notifier.NewFactory()

		handler1 := &notificationmocks.MockNotifierHandler{IDValue: "h1"}
		handler2 := &notificationmocks.MockNotifierHandler{IDValue: "h2"}

		// Processor 1: returns [h1]
		f.RegisterProcessor(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
			return []notifier.NotifierHandler{handler1}, nil
		})

		// Processor 2: returns [h2]
		f.RegisterProcessor(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
			return []notifier.NotifierHandler{handler2}, nil
		})

		// Processor 3: returns empty (should not affect result)
		f.RegisterProcessor(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
			return []notifier.NotifierHandler{}, nil
		})

		handlers, err := f.CreateNotifiers(testConfig, mockExecutor)
		require.NoError(t, err)
		require.Len(t, handlers, 2)
		assert.Equal(t, handler1, handlers[0])
		assert.Equal(t, handler2, handlers[1])
	})

	t.Run("Error Handling - Fail Fast", func(t *testing.T) {
		// 테스트 목적: 프로세서 중 하나가 에러를 반환하면 즉시 중단하고 에러를 반환하는지 확인
		f := notifier.NewFactory()
		expectedErr := errors.New("processor failed")

		// Processor 1: Success
		f.RegisterProcessor(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
			return []notifier.NotifierHandler{&notificationmocks.MockNotifierHandler{}}, nil
		})

		// Processor 2: Error
		f.RegisterProcessor(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
			return nil, expectedErr
		})

		// Processor 3: Should NOT be called
		f.RegisterProcessor(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
			assert.Fail(t, "This processor should not be called after previous error")
			return nil, nil
		})

		handlers, err := f.CreateNotifiers(testConfig, mockExecutor)
		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, handlers, "Handlers should be nil on error")
	})
}

func TestFactory_RegisterProcessor_Safety(t *testing.T) {
	t.Run("Registering nil processor is safe", func(t *testing.T) {
		// 테스트 목적: nil 프로세서를 등록해도 패닉이 발생하지 않고 무시되는지 확인
		f := notifier.NewFactory()

		assert.NotPanics(t, func() {
			f.RegisterProcessor(nil)
		})

		// Verify it functions normally afterwards
		handlers, err := f.CreateNotifiers(&config.AppConfig{}, &taskmocks.MockExecutor{})
		require.NoError(t, err)
		assert.Empty(t, handlers)
	})
}
