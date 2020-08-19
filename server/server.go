package server

import (
	"github.com/darkkaiser/notify-server/global"
)

type NotifierId string

const (
	NOTIFIER_TELEGRAM NotifierId = "TELEGRAM"
)

type Notifier interface {
	Init(token string)
	Id() NotifierId
	Notify(m string) bool
}

type NotiServer struct {
	// 카카오톡, 텔레그램, 메일 등등으로 추가할 수 있도록 구성한다. 일단은 델레그램만 구현한다.
	// 알림 객체는 여러개 존재
	NotiList []Notifier
}

func (n *NotiServer) Start(appConfig *global.AppConfig) {
	for _, notifier := range appConfig.Notifiers {
		var iii = NotifierId(notifier.Id)
		if iii == NOTIFIER_TELEGRAM {
			// 등록 가능한 모든 알림을 등록한다.
			t := TelegramNotifier{}

			go t.Init(notifier.Token)

			n.NotiList = append(n.NotiList, &t)
		}
	}
}

func (n *NotiServer) Notify(id NotifierId, m string) {
	for _, notifier := range n.NotiList {
		if notifier.Id() == id {
			notifier.Notify(m)
			break
		}
	}
}
