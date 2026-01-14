package mocks

import (
	"github.com/darkkaiser/notify-server/internal/service/task"
	"github.com/stretchr/testify/mock"
)

// MockExecutor는 task.Executor 인터페이스의 Mock 구현체입니다.
//
// 이 Mock은 Task 실행 및 취소 동작을 테스트하는 데 사용됩니다.
type MockExecutor struct {
	mock.Mock
}

// SubmitTask는 Task를 제출합니다.
func (m *MockExecutor) SubmitTask(req *task.SubmitRequest) error {
	args := m.Called(req)
	return args.Error(0)
}

// CancelTask는 실행 중인 Task를 취소합니다.
func (m *MockExecutor) CancelTask(instanceID task.InstanceID) error {
	args := m.Called(instanceID)
	return args.Error(0)
}
