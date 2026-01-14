package notifier_test

import (
	"testing"

	notificationmocks "github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Interface Compliance Checks
// =============================================================================

// 이 블록은 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
// 만약 구현체가 인터페이스를 충족하지 못하면, 컴파일 에러가 발생합니다.
//
// 검증 대상:
//   - Sender 인터페이스: Service
//   - NotifierHandler 인터페이스: telegramNotifier, mockNotifierHandler
//   - NotifierFactory 인터페이스: DefaultNotifierFactory, mockNotifierFactory
var (
	// Sender Implementation
	// _ Sender = (*Service)(nil) // Moved to notification package to avoid circular dependency

	// NotifierHandler Implementation
	_ notifier.NotifierHandler = (*notificationmocks.MockNotifierHandler)(nil) // Test Mock도 인터페이스를 준수해야 함

	// NotifierFactory Implementation
	_ notifier.NotifierFactory = (*notifier.DefaultNotifierFactory)(nil)
	_ notifier.NotifierFactory = (*notificationmocks.MockNotifierFactory)(nil) // Test Mock
)

// =============================================================================
// Compile-Time Verification Test
// =============================================================================

// TestInterfaces는 인터페이스 구현 검증을 위한 테스트 함수입니다.
//
// 이 함수는 컴파일 타임과 런타임 모두에서 인터페이스 구현을 검증합니다.
//
// 검증 방식:
//   - 컴파일 타임: var 블록의 타입 할당이 실패하면 컴파일 에러 발생
//   - 런타임: 인터페이스 타입 assertion으로 구현 여부 재확인
func TestInterfaces(t *testing.T) {
	/*
		t.Run("Sender interface", func(t *testing.T) {
			var service interface{} = &Service{}
			_, ok := service.(Sender)
			require.True(t, ok, "Service should implement Sender interface")
		})
	*/

	t.Run("NotifierHandler interface", func(t *testing.T) {
		tests := []struct {
			name string
			impl interface{}
		}{
			{"mockNotifierHandler", &notificationmocks.MockNotifierHandler{}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, ok := tt.impl.(notifier.NotifierHandler)
				require.True(t, ok, "%s should implement NotifierHandler interface", tt.name)
			})
		}
	})

	t.Run("NotifierFactory interface", func(t *testing.T) {
		tests := []struct {
			name string
			impl interface{}
		}{
			{"DefaultNotifierFactory", &notifier.DefaultNotifierFactory{}},
			{"mockNotifierFactory", &notificationmocks.MockNotifierFactory{}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, ok := tt.impl.(notifier.NotifierFactory)
				require.True(t, ok, "%s should implement NotifierFactory interface", tt.name)
			})
		}
	})
}

// =============================================================================
// Interface Method Verification Tests
// =============================================================================

// TestSenderInterfaceMethods는 Sender 인터페이스의 메서드 존재를 검증합니다.
func TestSenderInterfaceMethods(t *testing.T) {
	// var sender Sender = &Service{} // Moved to notification package
	// assert.NotNil(t, sender)
}

// TestNotifierHandlerInterfaceMethods는 NotifierHandler 인터페이스의 메서드 존재를 검증합니다.
func TestNotifierHandlerInterfaceMethods(t *testing.T) {
	var handler notifier.NotifierHandler = &notificationmocks.MockNotifierHandler{IDValue: "test"}

	// ID() 메서드 호출 가능 여부 확인
	id := handler.ID()
	assert.NotEmpty(t, id, "ID() should return non-empty value")

	// SupportsHTML() 메서드 호출 가능 여부 확인
	supportsHTML := handler.SupportsHTML()
	assert.NotNil(t, supportsHTML, "SupportsHTML() should return a boolean value")
}
