package mocks

import (
	"context"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/stretchr/testify/mock"
)

// MockTaskExecutor는 contract.TaskExecutor 인터페이스의 Mock 구현체입니다.
// 이 Mock은 Task 실행 및 취소 동작을 테스트하는 데 사용됩니다.
type MockTaskExecutor struct {
	mock.Mock
}

// Submit은 Task를 제출합니다.
func (m *MockTaskExecutor) Submit(ctx context.Context, req *contract.TaskSubmitRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

// Cancel 실행 중인 Task를 취소합니다.
func (m *MockTaskExecutor) Cancel(instanceID contract.TaskInstanceID) error {
	args := m.Called(instanceID)
	return args.Error(0)
}
