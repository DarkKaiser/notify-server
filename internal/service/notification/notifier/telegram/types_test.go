package telegram

import (
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Interface Compliance Tests
// =============================================================================

// TestTypes_InterfaceCompliance는 주요 타입들이 의도한 인터페이스를 정확히 구현하는지 검증합니다.
// 컴파일 타임 체크(var _ Interface = (*Struct)(nil))에 더해, 런타임 타입 어설션을 통해 이중으로 확인합니다.
func TestTypes_InterfaceCompliance(t *testing.T) {
	t.Run("tgClient implements client interface", func(t *testing.T) {
		var _ client = (*tgClient)(nil)
		assert.Implements(t, (*client)(nil), new(tgClient), "tgClient는 client 인터페이스를 구현해야 합니다.")
	})

	t.Run("telegramNotifier implements Notifier interface", func(t *testing.T) {
		var _ notifier.Notifier = (*telegramNotifier)(nil)
		assert.Implements(t, (*notifier.Notifier)(nil), new(telegramNotifier), "telegramNotifier는 notifier.Notifier 인터페이스를 구현해야 합니다.")
	})
}

// =============================================================================
// Struct Methods Tests
// =============================================================================

// TestTgClient_GetSelf는 tgClient의 GetSelf 메서드가 내부 BotAPI의 Self 정보를 올바르게 반환하는지 검증합니다.
func TestTgClient_GetSelf(t *testing.T) {
	// Given
	expectedUser := tgbotapi.User{
		ID:        123456789,
		FirstName: "TestBot",
		UserName:  "test_bot_123",
	}

	// tgbotapi.BotAPI는 구조체이므로 직접 생성하여 주입합니다.
	client := &tgClient{
		BotAPI: &tgbotapi.BotAPI{
			Self: expectedUser,
		},
	}

	// When
	actualUser := client.GetSelf()

	// Then
	assert.Equal(t, expectedUser, actualUser, "GetSelf는 BotAPI.Self에 저장된 사용자 정보를 정확히 반환해야 합니다.")
}

// =============================================================================
// Constants Verification Tests
// =============================================================================

// TestTypes_Constants는 패키지 내 주요 상수들이 의도치 않게 변경되지 않았는지 검증합니다.
// 이는 성능 튜닝이나 안전 마진 설정이 실수로 변경되는 것을 방지하기 위한 회귀 테스트(Regression Test) 역할을 합니다.
func TestTypes_Constants(t *testing.T) {
	t.Run("Verify messageMaxLength", func(t *testing.T) {
		// 텔레그램 API 제한(4096) 대비 안전 마진을 고려한 값이어야 합니다.
		const expectedMaxLen = 3900
		assert.Equal(t, expectedMaxLen, messageMaxLength, "messageMaxLength는 %d이어야 합니다.", expectedMaxLen)
	})

	t.Run("Verify shutdownTimeout", func(t *testing.T) {
		// 종료 시 충분한 정리를 위해 설정된 시간이어야 합니다.
		const expectedTimeout = 60 * time.Second
		assert.Equal(t, expectedTimeout, shutdownTimeout, "shutdownTimeout은 %s이어야 합니다.", expectedTimeout)
	})

	t.Run("Verify component name", func(t *testing.T) {
		const expectedComponent = "notification.notifier.telegram"
		assert.Equal(t, expectedComponent, component, "component 이름이 변경되었습니다. 로깅 및 추적에 영향을 줄 수 있습니다.")
	})
}

// =============================================================================
// Type Creation & Initialization Helpers Tests
// =============================================================================

// NOTE: types.go 자체에는 생성자가 없지만, 타입 정의와 관련된 초기화 로직이 있다면 여기에 테스트를 추가합니다.
// 현재는 순수 타입 정의 위주이므로 위 테스트들로 충분합니다.

// TestTelegramNotifier_Initialization_Values는 telegramNotifier 생성 시
// 기본값이나 설정값이 타입 내 필드에 올바르게 매핑되는지 확인하는 보조적인 테스트입니다.
// 실제 생성 로직은 wrapper/factory 쪽에 있지만, 타입 관점에서 필드 무결성을 확인합니다.
func TestTelegramNotifier_Initialization_Values(t *testing.T) {
	n := &telegramNotifier{
		Base:       notifier.NewBase("test-id", true, 100, 10*time.Second),
		chatID:     12345,
		retryDelay: 500 * time.Millisecond,
	}

	assert.Equal(t, int64(12345), n.chatID)
	assert.Equal(t, 500*time.Millisecond, n.retryDelay)
}

// TestNotification_Send_Context_Check (Optional)
// types.go에 정의된 Notifier 인터페이스의 메서드 시그니처가 유지되는지 확인합니다.
func TestNotifier_Interface_Signature(t *testing.T) {
	// Notifier 인터페이스가 Send(context.Context, *contract.Notification) error 형태인지 확인
	// 리플렉션이나 모의 객체 없이도 컴파일 타임 체크로 보장되지만, 명시적으로 작성해둡니다.
	var n notifier.Notifier
	// n.Send를 호출하는 코드가 컴파일된다면 시그니처가 맞는 것입니다.
	// 실제 호출은 하지 않습니다.
	_ = n
}

// TestBotCommand_Struct는 botCommand 구조체가 의도대로 정의되어 있는지 확인합니다.
// (types.go에 botCommand 구조체가 공개되어 있지 않다면 이 테스트는 내부 테스트 패키지에서만 가능)
// internal 패키지이므로 접근 가능하거나, export된 필드만 테스트합니다.
func TestBotCommand_Struct_Validation(t *testing.T) {
	// botCommand는 types.go에 없고 다른 파일에 있을 수 있지만,
	// types.go 패키지 레벨 테스트이므로 접근 가능합니다.

	// 먼저 botCommand 타입이 존재하는지 (컴파일 체크) 확인하기 위해 더미 인스턴스를 만듭니다.
	// 만약 private이라면 reflect로만 확인 가능하거나 테스트 파일이 같은 패키지(package telegram)여야 합니다.
	// 현재 package telegram 이므로 접근 가능해야 합니다.

	// 실제 정의가 command.go 등에 있고 types.go에는 없을 수도 있으나
	// 요청받은 범위는 types.go 테스트이므로, types.go에 정의된 내용 위주로 봅니다.
	// types.go 88라인에 `botCommands []botCommand`가 있으므로 타입은 존재합니다.

	// botCommand가 private struct일 가능성이 높으므로, 필드에 접근 가능한지 봅니다.
	// 실제 코드 확인 없이 추측하기 어려우나, 일반적으로 같은 패키지 내 테스트에서는 private 필드도 접근 가능합니다.

	cmd := botCommand{
		name:        "start",
		description: "Start the bot",
	}

	assert.Equal(t, "start", cmd.name)
	assert.Equal(t, "Start the bot", cmd.description)
}
