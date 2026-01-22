package telegram

import (
	"fmt"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
)

// =============================================================================
// Telegram Bot Mock
// =============================================================================

// 컴파일 타임에 client 인터페이스 구현 여부를 검증합니다.
var _ client = (*MockTelegramBot)(nil)

// MockTelegramBot Telegram Bot API(client)의 Mock 구현체입니다.
// stretchr/testify/mock을 사용하여 동작을 모의(Mocking)하고 호출을 검증(Assertion)합니다.
//
// 주요 기능:
//   - NewMockTelegramBot 팩토리 함수 제공
//   - 채널(chan Update)과 UpdatesChannel 간의 유연한 타입 변환 지원
type MockTelegramBot struct {
	mock.Mock
}

// NewMockTelegramBot 새로운 MockTelegramBot 인스턴스를 생성합니다.
// t를 전달하면, Mock 객체가 테스트 컨텍스트를 인지하여 테스트 종료 시 Cleanup 등을 연동할 수 있습니다.
func NewMockTelegramBot(t *testing.T) *MockTelegramBot {
	m := &MockTelegramBot{}
	m.Test(t)
	return m
}

// GetUpdatesChan 업데이트 수신 채널을 반환합니다.
//
// Mock 설정 예시:
//
//	updates := make(chan tgbotapi.Update, 100)
//	mockBot.On("GetUpdatesChan", mock.Anything).Return(updates)
func (m *MockTelegramBot) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	args := m.Called(config)
	return getUpdatesChannel(args.Get(0))
}

// Send 메시지를 전송합니다.
//
// Mock 설정 예시:
//
//	mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.MessageConfig) bool {
//	    return c.Text == "expected message"
//	})).Return(tgbotapi.Message{MessageID: 1}, nil)
func (m *MockTelegramBot) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	args := m.Called(c)

	// 첫 번째 리턴값(Message) 처리
	var msg tgbotapi.Message
	if args.Get(0) != nil {
		msg = args.Get(0).(tgbotapi.Message)
	}

	// 두 번째 리턴값(error) 처리
	return msg, args.Error(1)
}

// StopReceivingUpdates 업데이트 수신 중지를 요청합니다.
func (m *MockTelegramBot) StopReceivingUpdates() {
	m.Called()
}

// GetSelf 봇 자신의 정보를 반환합니다.
func (m *MockTelegramBot) GetSelf() tgbotapi.User {
	args := m.Called()

	if args.Get(0) != nil {
		return args.Get(0).(tgbotapi.User)
	}
	return tgbotapi.User{}
}

// -----------------------------------------------------------------------------
// Internal Helpers
// -----------------------------------------------------------------------------

// getUpdatesChannel Mock 리턴값을 tgbotapi.UpdatesChannel로 안전하게 변환합니다.
func getUpdatesChannel(ret interface{}) tgbotapi.UpdatesChannel {
	if ret == nil {
		return nil
	}

	// 1. 이미 정확한 타입인 경우
	if ch, ok := ret.(tgbotapi.UpdatesChannel); ok {
		return ch
	}

	// 2. 양방향 채널(chan tgbotapi.Update)인 경우 (테스트 코드에서 주로 생성)
	// tgbotapi.UpdatesChannel은 읽기 전용(<-chan)이므로, 명시적인 형변환이 필요하지 않지만
	// interface{}에서 꺼낼 때는 타입 어설션이 엄격하므로 별도 처리가 필요할 수 있습니다.
	// 하지만 Go의 타입 시스템상 `chan T`는 `<-chan T` 인터페이스에 할당되지 않고,
	// `args.Get(0)`은 interface{}를 반환하므로 동적 타입이 `chan T`면 `.(<-chan T)` 어설션은 실패합니다.
	// 따라서 `chan T`로 꺼낸 뒤 리턴 시점에 묵시적 변환을 이용합니다.
	if ch, ok := ret.(chan tgbotapi.Update); ok {
		return ch
	}

	panic(fmt.Sprintf("MockTelegramBot.GetUpdatesChan: unexpected return type: %T. Expected 'chan tgbotapi.Update' or 'tgbotapi.UpdatesChannel'", ret))
}
