package mocks

import (
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/stretchr/testify/mock"
)

// MockIDGenerator는 contract.IDGenerator 인터페이스의 Mock 구현체입니다.
// 테스트 환경에서 예측 가능한 ID를 반환하기 위해 사용됩니다.
type MockIDGenerator struct {
	mock.Mock
}

// New 지정된 Mock 동작에 따라 TaskInstanceID를 반환합니다.
func (m *MockIDGenerator) New() contract.TaskInstanceID {
	args := m.Called()
	return args.Get(0).(contract.TaskInstanceID)
}
