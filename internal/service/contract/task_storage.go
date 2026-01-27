package contract

// TaskResultStore Task 실행 결과(스냅샷)를 영속적으로 저장하고 불러오는 저장소 인터페이스입니다.
//
// 이 인터페이스는 Task가 실행될 때마다 생성되는 중간 결과 데이터를 저장하여,
// 다음 실행 시 이전 상태를 기반으로 변경 사항을 감지하거나 증분 처리를 수행할 수 있도록 합니다.
type TaskResultStore interface {
	// Save Task 실행 결과를 저장합니다.
	//
	// 동일한 taskID와 commandID 조합으로 Save를 호출하면 기존 데이터를 덮어씁니다.
	Save(taskID TaskID, commandID TaskCommandID, v any) error

	// Load 저장된 Task 실행 결과를 불러옵니다.
	//
	// 저장된 데이터가 없는 경우 에러를 반환하지 않고 v를 변경하지 않습니다.
	// 이는 최초 실행 시나리오를 간단하게 처리하기 위함입니다.
	Load(taskID TaskID, commandID TaskCommandID, v any) error
}
