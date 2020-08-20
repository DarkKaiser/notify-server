package notifiers

import (
	"github.com/darkkaiser/notify-server/global"
)

type NotifierId string

const (
	NOTIFIER_TELEGRAM NotifierId = "darkkaiser_notify_bot"
)

type Notifier interface {
	Id() NotifierId
	Init(token string)
	Notify(m string) bool
}

type NotifierManager struct {
	NotiList []Notifier
}

func (n *NotifierManager) Start(appConfig *global.AppConfig) {
	for _, tgm := range appConfig.Notifiers.Telegrams {
		var iii = NotifierId(tgm.Id)
		if iii == NOTIFIER_TELEGRAM {
			// 등록 가능한 모든 알림을 등록한다.
			t := TelegramNotifier{}

			t.Init(tgm.Token)

			n.NotiList = append(n.NotiList, &t)
		} else {

		}
	}
}

func (n *NotifierManager) Notify(id NotifierId, m string) {
	for _, notifier := range n.NotiList {
		if notifier.Id() == id {
			notifier.Notify(m)
			break
		}
	}
}
