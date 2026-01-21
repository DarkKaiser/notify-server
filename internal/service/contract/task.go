package contract

import "context"

// TaskSubmitter 새로운 작업을 시스템에 등록하는 인터페이스입니다.
type TaskSubmitter interface {
	// Submit 작업을 실행 요청 대기열(Queue)에 등록합니다.
	Submit(ctx context.Context, req *TaskSubmitRequest) error
}

// TaskCanceler 실행 중이거나 대기 중인 작업을 취소하는 인터페이스입니다.
//
// 사용자 요청이나 시스템 종료 등의 사유로 작업 실행을 중단해야 할 때 사용됩니다.
type TaskCanceler interface {
	// Cancel 특정 작업 인스턴스의 실행을 중단 요청합니다.
	Cancel(instanceID TaskInstanceID) error
}

// TaskExecutor 작업의 등록과 취소 기능을 통합한 인터페이스입니다.
type TaskExecutor interface {
	TaskSubmitter
	TaskCanceler
}
