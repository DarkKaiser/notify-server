package notification

import (
	"errors"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/darkkaiser/notify-server/internal/service/notification/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Interface Compliance Checks
// =============================================================================

// Sender Implementation Verification
// *Service가 contract.NotificationSender 인터페이스를 올바르게 구현하고 있는지 컴파일 타임에 검증합니다.
var _ contract.NotificationSender = (*Service)(nil)

// contract.NotificationHealthChecker Implementation Verification
// *Service가 contract.NotificationHealthChecker 인터페이스를 올바르게 구현하고 있는지 컴파일 타임에 검증합니다.
var _ contract.NotificationHealthChecker = (*Service)(nil)

// =============================================================================
// Helper Types for Testing
// =============================================================================

// FunctionalMockHandler는 Notify 메서드의 동작을 함수로 제어할 수 있는 Mock Wrapper입니다.
type FunctionalMockHandler struct {
	*mocks.MockNotifierHandler
	NotifyFunc func(contract.TaskContext, string) bool
}

func (m *FunctionalMockHandler) Notify(taskCtx contract.TaskContext, message string) bool {
	if m.NotifyFunc != nil {
		return m.NotifyFunc(taskCtx, message)
	}
	return m.MockNotifierHandler.Notify(taskCtx, message)
}

// =============================================================================
// Sender Interface Method Tests
// =============================================================================

// TestSender_NotifyWithTitle는 Sender.NotifyWithTitle 메서드의 동작을 검증합니다.
// 제목, 메시지, 에러 플래그가 올바르게 전달되는지, TaskContext가 올바르게 생성되는지 확인합니다.
func TestSender_NotifyWithTitle(t *testing.T) {
	tests := []struct {
		name          string
		notifierID    string
		title         string
		message       string
		errorOccurred bool
		expectError   bool
		expectedMsg   string
		setupMock     func(*FunctionalMockHandler)
	}{
		{
			name:          "성공: 일반 알림 전송",
			notifierID:    testNotifierID,
			title:         "Notice",
			message:       "This is a test message.",
			errorOccurred: false,
			expectError:   false,
			expectedMsg:   "This is a test message.",
			setupMock:     nil,
		},
		{
			name:          "성공: 에러 알림 전송 (Error Flag True)",
			notifierID:    testNotifierID,
			title:         "Error Alert",
			message:       "Something went wrong.",
			errorOccurred: true,
			expectError:   false,
			expectedMsg:   "Something went wrong.",
			setupMock: func(m *FunctionalMockHandler) {
				m.NotifyFunc = func(ctx contract.TaskContext, message string) bool {
					// 기록을 위해 기본 Notify 호출
					m.MockNotifierHandler.Notify(ctx, message)
					return ctx.IsErrorOccurred() == true
				}
			},
		},
		{
			name:        "실패: 존재하지 않는 Notifier",
			notifierID:  "unknown_notifier",
			title:       "Fail",
			message:     "This should fail.",
			expectError: true,
			setupMock:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			service, _, baseMock := setupMockServiceWithOptions(mockServiceOptions{
				notifierID:   testNotifierID,
				supportsHTML: true,
				running:      true,
			})

			// Wrap mock
			mockNotifier := &FunctionalMockHandler{MockNotifierHandler: baseMock}
			if tt.setupMock != nil {
				tt.setupMock(mockNotifier)
			}

			// Replace in service map only if ID matches testNotifierID,
			// otherwise we leave it as is or handle unknown ID case.
			if tt.notifierID == testNotifierID {
				service.notifiersMap[types.NotifierID(testNotifierID)] = mockNotifier
			}

			// Act
			err := service.NotifyWithTitle(types.NotifierID(tt.notifierID), tt.title, tt.message, tt.errorOccurred)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				require.NotEmpty(t, baseMock.NotifyCalls, "Notify should be called on the base mock")
				lastCall := baseMock.NotifyCalls[len(baseMock.NotifyCalls)-1]
				assert.Equal(t, tt.expectedMsg, lastCall.Message)

				// TaskContext Verification
				require.NotNil(t, lastCall.TaskCtx)
				if tCtx, ok := lastCall.TaskCtx.(contract.TaskContext); ok {
					assert.Equal(t, tt.title, tCtx.GetTitle())
					assert.Equal(t, tt.errorOccurred, tCtx.IsErrorOccurred())
				} else {
					// Fallback assertion if it's not a TaskContext (though it should be for NotifyWithTitle)
					// Or fail the test
					// assert.Fail(t, "TaskCtx is not contract.TaskContext")
				}
			}
		})
	}
}

