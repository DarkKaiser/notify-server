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

// NotifierHandler Implementation
var _ notifier.NotifierHandler = (*notificationmocks.MockNotifierHandler)(nil) // Test Mock도 인터페이스를 준수해야 함

// =============================================================================
// Compile-Time Verification Test
// =============================================================================

func TestNotifierHandlerInterface(t *testing.T) {
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
}

// =============================================================================
// Interface Method Verification Tests
// =============================================================================

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
