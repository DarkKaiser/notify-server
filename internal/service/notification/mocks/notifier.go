package mocks

import (
	"context"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/stretchr/testify/mock"
)

// Interface Compliance Check
var _ notifier.Notifier = (*MockNotifier)(nil)

// MockNotifier는 Notifier 인터페이스의 Mock 구현체입니다.
// "github.com/stretchr/testify/mock" 패키지를 사용하여 동작을 정의하고 검증할 수 있습니다.
type MockNotifier struct {
	mock.Mock
}

// NewMockNotifier는 새로운 MockNotifier 인스턴스를 생성합니다.
//
// 주요 기능:
//   - t.Cleanup을 사용하여 테스트 종료 시 AssertExpectations 자동 호출
//   - ID, Run, Done 등 기본 메서드에 대한 허용적(Permissive) Mock 설정 자동 적용
//     (일일이 설정하지 않아도 테스트가 실패하지 않도록)
func NewMockNotifier(t interface {
	mock.TestingT
	Cleanup(func())
}, id contract.NotifierID) *MockNotifier {
	m := &MockNotifier{}
	m.Mock.Test(t)

	t.Cleanup(func() {
		m.AssertExpectations(t)
	})

	// 1. Immutable ID 설정 (항상 설정된 ID 반환)
	m.On("ID").Return(id).Maybe()

	// 2. Lifecycle 메서드 기본 동작 (Boilerplate 감소용)
	// 명시적 검증이 필요 없으면 Run/Done 호출을 허용합니다.
	m.On("Run", mock.Anything).Return().Maybe()
	m.On("Done").Return(nil).Maybe()
	m.On("SupportsHTML").Return(true).Maybe() // 기본적으로 HTML 지원 가정
	m.On("Close").Return().Maybe()            // 종료 시 Close 호출 허용

	return m
}

// =============================================================================
// Interface Implementation
// =============================================================================

// ID returns the notifier's ID.
func (m *MockNotifier) ID() contract.NotifierID {
	args := m.Called()
	return args.Get(0).(contract.NotifierID)
}

// Send sends a notification.
func (m *MockNotifier) Send(ctx context.Context, notification contract.Notification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

// TrySend sends a notification without blocking.
func (m *MockNotifier) TrySend(ctx context.Context, notification contract.Notification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

// Run runs the notifier.
func (m *MockNotifier) Run(ctx context.Context) {
	m.Called(ctx)
}

// Close closes the notifier.
func (m *MockNotifier) Close() {
	m.Called()
}

// Done returns a channel that is closed when the notifier is done.
func (m *MockNotifier) Done() <-chan struct{} {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	// 안전한 타입 변환
	if ch, ok := args.Get(0).(<-chan struct{}); ok {
		return ch
	}
	if ch, ok := args.Get(0).(chan struct{}); ok {
		return ch
	}
	return nil
}

// SupportsHTML returns whether the notifier supports HTML.
func (m *MockNotifier) SupportsHTML() bool {
	args := m.Called()
	return args.Bool(0)
}

// =============================================================================
// Fluent Helpers (Expectation Setup)
// =============================================================================

// OnSend Send 메서드 호출에 대한 기대치를 설정합니다.
func (m *MockNotifier) OnSend(ctx interface{}, notification interface{}) *mock.Call {
	if ctx == nil {
		ctx = mock.Anything
	}
	if notification == nil {
		notification = mock.Anything
	}
	return m.On("Send", ctx, notification)
}

// ExpectSendSuccess Send 호출 성공을 가정합니다.
func (m *MockNotifier) ExpectSendSuccess() *mock.Call {
	return m.OnSend(mock.Anything, mock.Anything).Return(nil)
}

// ExpectSendFailure Send 호출 실패를 가정합니다.
func (m *MockNotifier) ExpectSendFailure(err error) *mock.Call {
	return m.OnSend(mock.Anything, mock.Anything).Return(err)
}

// OnTrySend TrySend 메서드 호출에 대한 기대치를 설정합니다.
func (m *MockNotifier) OnTrySend(ctx interface{}, notification interface{}) *mock.Call {
	if ctx == nil {
		ctx = mock.Anything
	}
	if notification == nil {
		notification = mock.Anything
	}
	return m.On("TrySend", ctx, notification)
}

// ExpectTrySendSuccess TrySend 호출 성공을 가정합니다.
func (m *MockNotifier) ExpectTrySendSuccess() *mock.Call {
	return m.OnTrySend(mock.Anything, mock.Anything).Return(nil)
}

// ExpectTrySendFailure TrySend 호출 실패를 가정합니다.
func (m *MockNotifier) ExpectTrySendFailure(err error) *mock.Call {
	return m.OnTrySend(mock.Anything, mock.Anything).Return(err)
}

// OnClose Close 메서드 호출에 대한 기대치를 설정합니다.
func (m *MockNotifier) OnClose() *mock.Call {
	return m.On("Close")
}

// ExpectClose Close 호출을 가정합니다.
func (m *MockNotifier) ExpectClose() *mock.Call {
	return m.OnClose().Return()
}

// =============================================================================
// Deprecated / Legacy Helpers (Backward Compatibility)
// =============================================================================

// WithSend configures the mock to return a specific error for Send calls.
// Deprecated: Use OnSend or ExpectSendSuccess instead.
func (m *MockNotifier) WithSend(err error) *MockNotifier {
	m.On("Send", mock.Anything, mock.Anything).Return(err)
	return m
}

// WithSupportsHTML configures the mock to return a specific boolean for SupportsHTML calls.
// Deprecated: Use On("SupportsHTML").Return(supported) instead.
func (m *MockNotifier) WithSupportsHTML(supported bool) *MockNotifier {
	m.On("SupportsHTML").Return(supported)
	return m
}
