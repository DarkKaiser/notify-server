package notification

import (
	"testing"
)

// -- Interface Compliance Checks --
// 이 블록은 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
// 만약 구현체가 인터페이스를 충족하지 못하면, 컴파일 에러가 발생합니다.
var (
	// Sender Implementation
	_ Sender = (*Service)(nil)

	// NotifierHandler Implementation
	_ NotifierHandler = (*telegramNotifier)(nil)
	_ NotifierHandler = (*mockNotifierHandler)(nil) // Test Mock도 인터페이스를 준수해야 함

	// NotifierFactory Implementation
	_ NotifierFactory = (*DefaultNotifierFactory)(nil)
	_ NotifierFactory = (*mockNotifierFactory)(nil) // Test Mock
)

func TestInterfaces(t *testing.T) {
	// 이 함수는 빈 상태로 유지됩니다.
	// 목적은 위의 'var' 블록을 통해 컴파일 타임 검증을 수행하는 것이며,
	// 'go test' 실행 시 해당 파일이 컴파일됨으로써 검증이 완료됩니다.
}
