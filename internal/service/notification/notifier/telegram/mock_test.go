package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
)

// =============================================================================
// Telegram Bot Mock
// =============================================================================

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
	updatesChan chan tgbotapi.Update
}

// GetUpdatesChan은 Telegram 업데이트를 수신하는 채널을 반환합니다.
func (m *MockTelegramBot) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	m.Called(config)
	if m.updatesChan == nil {
		m.updatesChan = make(chan tgbotapi.Update, 100)
	}
	return m.updatesChan
}

// Send는 Telegram 메시지를 전송합니다.
func (m *MockTelegramBot) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	args := m.Called(c)
	return args.Get(0).(tgbotapi.Message), args.Error(1)
}

// StopReceivingUpdates는 업데이트 수신을 중지합니다.
func (m *MockTelegramBot) StopReceivingUpdates() {
	m.Called()
}

// GetSelf는 봇 자신의 정보를 반환합니다.
func (m *MockTelegramBot) GetSelf() tgbotapi.User {
	args := m.Called()
	return args.Get(0).(tgbotapi.User)
}