// TestSender_NotifyDefault는 Sender.NotifyDefault 메서드의 동작을 검증합니다.
// 기본 Notifier로 메시지가 전달되는지 확인합니다.
func TestSender_NotifyDefault(t *testing.T) {
	tests := []struct {
		name            string
		message         string
		defaultNotifier string
		running         bool
		expectError     bool
		errorIs         error
		setupMock       func(*FunctionalMockHandler)
	}{
		{
			name:            "성공: 기본 Notifier로 전송",
			message:         "Default Message",
			defaultNotifier: defaultNotifierID,
			running:         true,
			expectError:     false,
		},
		{
			name:            "실패: 기본 Notifier 미설정 (Service Stopped 간주)",
			message:         "Fail Message",
			defaultNotifier: "", // 미설정
			running:         true,
			expectError:     true,
			errorIs:         notifier.ErrServiceStopped,
		},
		{
			name:            "실패: 메시지 전송 실패 (Queue Full 등)",
			message:         "Fail Message",
			defaultNotifier: defaultNotifierID,
			running:         true,
			expectError:     true,
			setupMock: func(m *FunctionalMockHandler) {
				m.NotifyFunc = func(ctx contract.TaskContext, message string) bool {
					return false // 실패 시뮬레이션
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			// Use defaultNotifierID if tt.defaultNotifier is set, otherwise use a placeholder that we will clear
			setupID := defaultNotifierID
			if tt.defaultNotifier != "" {
				setupID = tt.defaultNotifier
			}

			opts := mockServiceOptions{
				notifierID:   types.NotifierID(setupID),
				supportsHTML: true,
				running:      tt.running,
			}

			service, _, baseMock := setupMockServiceWithOptions(opts)

			mockNotifier := &FunctionalMockHandler{MockNotifierHandler: baseMock}
			if tt.setupMock != nil {
				tt.setupMock(mockNotifier)
			}

			// Inject wrapped mock if needed
			if service.defaultNotifier != nil {
				// ID가 일치하면 교체 (여기서는 setupMockServiceWithOptions가 생성한 mockNotifier를 사용하므로 항상 일치)
				service.defaultNotifier = mockNotifier
			}

			// If test case expects no default notifier, explicitly set to nil
			if tt.defaultNotifier == "" {
				service.defaultNotifier = nil
			}

			// Act
			err := service.NotifyDefault(tt.message)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorIs != nil {
					assert.ErrorIs(t, err, tt.errorIs)
				}
			} else {
				assert.NoError(t, err)
				assertNotifyCalled(t, baseMock, tt.message)
			}
		})
	}
}

// TestSender_NotifyDefaultWithError는 Sender.NotifyDefaultWithError 메서드를 검증합니다.
// 에러 플래그가 설정된 채로 기본 Notifier에 전달되는지 확인합니다.
func TestSender_NotifyDefaultWithError(t *testing.T) {
	// Arrange
	service, _, mockNotifier := setupMockServiceWithOptions(mockServiceOptions{
		notifierID:   defaultNotifierID,
		supportsHTML: true,
		running:      true,
	})

	message := "Critical Error"

	// Act
	err := service.NotifyDefaultWithError(message)

	// Assert
	assert.NoError(t, err)
	require.Len(t, mockNotifier.NotifyCalls, 1)

	call := mockNotifier.NotifyCalls[0]
	assert.Equal(t, message, call.Message)
	require.NotNil(t, call.TaskCtx)
	if tCtx, ok := call.TaskCtx.(contract.TaskContext); ok {
		assert.True(t, tCtx.IsErrorOccurred(), "Error flag should be true")
	}
}

// TestHealthChecker_Health는 HealthChecker.Health 메서드의 동작을 검증합니다.
// 서비스 실행 상태에 따른 반환값을 확인합니다.
func TestHealthChecker_Health(t *testing.T) {
	tests := []struct {
		name        string
		running     bool
		expectError bool
		errorIs     error
	}{
		{
			name:        "정상: 서비스 실행 중",
			running:     true,
			expectError: false,
		},
		{
			name:        "에러: 서비스 중지됨",
			running:     false,
			expectError: true,
			errorIs:     notifier.ErrServiceStopped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			service, _, _ := setupMockServiceWithOptions(mockServiceOptions{
				notifierID:   testNotifierID,
				supportsHTML: true,
				running:      tt.running,
			})

			// Act
			err := service.Health()

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorIs != nil {
					assert.ErrorIs(t, err, tt.errorIs)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestSender_Mockability는 Sender 및 HealthChecker 인터페이스가 Mocking 가능한지 시연합니다.
// 이는 "Interface Code" 자체에 대한 테스트의 일환으로 볼 수 있습니다.
type MockSender struct {
	NotifyWithTitleFunc        func(notifierID types.NotifierID, title string, message string, errorOccurred bool) error
	NotifyDefaultFunc          func(message string) error
	NotifyDefaultWithErrorFunc func(message string) error
	HealthFunc                 func() error // HealthChecker 구현
}

func (m *MockSender) NotifyWithTitle(notifierID types.NotifierID, title string, message string, errorOccurred bool) error {
	if m.NotifyWithTitleFunc != nil {
		return m.NotifyWithTitleFunc(notifierID, title, message, errorOccurred)
	}
	return nil
}
func (m *MockSender) NotifyDefault(message string) error {
	if m.NotifyDefaultFunc != nil {
		return m.NotifyDefaultFunc(message)
	}
	return nil
}
func (m *MockSender) NotifyDefaultWithError(message string) error {
	if m.NotifyDefaultWithErrorFunc != nil {
		return m.NotifyDefaultWithErrorFunc(message)
	}
	return nil
}
func (m *MockSender) Health() error {
	if m.HealthFunc != nil {
		return m.HealthFunc()
	}
	return nil
}

func TestHealthChecker_Mockability(t *testing.T) {
	// Arrange
	mock := &MockSender{
		HealthFunc: func() error {
			return errors.New("mock error")
		},
	}

	// Act
	var healthChecker contract.NotificationHealthChecker = mock
	err := healthChecker.Health()

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "mock error", err.Error())
}

// =============================================================================
// Helper Tests
// =============================================================================
// (Empty)
