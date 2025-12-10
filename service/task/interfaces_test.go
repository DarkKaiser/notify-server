package task

import (
	"testing"
)

// Compile-time checks to ensure types implement the interfaces.
var (
	// MockNotificationSender가 NotificationSender 인터페이스를 구현하는지 확인
	_ NotificationSender = (*MockNotificationSender)(nil)
)

func TestInterfaces(t *testing.T) {
	// 인터페이스 정의 자체를 테스트할 수는 없지만,
	// 주요 구현체들이 인터페이스를 준수하는지는 컴파일 타임 검사를 통해 보장합니다.
	// 위 var 블록에서 컴파일 에러가 발생하지 않는다면 테스트는 성공한 것으로 간주합니다.
}
