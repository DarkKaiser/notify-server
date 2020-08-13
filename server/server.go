package server

import "github.com/darkkaiser/notify-server/telegram"

type NotiServer struct {
	NotiList []Notifier
	// 알림 객체는 여러개 존재
	// 카카오톡, 텔레그램, 메일 등등으로 추가할 수 있도록 구성한다. 일단은 델레그램만 구현한다.

}

func (n *NotiServer) Start() {
	// 등록 가능한 모든 알림을 등록한다.
	t := telegram.TelegramNotifier{}
	t.Init()

	n.NotiList = append(n.NotiList, &t)
}

func (n *NotiServer) Notify(id string, m string) {
	for _, notifier := range n.NotiList {
		if notifier.Id() == id {
			notifier.Notify(m)
			break
		}
	}
}

type Notifier interface {
	Init()
	Id() string
	Name() string
	Notify(m string) bool
}
