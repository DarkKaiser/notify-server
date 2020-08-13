package server

type NotiServer struct {
	NotiList []int
	// 알림 객체는 여러개 존재
	// 카카오톡, 텔레그램, 메일 등등으로 추가할 수 있도록 구성한다. 일단은 델레그램
	// 각 알림 객체는 고유의 ID를 가진다. 이건 json 파일에서 읽어올수 있도록 한다. 각 알림객체는 자신만의 데이터가 필요하기도 하다(계정정보 등)
}

func (n *NotiServer) Start() {
	// 알림서버 시작
}
