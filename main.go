package main

import (
	"fmt"
	"github.com/robfig/cron"
)

func main() {
	fmt.Print("hello")

}

type Cron struct {
	// task를 모두 등록하여 관리한다.
}

func (c *Cron) Start() {
	cc := cron.New()
	cc.AddFunc("30 * * * *", func() { fmt.Println("30 분마다") })

	// @@@@@ 이미 등록된 테스크중에서 바로 실행되어야 하는경우는 어떻게 할 것인가?
	// AddFunc() 함수 2번째 인자로 넘어가는 함수를 저장해뒀다가 다시 Add하고 작업 끝나면 Remove????
	//
	// 함수목록을 관리??(또는 그때그때 함수를 생성해서 고루틴으로 돌려도 됨, 함수생성시 어떤 노티에 보내야 하는지 설정필요)
	// 함수목록을 cron에 등록하여 지정된 시간에 실행
	// 사용자로부터 요청이 들어오면 해당 함수를 찾아서 고루틴으로 실행, waitgroup을 둬서 stop시 실행이 종료될때까지 대기하도록 한다.
	// 스레드 안전하게 테스크를 실행가능하도록 만들어야 한다.
}

type Task interface {
	Id() int
	Run() bool
}

type AlganicMallTask struct {
}

func (a *AlganicMallTask) Id() int {
	return 1
}

func (a *AlganicMallTask) Run() bool {
	// 웹 크롤링해서 이벤트를 로드하고 Noti로 알린다.
	// 각각의 데이터는 data.xxx.json 파일로 관리한다.
	// 데이터파일에서 어떤 노티에 보내야하는지를 설정한다.(없으면 모두, 있으면 해당 노티로 보낸다, 지금은 1개)
	// 만약 사용자가 직접 요청해서 실행된 결과라면 요청한 노티로 보내야 한다.

	// 로또번호구하기 : 타 프로그램 실행 후 결과 받기
	// - 이미 실행된 프로그램? 아니면 새로 시작할것인가?
	// > 이미 실행된 프로그램 XXX
	//	 프로그램을 찾아서 메시지를 넘겨서 결과를 전송받아야 한다.
	// > 새로 시작
	//	 프로그램 시작시 메시지를 같이 넘기고 그 결과를 전송받아야 한다.
	//	 결과는 프로그램 콘솔에 찍힌걸 읽어올수 있으면 이걸 사용
	//	 안되면 결과파일을 지정해서 넘겨주고 종료시 이 결과파일을 분석

	return true
}

type NotiServer struct {
	NotiList []int
	// 알림 객체는 여러개 존재
	// 카카오톡, 텔레그램, 메일 등등으로 추가할 수 있도록 구성한다. 일단은 델레그램
	// 각 알림 객체는 고유의 ID를 가진다. 이건 json 파일에서 읽어올수 있도록 한다. 각 알림객체는 자신만의 데이터가 필요하기도 하다(계정정보 등)
}

func (n *NotiServer) Start() {
	// 알림서버 시작
}
