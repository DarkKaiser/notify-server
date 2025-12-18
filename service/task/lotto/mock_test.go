package lotto

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockCommandProcess commandProcess 인터페이스의 Mock 구현체입니다.
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

func (m *MockCommandProcess) Stdout() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockCommandProcess) Stderr() string {
	args := m.Called()
	return args.String(0)
}

// MockCommandExecutor commandExecutor 인터페이스의 Mock 구현체입니다.
type MockCommandExecutor struct {
	mock.Mock
}

func (m *MockCommandExecutor) StartCommand(ctx context.Context, name string, args ...string) (commandProcess, error) {
	// 가변 인자(variadic args) 처리를 위해 arguments를 펼쳐서 전달
	callArgs := make([]interface{}, 0, len(args)+2)
	callArgs = append(callArgs, ctx, name)
	for _, arg := range args {
		callArgs = append(callArgs, arg)
	}

	ret := m.Called(ctx, name, args)

	var proc commandProcess
	if ret.Get(0) != nil {
		proc = ret.Get(0).(commandProcess)
	}
	return proc, ret.Error(1)
}
