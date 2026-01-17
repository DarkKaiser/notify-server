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

// Notifier Implementation
var _ notifier.Notifier = (*notificationmocks.MockNotifier)(nil) // Test Mock도 인터페이스를 준수해야 함

// =============================================================================
// Compile-Time Verification Test
// =============================================================================

func TestNotifierInterface(t *testing.T) {
	tests := []struct {
		name string
		impl interface{}
	}{
		{"mockNotifier", notificationmocks.NewMockNotifier("mock")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := tt.impl.(notifier.Notifier)
			require.True(t, ok, "%s should implement Notifier interface", tt.name)
		})
	}
}

// =============================================================================
// Interface Method Verification Tests
// =============================================================================

// TestNotifierInterfaceMethods는 Notifier 인터페이스의 메서드 존재를 검증합니다.
func TestNotifierInterfaceMethods(t *testing.T) {
	var notifier notifier.Notifier = notificationmocks.NewMockNotifier("test")

	// ID() 메서드 호출 가능 여부 확인
	id := notifier.ID()
	assert.NotEmpty(t, id, "ID() should return non-empty value")

	// SupportsHTML() 메서드 호출 가능 여부 확인
	supportsHTML := notifier.SupportsHTML()
	assert.NotNil(t, supportsHTML, "SupportsHTML() should return a boolean value")
}
