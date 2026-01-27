package mocks

import (
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/stretchr/testify/mock"
)

// MockTaskResultStorage는 contract.TaskResultStorage 인터페이스의 Mock 구현체입니다.
type MockTaskResultStorage struct {
	mock.Mock
}

// Save 결과를 저장하는 Mock 메서드입니다.
func (m *MockTaskResultStorage) Save(taskID contract.TaskID, commandID contract.TaskCommandID, v any) error {
	args := m.Called(taskID, commandID, v)
	return args.Error(0)
}

// Load 결과를 불러오는 Mock 메서드입니다.
func (m *MockTaskResultStorage) Load(taskID contract.TaskID, commandID contract.TaskCommandID, v any) error {
	args := m.Called(taskID, commandID, v)
	return args.Error(0)
}
