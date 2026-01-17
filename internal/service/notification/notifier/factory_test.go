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
	t.Parallel()

	t.Run("NewFactory returns Factory interface", func(t *testing.T) {
		f := notifier.NewFactory()
		require.NotNil(t, f)
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
	t.Parallel()

	// Given
	mockExecutor := &taskmocks.MockExecutor{}
	testConfig := &config.AppConfig{
		Notifier: config.NotifierConfig{
			DefaultNotifierID: "default",
		},
	}
	handler1 := &notificationmocks.MockNotifierHandler{IDValue: "h1"}
	handler2 := &notificationmocks.MockNotifierHandler{IDValue: "h2"}

	tests := []struct {
		name             string
		registrations    []notifier.Creator
		expectError      bool
		expectedHandlers []notifier.NotifierHandler
		verifyCall       func(t *testing.T, cfg *config.AppConfig, executor contract.TaskExecutor) // Optional additional verification
	}{
		{
			name:             "No creators registered",
			registrations:    nil,
			expectError:      false,
			expectedHandlers: nil, // Expect empty
		},
		{
			name: "Single creator success",
			registrations: []notifier.Creator{
				notifier.FactoryFunc(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
					return []notifier.NotifierHandler{handler1}, nil
				}),
			},
			expectError:      false,
			expectedHandlers: []notifier.NotifierHandler{handler1},
		},
		{
			name: "Multiple creators aggregation",
			registrations: []notifier.Creator{
				notifier.FactoryFunc(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
					return []notifier.NotifierHandler{handler1}, nil
				}),
				notifier.FactoryFunc(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
					return []notifier.NotifierHandler{handler2}, nil
				}),
			},
			expectError:      false,
			expectedHandlers: []notifier.NotifierHandler{handler1, handler2},
		},
		{
			name: "Creator returns error (Fail Fast)",
			registrations: []notifier.Creator{
				notifier.FactoryFunc(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
					return []notifier.NotifierHandler{handler1}, nil
				}),
				notifier.FactoryFunc(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
					return nil, errors.New("creator failed")
				}),
				// This one should not be called
				notifier.FactoryFunc(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
					assert.Fail(t, "Should not be called")
					return nil, nil
				}),
			},
			expectError: true,
		},
		{
			name: "Argument Propagation Verification",
			registrations: []notifier.Creator{
				notifier.FactoryFunc(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
					assert.Equal(t, testConfig, cfg)
					assert.Equal(t, mockExecutor, executor)
					return nil, nil
				}),
			},
			expectError: false,
		},
		{
			name: "Nil creator registration safety",
			registrations: []notifier.Creator{
				notifier.FactoryFunc(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
					return []notifier.NotifierHandler{handler1}, nil
				}),
				nil, // Explicit nil registration
			},
			expectError:      false,
			expectedHandlers: []notifier.NotifierHandler{handler1},
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup
			f := notifier.NewFactory()
			for _, reg := range tt.registrations {
				f.Register(reg)
			}

			// Execution
			handlers, err := f.CreateNotifiers(testConfig, mockExecutor)

			// Verification
			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, handlers)
			} else {
				require.NoError(t, err)
				if tt.expectedHandlers == nil {
					assert.Empty(t, handlers)
				} else {
					assert.Equal(t, tt.expectedHandlers, handlers)
				}
			}

			if tt.verifyCall != nil {
				// Note: verifyCall is limited in parallel tests if attempting to capture arguments from closure.
				// For simple assertions inside the creator (like Argument Propagation case), it works directly.
			}
		})
	}
}

// TestExecutionOrder verifies that registered creators are executed in the order they were registered.
func TestFactory_ExecutionOrder(t *testing.T) {
	t.Parallel()

	f := notifier.NewFactory()
	var callOrder []string

	f.Register(notifier.FactoryFunc(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
		callOrder = append(callOrder, "first")
		return nil, nil
	}))

	f.Register(notifier.FactoryFunc(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
		callOrder = append(callOrder, "second")
		return nil, nil
	}))

	_, err := f.CreateNotifiers(&config.AppConfig{}, &taskmocks.MockExecutor{})
	require.NoError(t, err)

	assert.Equal(t, []string{"first", "second"}, callOrder)
}
