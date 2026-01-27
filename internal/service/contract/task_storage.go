package contract

// TaskResultStorage Task 실행 결과를 저장하고 불러오는 저장소 인터페이스입니다.
type TaskResultStorage interface {
	// Save Task 실행 결과를 저장합니다.
	Save(taskID TaskID, commandID TaskCommandID, v any) error

	// Load 저장된 Task 결과를 불러옵니다.
	Load(taskID TaskID, commandID TaskCommandID, v any) error
}
