package task

import (
	"errors"
	"fmt"
	"github.com/darkkaiser/notify-server/global"
)

type TaskId string
type CommandId string

type Tasker interface {
	Id() TaskId
	CommandId() CommandId

	Run() bool
}

const (
	ALGANICMALL_TASK TaskId = "ALGANICMALL"
)

const (
	// 엘가닉몰
	ALGANICMALL_COMMAND_CRAWING CommandId = "crawing"
)

type Task struct {
	id        TaskId
	commandId CommandId
}

func (t *Task) Id() TaskId {
	return t.id
}

func (t *Task) CommandId() CommandId {
	return t.commandId
}

// singletone 구현
// https://blog.puppyloper.com/menus/Golang/articles/Golang%EA%B3%BC%20Singleton

type TaskManager struct {
	TaskList  []Tasker
	appConfig *global.AppConfig

	// task를 싱핼시 해당 실행 task에 대한 id를 반환하며 이 id를 이용하여 언제든 작업을 쉬소할 수 있다.
}

func (tm *TaskManager) Init(appConfig *global.AppConfig) {
	// 초기화
	// 환경설정값이 올바른지 확인
	tm.appConfig = appConfig

	// @@@@@
	// 채널을 만들고.. 이걸 cron이나 notifier에서 호출???

	go func() {

	}()
}

func (tm *TaskManager) Run(id TaskId, command CommandId) (Tasker, error) {
	switch id {
	case ALGANICMALL_TASK:
		tasker := NewAlganicMallTask(command)
		go tasker.Run()
		return tasker, nil
	}

	return nil, errors.New(fmt.Sprintf("Payment method %d not recognized \n", 1))
}
