package testutil

import (
	"context"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/stretchr/testify/mock"
)

type MockTaskExecutor struct {
	mock.Mock
}

func (m *MockTaskExecutor) Submit(ctx context.Context, req *contract.TaskSubmitRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func (m *MockTaskExecutor) Cancel(instanceID contract.TaskInstanceID) error {
	args := m.Called(instanceID)
	return args.Error(0)
}
