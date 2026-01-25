package mocks

import (
	"context"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/stretchr/testify/mock"
)

// Interface Compliance Checks
// 컴파일 타임에 Mock 객체가 인터페이스를 올바르게 구현하고 있는지 검증합니다.
var _ contract.NotificationSender = (*MockNotificationSender)(nil)
var _ contract.NotificationHealthChecker = (*MockNotificationSender)(nil)

// MockNotificationSender 테스트용 NotificationSender 및 HealthChecker 구현체입니다.
// "github.com/stretchr/testify/mock" 패키지를 사용하여 동작을 정의하고 검증할 수 있습니다.
type MockNotificationSender struct {
	mock.Mock
}

// NewMockNotificationSender 새로운 MockNotificationSender 인스턴스를 생성합니다.
//
// 주요 기능:
//   - t.Cleanup을 사용하여 테스트 종료 시 AssertExpectations 자동 호출
//   - SupportsHTML 메서드에 대한 기본 허용적 설정을 적용합니다.
func NewMockNotificationSender(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockNotificationSender {
	m := &MockNotificationSender{}
	m.Mock.Test(t)

	t.Cleanup(func() {
		m.AssertExpectations(t)
	})

	// 기본 동작: SupportsHTML은 상태 변경이 없으므로 기본적으로 true 반환
	m.On("SupportsHTML", mock.Anything).Return(true).Maybe()

	return m
}

// =============================================================================
// Interface Implementation
// =============================================================================

// Notify 메타데이터와 함께 알림을 전송합니다.
func (m *MockNotificationSender) Notify(ctx context.Context, notification contract.Notification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

// SupportsHTML HTML 지원 여부를 반환합니다.
func (m *MockNotificationSender) SupportsHTML(notifierID contract.NotifierID) bool {
	args := m.Called(notifierID)
	return args.Bool(0)
}

// Health 서비스 상태를 확인합니다.
func (m *MockNotificationSender) Health() error {
	args := m.Called()
	return args.Error(0)
}

// =============================================================================
// Fluent Helpers (Expectation Setup)
// =============================================================================

// OnNotify Notify 메서드 호출에 대한 기대치를 설정합니다.
func (m *MockNotificationSender) OnNotify(ctx interface{}, notification interface{}) *mock.Call {
	if ctx == nil {
		ctx = mock.Anything
	}
	if notification == nil {
		notification = mock.Anything
	}
	return m.On("Notify", ctx, notification)
}

// ExpectNotifySuccess Notify 호출 성공을 가정합니다.
func (m *MockNotificationSender) ExpectNotifySuccess() *mock.Call {
	return m.OnNotify(mock.Anything, mock.Anything).Return(nil)
}

// ExpectNotifyFailure Notify 호출 실패를 가정합니다.
func (m *MockNotificationSender) ExpectNotifyFailure(err error) *mock.Call {
	return m.OnNotify(mock.Anything, mock.Anything).Return(err)
}

// OnHealth Health 메서드 호출에 대한 기대치를 설정합니다.
func (m *MockNotificationSender) OnHealth() *mock.Call {
	return m.On("Health")
}

// ExpectHealthy Health 호출 시 정상(nil error)을 가정합니다.
func (m *MockNotificationSender) ExpectHealthy() *mock.Call {
	return m.OnHealth().Return(nil)
}

// ExpectUnhealthy Health 호출 시 에러를 가정합니다.
func (m *MockNotificationSender) ExpectUnhealthy(err error) *mock.Call {
	return m.OnHealth().Return(err)
}

// =============================================================================
// Deprecated / Legacy Helpers (Backward Compatibility)
// =============================================================================

// WithNotify configures the mock to return a specific error for Notify calls.
// Deprecated: Use OnNotify or ExpectNotifySuccess instead.
func (m *MockNotificationSender) WithNotify(err error) *MockNotificationSender {
	m.On("Notify", mock.Anything, mock.Anything).Return(err)
	return m
}

// WithHealthy configures the mock to return nil for Health calls (healthy state).
// Deprecated: Use ExpectHealthy instead.
func (m *MockNotificationSender) WithHealthy() *MockNotificationSender {
	m.On("Health").Return(nil)
	return m
}

// WithUnhealthy configures the mock to return an error for Health calls (unhealthy state).
// Deprecated: Use ExpectUnhealthy instead.
func (m *MockNotificationSender) WithUnhealthy(err error) *MockNotificationSender {
	m.On("Health").Return(err)
	return m
}
