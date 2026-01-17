package notifier_test

import (
	"errors"
	"fmt"
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
// Interface Compliance
// =============================================================================

func TestFactory_Interfaces(t *testing.T) {
	t.Parallel()

	t.Run("Factory Compliance", func(t *testing.T) {
		var _ notifier.Factory = notifier.NewFactory()
		var _ notifier.Factory = (*notificationmocks.MockFactory)(nil)
	})

	t.Run("Creator Compliance", func(t *testing.T) {
		var _ notifier.Creator = notifier.CreatorFunc(nil)
		var _ notifier.Creator = notifier.NewFactory()
	})
}

// =============================================================================
// FactoryFunc Adapter Tests
// =============================================================================

func TestFactoryFunc_CreateNotifiers(t *testing.T) {
	t.Parallel()

	// Given
	called := false
	expectedNotifiers := []notifier.Notifier{notificationmocks.NewMockNotifier("test")}

	adapter := notifier.CreatorFunc(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.Notifier, error) {
		called = true
		return expectedNotifiers, nil
	})

	// When
	result, err := adapter.CreateNotifiers(nil, nil)

	// Then
	assert.True(t, called)
	assert.NoError(t, err)
	assert.Equal(t, expectedNotifiers, result)
}

// =============================================================================
// Factory Tests
// =============================================================================

func TestFactory_CreateNotifiers(t *testing.T) {
	t.Parallel()

	// Given Common Mocks
	mockExecutor := &taskmocks.MockExecutor{}
	testConfig := &config.AppConfig{
		Notifier: config.NotifierConfig{
			DefaultNotifierID: "default",
		},
	}
	n1 := notificationmocks.NewMockNotifier("n1")
	n2 := notificationmocks.NewMockNotifier("n2")

	// CreatorFunc를 사용하여 간단하게 Mock 생성
	// Helper to create a simple creator
	// Helper to create a simple creator
	createCreator := func(notifiers []notifier.Notifier, err error) notifier.Creator {
		return notifier.CreatorFunc(func(_ *config.AppConfig, _ contract.TaskExecutor) ([]notifier.Notifier, error) {
			return notifiers, err
		})
	}

	tests := []struct {
		name              string
		registrations     []notifier.Creator
		expectError       bool
		expectedNotifiers []notifier.Notifier
		expectedErrStr    string
	}{
		{
			name:              "Empty Factory (No Creators)",
			registrations:     nil,
			expectError:       false,
			expectedNotifiers: nil, // Should be empty or nil
		},
		{
			name: "Single Creator Success",
			registrations: []notifier.Creator{
				createCreator([]notifier.Notifier{n1}, nil),
			},
			expectError:       false,
			expectedNotifiers: []notifier.Notifier{n1},
		},
		{
			name: "Multiple Creators Aggregation",
			registrations: []notifier.Creator{
				createCreator([]notifier.Notifier{n1}, nil),
				createCreator([]notifier.Notifier{n2}, nil),
			},
			expectError:       false,
			expectedNotifiers: []notifier.Notifier{n1, n2},
		},
		{
			name: "Creator Returns Error (Fail Fast)",
			registrations: []notifier.Creator{
				createCreator([]notifier.Notifier{n1}, nil),            // Successful one
				createCreator(nil, errors.New("initialization error")), // Failing one
			},
			expectError:    true,
			expectedErrStr: "initialization error",
		},
		{
			name: "Nil Creator Registration (Robustness)",
			registrations: []notifier.Creator{
				createCreator([]notifier.Notifier{n1}, nil),
				nil, // Explicit nil should be ignored
			},
			expectError:       false,
			expectedNotifiers: []notifier.Notifier{n1},
		},
		{
			name: "Creator Returns Empty List",
			registrations: []notifier.Creator{
				createCreator([]notifier.Notifier{}, nil),
			},
			expectError:       false,
			expectedNotifiers: []notifier.Notifier{}, // Empty slice, likely
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			f := notifier.NewFactory()
			for _, reg := range tt.registrations {
				f.Register(reg)
			}

			// Act
			notifiers, err := f.CreateNotifiers(testConfig, mockExecutor)

			// Assert
			if tt.expectError {
				require.Error(t, err)
				if tt.expectedErrStr != "" {
					assert.Contains(t, err.Error(), tt.expectedErrStr)
				}
				assert.Nil(t, notifiers)
			} else {
				require.NoError(t, err)
				if len(tt.expectedNotifiers) == 0 {
					assert.Empty(t, notifiers)
				} else {
					assert.Equal(t, tt.expectedNotifiers, notifiers)
				}
			}
		})
	}
}

func TestFactory_ArgumentPropagation(t *testing.T) {
	t.Parallel()

	// Given
	expectedConfig := &config.AppConfig{Notifier: config.NotifierConfig{DefaultNotifierID: "verify"}}
	expectedExecutor := &taskmocks.MockExecutor{}

	f := notifier.NewFactory()
	called := false

	f.Register(notifier.CreatorFunc(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.Notifier, error) {
		called = true
		assert.Equal(t, expectedConfig, cfg, "Config should be propagated")
		assert.Equal(t, expectedExecutor, executor, "Executor should be propagated")
		return nil, nil
	}))

	// Act
	_, err := f.CreateNotifiers(expectedConfig, expectedExecutor)

	// Assert
	require.NoError(t, err)
	assert.True(t, called, "Creator should have been called")
}

func TestFactory_ExecutionOrder(t *testing.T) {
	t.Parallel()

	// Given
	f := notifier.NewFactory()
	var callOrder []string

	// Register multiple creators
	f.Register(notifier.CreatorFunc(func(_ *config.AppConfig, _ contract.TaskExecutor) ([]notifier.Notifier, error) {
		callOrder = append(callOrder, "first")
		return nil, nil
	}))
	f.Register(notifier.CreatorFunc(func(_ *config.AppConfig, _ contract.TaskExecutor) ([]notifier.Notifier, error) {
		callOrder = append(callOrder, "second")
		return nil, nil
	}))

	// Act
	_, err := f.CreateNotifiers(&config.AppConfig{}, &taskmocks.MockExecutor{})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"first", "second"}, callOrder, "Creators should be executed in registration order")
}

// =============================================================================
// Documentation Examples
// =============================================================================

func ExampleFactory() {
	// 1. Factory 생성
	f := notifier.NewFactory()

	// 2. Creator 등록 (일반적으로 패키지 init이나 메인 설정 단계에서 수행)
	f.Register(notifier.CreatorFunc(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.Notifier, error) {
		// 실제 구현에서는 여기서 설정(cfg)을 읽어 Notifier를 초기화합니다.
		fmt.Println("Initializing Custom Notifier")
		return []notifier.Notifier{}, nil // 예시를 위해 빈 목록 반환
	}))

	// 3. Notifier 생성 (앱 시작 시 호출)
	notifiers, err := f.CreateNotifiers(&config.AppConfig{}, nil)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Created %d notifiers\n", len(notifiers))

	// Output:
	// Initializing Custom Notifier
	// Created 0 notifiers
}
