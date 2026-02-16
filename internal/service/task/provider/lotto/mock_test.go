package lotto

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockCommandProcess commandProcess 인터페이스의 Mock 구현체입니다.
// 실제 프로세스를 실행하지 않고, 미리 정의된 동작을 시뮬레이션합니다.
type MockCommandProcess struct {
	mock.Mock
}

// Wait 프로세스가 종료될 때까지 대기하는 동작을 모킹합니다.
func (m *MockCommandProcess) Wait() error {
	args := m.Called()
	return args.Error(0)
}

// Kill 프로세스를 강제로 종료하는 동작을 모킹합니다.
func (m *MockCommandProcess) Kill() error {
	args := m.Called()
	return args.Error(0)
}

// Stdout 프로세스의 표준 출력(Stdout) 결과를 반환하는 동작을 모킹합니다.
func (m *MockCommandProcess) Stdout() string {
	args := m.Called()
	return args.String(0)
}

// Stderr 프로세스의 표준 에러(Stderr) 결과를 반환하는 동작을 모킹합니다.
func (m *MockCommandProcess) Stderr() string {
	args := m.Called()
	return args.String(0)
}

// MockCommandExecutor commandExecutor 인터페이스의 Mock 구현체입니다.
// 실제 시스템 명령어를 실행하지 않고, Mock 프로세스 객체를 반환합니다.
type MockCommandExecutor struct {
	mock.Mock
}

// Start 명령어를 실행하는 동작을 모킹합니다.
// 입력된 인자들을 그대로 Mock 객체에 전달하여 호출 여부를 검증할 수 있게 합니다.
func (m *MockCommandExecutor) Start(ctx context.Context, name string, args ...string) (commandProcess, error) {
	// 가변 인자(args)를 interface{} 슬라이스로 변환하여 m.Called에 전달합니다.
	// 이렇게 해야 Mock의 Called 메서드가 가변 인자들을 개별 인자로 인식하지 않고,
	// 하나의 슬라이스 인자로 인식하거나, 혹은 의도대로 전달할 수 있습니다.
	// 여기서는 (ctx, name, []string) 형태의 3개 인자로 호출된 것으로 기록합니다.
	ret := m.Called(ctx, name, args)

	// 첫 번째 반환값: commandProcess (또는 nil)
	var proc commandProcess
	if ret.Get(0) != nil {
		// 타입 단언(Type Assertion)을 안전하게 수행하거나,
		// 테스트 코드에서 nil을 리턴하도록 설정했을 경우를 대비합니다.
		proc = ret.Get(0).(commandProcess)
	}

	// 두 번째 반환값: error
	return proc, ret.Error(1)
}
