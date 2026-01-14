package notification_test

import (
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/notification"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Interface Compliance Checks
// =============================================================================

// Sender Implementation
var _ notification.Sender = (*notification.Service)(nil)

// =============================================================================
// Interface Method Verification Tests
// =============================================================================

// TestSenderInterfaceMethods는 Sender 인터페이스의 메서드 존재를 검증합니다.
func TestSenderInterfaceMethods(t *testing.T) {
	var sender notification.Sender = &notification.Service{}
	assert.NotNil(t, sender)
}
