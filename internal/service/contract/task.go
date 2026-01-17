package contract

// TaskSubmitter 작업을 등록하기 위한 인터페이스입니다.
type TaskSubmitter interface {
	// Submit 작업을 실행 큐에 등록합니다.
	Submit(req *TaskSubmitRequest) error
}

// TaskCanceler 실행 중인 작업을 취소하기 위한 인터페이스입니다.
type TaskCanceler interface {
	// Cancel 특정 작업 인스턴스의 실행을 취소합니다.
	Cancel(instanceID TaskInstanceID) error
}

// TaskExecutor 작업 등록 및 취소 기능을 통합한 인터페이스입니다.
type TaskExecutor interface {
	TaskSubmitter
	TaskCanceler
}
