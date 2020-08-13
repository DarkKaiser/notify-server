package telegram

type TelegramNotifier struct {
	// 각 알림 객체는 고유의 ID를 가진다. 이건 json 파일에서 읽어올수 있도록 한다. 각 알림객체는 자신만의 데이터가 필요하기도 하다(계정정보 등)
}

func (t *TelegramNotifier) Init() {
	// 파일에서 데이터 읽어오고 객체 초기화
}

func (t *TelegramNotifier) Id() string {
	return "dd"
}

func (t *TelegramNotifier) Name() string {
	return "dd"
}

func (t *TelegramNotifier) Notify(m string) bool {
	return true
}
