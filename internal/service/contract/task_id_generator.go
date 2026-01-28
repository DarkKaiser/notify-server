package contract

// IDGenerator 작업 인스턴스의 고유 ID를 생성하는 인터페이스입니다.
type IDGenerator interface {
	// New 새로운 고유한 TaskInstanceID를 생성하여 반환합니다.
	//
	// 반환되는 ID는 시스템 전체에서 고유성이 보장되어야 하며,
	// 동시에 여러 고루틴에서 호출되어도 안전해야 합니다.
	//
	// 반환값:
	//   - TaskInstanceID: 고유한 작업 인스턴스 ID
	New() TaskInstanceID
}
