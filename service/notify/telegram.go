package notify

import (
	"context"
	"github.com/darkkaiser/notify-server/global"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	log "github.com/sirupsen/logrus"
	"sync"
)

type telegramNotifyService struct {
	serviceCtx        context.Context
	serviceStopWaiter *sync.WaitGroup

	id     NotifierId
	token  string
	chatId int64

	// @@@@@@
	// 각 알림 객체는 고유의 ID를 가진다. 이건 json 파일에서 읽어올수 있도록 한다. 각 알림객체는 자신만의 데이터가 필요하기도 하다(계정정보 등)
	bot *tgbotapi.BotAPI
}

// @@@@@
func newTelegramNotifyService(serviceCtx context.Context, serviceStopWaiter *sync.WaitGroup, id NotifierId, token string, chatId int64) *telegramNotifyService {
	return &telegramNotifyService{
		serviceCtx:        serviceCtx,
		serviceStopWaiter: serviceStopWaiter,

		id:     id,
		token:  token,
		chatId: chatId,
	}
}

// @@@@@
func (s *telegramNotifyService) Run(*global.AppConfig) {
	// 파일에서 데이터 읽어오고 객체 초기화
	var err error
	s.bot, err = tgbotapi.NewBotAPI(s.token)
	if err != nil {
		log.Panic(err)
	}

	s.bot.Debug = true

	log.Printf("Authorized on account %s", s.bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := s.bot.GetUpdatesChan(u)

	go func() {
		defer s.serviceStopWaiter.Done()

		for {
			select {
			case update := <-updates:
				if update.Message == nil { // ignore any non-Message Updates
					continue
				}

				log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

				if update.Message.Text == "/help" {
					s.Notify("도움말은 아직이예요")
				} else if update.Message.Text == "/new_alganicmall_event" {
					ctx := context.Background()
					ctx = context.WithValue(ctx, "chatId", update.Message.Chat.ID)
					ctx = context.WithValue(ctx, "messageId", update.Message.MessageID)

					//					add <- struct {
					//						taskId : TI_ALGANICMALL,
					//						commandId : TCI_ALGANICMALL_CRAWING,
					//						ctx : ctx,
					//					}
				}
				//			case receive<-notifymessage:받을때 context를 그대로 받는다.
				//				msg := tgbotapi.NewMessage(297396697, m)
				//				//msg.ReplyToMessageID = update.Message.MessageID
				//				s.bot.Send(msg)
			case <-s.serviceCtx.Done():
				log.Info("telegram 종료중...")
				//close(tm.add)
				//close(tm.cancel)
				//close(tm.taskcancel)
				//n.twg.Wait()
				log.Info("telegram 종료됨")
				return
			}
		}

		//		for update := range updates {
		//		}
	}()
}

// @@@@@
func (s *telegramNotifyService) _run_() {

}

// @@@@@
func (s *telegramNotifyService) Id() NotifierId {
	return s.id
}

//@@@@@ XXXXX channel로 수신
func (s *telegramNotifyService) Notify(m string) bool {
	msg := tgbotapi.NewMessage(297396697, m)
	//msg.ReplyToMessageID = update.Message.MessageID
	s.bot.Send(msg)

	return true
}
