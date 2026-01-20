package mocks

import (
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/stretchr/testify/mock"
)

// Interface Compliance Checks
// 컴파일 타임에 Mock 객체가 인터페이스를 올바르게 구현하고 있는지 검증합니다.
var _ contract.NotificationSender = (*MockNotificationSender)(nil)
var _ contract.NotificationHealthChecker = (*MockNotificationSender)(nil)

// MockNotificationSender 테스트용 NotificationSender 및 HealthChecker 구현체입니다.
type MockNotificationSender struct {
	mock.Mock
}

// NewMockNotificationSender 새로운 Mock 객체를 생성합니다.
func NewMockNotificationSender() *MockNotificationSender {
	return &MockNotificationSender{}
}

// Notify 메타데이터와 함께 알림을 전송합니다.
func (m *MockNotificationSender) Notify(ctx contract.TaskContext, notifierID contract.NotifierID, message string) error {
	args := m.Called(ctx, notifierID, message)
	return args.Error(0)
}

// NotifyWithTitle 제목을 포함하여 알림을 전송합니다.
func (m *MockNotificationSender) NotifyWithTitle(notifierID contract.NotifierID, title string, message string, errorOccurred bool) error {
	args := m.Called(notifierID, title, message, errorOccurred)
	return args.Error(0)
}

// NotifyDefault 기본 알림을 전송합니다.
func (m *MockNotificationSender) NotifyDefault(message string) error {
	args := m.Called(message)
	return args.Error(0)
}

// NotifyDefaultWithError 오류 알림을 전송합니다.
func (m *MockNotificationSender) NotifyDefaultWithError(message string) error {
	args := m.Called(message)
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
