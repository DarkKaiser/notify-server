package task

import (
	"errors"
	"fmt"
)

type TaskId int

type Tasker interface {
	Id() TaskId
	Run() bool
}

const (
	ALGANICMALL_TASK TaskId = iota
)

type Task struct {
	id TaskId
}

func (t *Task) Id() TaskId {
	return t.id
}

func (t *Task) SetId(id TaskId) {
	t.id = id
}

// singletone 구현
// https://blog.puppyloper.com/menus/Golang/articles/Golang%EA%B3%BC%20Singleton

type TaskManager struct {
	TaskList []Tasker
	// task를 싱핼시 해당 실행 task에 대한 id를 반환하며 이 id를 이용하여 언제든 작업을 쉬소할 수 있다.
}

// task를 실행시에 그때 객체를 생성하는건???
func (tm *TaskManager) Run(id TaskId) (Tasker, error) {
	switch id {
	case ALGANICMALL_TASK:
		tasker := NewAlganicMallTask(id)
		go tasker.Run()
		return tasker, nil
	default:
		return nil, errors.New(fmt.Sprintf("Payment method %d not recognized \n", m))
	}

	//for _, tasker := range tm.TaskList {
	//	if tasker.Id() == id {
	//		go tasker.Run()
	//		break
	//	}
	//}
}
