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
// Interface Compliance Checks
// =============================================================================

func TestInterfaces(t *testing.T) {
	// 정적 타입 검증 (Compile-time Check)
	var _ notifier.Factory = notifier.NewFactory()
	var _ notifier.Creator = notifier.CreatorFunc(nil)
}

// =============================================================================
// Factory Behavior Tests
// =============================================================================

func TestFactory_CreateAll(t *testing.T) {
	t.Parallel()

	// 공통 Mock 객체
	mockMsgExecutor := &taskmocks.MockExecutor{}
	cfg := &config.AppConfig{}

	// Test case 구조체의 가독성을 높여 정의
	tests := []struct {
		name              string
		creators          []notifier.Creator
		expectError       bool
		expectedErrStr    string
		expectedCount     int
		expectedNotifiers []string // ID 목록으로 검증
	}{
		{
			name:          "Success: No Creators (Empty)",
			creators:      nil,
			expectError:   false,
			expectedCount: 0,
		},
		{
			name: "Success: Single Creator",
			creators: []notifier.Creator{
				createMockCreator("n1"),
			},
			expectError:       false,
			expectedCount:     1,
			expectedNotifiers: []string{"n1"},
		},
		{
			name: "Success: Multiple Creators (Aggregation)",
			creators: []notifier.Creator{
				createMockCreator("n1"),
				createMockCreator("n2", "n3"),
			},
			expectError:       false,
			expectedCount:     3,
			expectedNotifiers: []string{"n1", "n2", "n3"},
		},
		{
			name: "Failure: Creator Returns Error (Fail Fast)",
			creators: []notifier.Creator{
				createMockCreator("n1"), // 성공
				notifier.CreatorFunc(func(_ *config.AppConfig, _ contract.TaskExecutor) ([]notifier.Notifier, error) {
					return nil, errors.New("init failed") // 실패
				}),
				createMockCreator("n2"), // 실행되지 않아야 함
			},
			expectError:    true,
			expectedErrStr: "init failed",
		},
		{
			name: "Robustness: Nil Creator Registration",
			creators: []notifier.Creator{
				createMockCreator("n1"),
				nil, // 무시되어야 함
			},
			expectError:       false,
			expectedCount:     1,
			expectedNotifiers: []string{"n1"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			f := notifier.NewFactory()
			for _, c := range tt.creators {
				f.Register(c)
			}

			// Act
			results, err := f.CreateAll(cfg, mockMsgExecutor)

			// Assert
			if tt.expectError {
				require.Error(t, err)
				if tt.expectedErrStr != "" {
					assert.Contains(t, err.Error(), tt.expectedErrStr)
				}
			} else {
				require.NoError(t, err)
				assert.Len(t, results, tt.expectedCount)

				if len(tt.expectedNotifiers) > 0 {
					var ids []string
					for _, n := range results {
						ids = append(ids, string(n.ID()))
					}
					assert.Equal(t, tt.expectedNotifiers, ids)
				}
			}
		})
	}
}

// TestFactory_Order_Preservation 는 등록된 순서대로 Creator가 실행되는지 검증합니다.
func TestFactory_Order_Preservation(t *testing.T) {
	t.Parallel()

	f := notifier.NewFactory()
	var actualOrder []string

	// Register 1
	f.Register(notifier.CreatorFunc(func(_ *config.AppConfig, _ contract.TaskExecutor) ([]notifier.Notifier, error) {
		actualOrder = append(actualOrder, "first")
		return nil, nil
	}))

	// Register 2
	f.Register(notifier.CreatorFunc(func(_ *config.AppConfig, _ contract.TaskExecutor) ([]notifier.Notifier, error) {
		actualOrder = append(actualOrder, "second")
		return nil, nil
	}))

	_, err := f.CreateAll(&config.AppConfig{}, nil)
	require.NoError(t, err)

	assert.Equal(t, []string{"first", "second"}, actualOrder, "Creators must be executed in the order of registration")
}

// =============================================================================
// Helper Functions
// =============================================================================

// createMockCreator는 지정된 ID를 가진 MockNotifier들을 반환하는 Creator를 생성합니다.
func createMockCreator(ids ...string) notifier.Creator {
	return notifier.CreatorFunc(func(_ *config.AppConfig, _ contract.TaskExecutor) ([]notifier.Notifier, error) {
		var list []notifier.Notifier
		for _, id := range ids {
			list = append(list, notificationmocks.NewMockNotifier(contract.NotifierID(id)))
		}
		return list, nil
	})
}

// =============================================================================
// Documentation Examples
// =============================================================================

func ExampleFactory() {
	// 1. Factory 생성
	f := notifier.NewFactory()

	// 2. Creator 등록 (앱 초기화 단계)
	f.Register(notifier.CreatorFunc(func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.Notifier, error) {
		// 설정(cfg)을 기반으로 Notifier 생성 로직 구현
		fmt.Println("Initializing specific notifier...")
		return []notifier.Notifier{}, nil
	}))

	// 3. 일괄 생성 (서비스 시작 단계)
	notifiers, _ := f.CreateAll(&config.AppConfig{}, nil)

	fmt.Printf("Total notifiers: %d\n", len(notifiers))

	// Output:
	// Initializing specific notifier...
	// Total notifiers: 0
}
