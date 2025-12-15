package lotto

import (
	"github.com/stretchr/testify/mock"
)

// MockCommandProcess commandProcess 인터페이스의 Mock 구현체입니다.
// 테스트 시 실제 프로세스를 생성하지 않고 프로세스의 동작(대기, 종료, 출력)을 시뮬레이션합니다.
type MockCommandProcess struct {
	mock.Mock
}

func (m *MockCommandProcess) Wait() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockCommandProcess) Kill() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockCommandProcess) Output() string {
	args := m.Called()
	return args.String(0)
}

// MockCommandExecutor commandExecutor 인터페이스의 Mock 구현체입니다.
// 테스트 시 명령 실행 요청을 가로채고 미리 정의된 MockCommandProcess를 반환합니다.
type MockCommandExecutor struct {
	mock.Mock
}

func (m *MockCommandExecutor) StartCommand(name string, args ...string) (commandProcess, error) {
	// 가변 인자(variadic args) 처리를 위해 arguments를 펼쳐서 전달
	callArgs := make([]interface{}, 0, len(args)+1)
	callArgs = append(callArgs, name)
	for _, arg := range args {
		callArgs = append(callArgs, arg)
	}

	// Testify Mock에서는 ... argument가 Slice로 전달되므로 이를 그대로 넘길 수 없어
	// 편의상 인자는 무시하거나 필요한 경우 커스텀 매처를 사용해야 합니다.
	// 여기서는 간단히 이름과 인자를 묶어서 호출 기록을 남깁니다.
	ret := m.Called(name, args)

	var proc commandProcess
	if ret.Get(0) != nil {
		proc = ret.Get(0).(commandProcess)
	}
	return proc, ret.Error(1)
}
