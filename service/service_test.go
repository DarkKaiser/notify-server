package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockService는 테스트용 Service 구현체입니다.
type mockService struct {
	runCalled bool
	runCount  int
}

func (m *mockService) Run(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) error {
	defer serviceStopWaiter.Done()
	m.runCalled = true
	m.runCount++

	// 서비스가 중지될 때까지 대기
	<-serviceStopCtx.Done()

	return nil
}

func TestServiceInterface(t *testing.T) {
	t.Run("Service 인터페이스 구현 테스트", func(t *testing.T) {
		var _ Service = (*mockService)(nil)
		// 컴파일 타임에 인터페이스 구현 여부 확인
	})
}

func TestMockService_Run(t *testing.T) {
	t.Run("서비스 실행 및 중지", func(t *testing.T) {
		mock := &mockService{}
		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}

		wg.Add(1)
		go mock.Run(ctx, wg)

		// 서비스가 시작될 때까지 대기
		time.Sleep(10 * time.Millisecond)

		assert.True(t, mock.runCalled, "Run 메서드가 호출되어야 합니다")
		assert.Equal(t, 1, mock.runCount, "Run 메서드가 1번 호출되어야 합니다")

		// 서비스 중지
		cancel()
		wg.Wait()
	})

	t.Run("여러 서비스 동시 실행", func(t *testing.T) {
		mock1 := &mockService{}
		mock2 := &mockService{}
		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}

		wg.Add(2)
		go mock1.Run(ctx, wg)
		go mock2.Run(ctx, wg)

		// 서비스가 시작될 때까지 대기
		time.Sleep(10 * time.Millisecond)

		assert.True(t, mock1.runCalled, "첫 번째 서비스가 실행되어야 합니다")
		assert.True(t, mock2.runCalled, "두 번째 서비스가 실행되어야 합니다")

		// 서비스 중지
		cancel()
		wg.Wait()
	})
}
