package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
)

// =============================================================================
// Telegram Bot Mock
// =============================================================================

// 컴파일 타임에 botClient 인터페이스 구현 여부를 검증합니다.
var _ botClient = (*MockTelegramBot)(nil)

// MockTelegramBot은 botClient 인터페이스의 Mock 구현체입니다.
//
// 이 Mock은 Telegram 봇 테스트에서 실제 Telegram API 호출 없이
// 봇 동작을 검증하는 데 사용됩니다.
//
// 주요 기능:
//   - 메시지 전송 시뮬레이션
//   - 업데이트 수신 시뮬레이션
//   - 봇 정보 조회 시뮬레이션
type MockTelegramBot struct {
	mock.Mock
}

// GetUpdatesChan은 Telegram 업데이트를 수신하는 채널을 반환합니다.
func (m *MockTelegramBot) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	args := m.Called(config)
	if ret := args.Get(0); ret != nil {
		if ch, ok := ret.(tgbotapi.UpdatesChannel); ok {
			return ch
		}
		// 채널 타입이 아닌 경우(예: chan tgbotapi.Update) 호환성을 위해 형변환 시도
		if ch, ok := ret.(chan tgbotapi.Update); ok {
			return ch
		}
		panic(fmt.Sprintf("GetUpdatesChan: unexpected return type: %T", ret))
	}
	return nil
}

// Send는 Telegram 메시지를 전송합니다.
func (m *MockTelegramBot) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	args := m.Called(c)
	// 두 번째 반환값(error)이 nil이 아니면 메시지는 빈 값일 수 있습니다.
	if args.Get(0) == nil {
		return tgbotapi.Message{}, args.Error(1)
	}
	return args.Get(0).(tgbotapi.Message), args.Error(1)
}

// StopReceivingUpdates는 업데이트 수신을 중지합니다.
func (m *MockTelegramBot) StopReceivingUpdates() {
	m.Called()
}

// GetSelf는 봇 자신의 정보를 반환합니다.
func (m *MockTelegramBot) GetSelf() tgbotapi.User {
	args := m.Called()
	if args.Get(0) == nil {
		return tgbotapi.User{}
	}
	return args.Get(0).(tgbotapi.User)
}
