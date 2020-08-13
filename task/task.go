package task

// singletone 구현
// https://blog.puppyloper.com/menus/Golang/articles/Golang%EA%B3%BC%20Singleton

type TaskManager struct {
	TaskList []Task
	// task를 싱핼시 해당 실행 task에 대한 id를 반환하며 이 id를 이용하여 언제든 작업을 쉬소할 수 있다.
}

func (tm *TaskManager) Run(id int) {
	for _, task := range tm.TaskList {
		if task.Id() == id {
			go task.Run()
			break
		}
	}
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
