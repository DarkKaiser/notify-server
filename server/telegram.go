package server

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
)

type TelegramNotifier struct {
	bot *tgbotapi.BotAPI
	// 각 알림 객체는 고유의 ID를 가진다. 이건 json 파일에서 읽어올수 있도록 한다. 각 알림객체는 자신만의 데이터가 필요하기도 하다(계정정보 등)
}

func (t *TelegramNotifier) Init(token string) {
	// 파일에서 데이터 읽어오고 객체 초기화
	var err error
	t.bot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Panic(err)
	}

	t.bot.Debug = true

	log.Printf("Authorized on account %s", t.bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := t.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil { // ignore any non-Message Updates
			continue
		}

		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
		if update.Message.Text == "/help" {
			t.Notify("도움말은 아직이예요")
		}
	}
}

func (t *TelegramNotifier) Id() NotifierId {
	return NOTIFIER_TELEGRAM
}

func (t *TelegramNotifier) Notify(m string) bool {
	msg := tgbotapi.NewMessage(297396697, m)
	//msg.ReplyToMessageID = update.Message.MessageID
	t.bot.Send(msg)

	return true
}
