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
	started   chan struct{} // 서비스 시작 신호 채널
	mu        sync.Mutex
}

func newMockService() *mockService {
	return &mockService{
		started: make(chan struct{}),
	}
}

func (m *mockService) Run(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) error {
	defer serviceStopWaiter.Done()

	m.mu.Lock()
	m.runCalled = true
	m.runCount++
	m.mu.Unlock()

	// 시작 신호 전송 (채널이 닫혀있지 않은 경우에만)
	select {
	case <-m.started:
	default:
		close(m.started)
	}

	// 서비스가 중지될 때까지 대기
	<-serviceStopCtx.Done()

	return nil
}

func TestServiceInterface(t *testing.T) {
	t.Run("Service 인터페이스 구현 테스트", func(t *testing.T) {
		var _ Service = (*mockService)(nil)
	})
}

func TestMockService_Run(t *testing.T) {
	t.Run("서비스 실행 및 중지", func(t *testing.T) {
		mock := newMockService()
		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}

		wg.Add(1)
		go mock.Run(ctx, wg)

		// 서비스가 시작될 때까지 대기 (채널 사용)
		select {
		case <-mock.started:
			// 정상 시작
		case <-time.After(1 * time.Second):
			t.Fatal("서비스가 시간 내에 시작되지 않았습니다")
		}

		mock.mu.Lock()
		assert.True(t, mock.runCalled, "Run 메서드가 호출되어야 합니다")
		assert.Equal(t, 1, mock.runCount, "Run 메서드가 1번 호출되어야 합니다")
		mock.mu.Unlock()

		// 서비스 중지
		cancel()

		// WaitGroup이 완료될 때까지 대기위해 별도 고루틴 사용 혹은 done 채널 사용
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// 정상 종료
		case <-time.After(1 * time.Second):
			t.Fatal("서비스가 시간 내에 종료되지 않았습니다")
		}
	})

	t.Run("여러 서비스 동시 실행", func(t *testing.T) {
		mock1 := newMockService()
		mock2 := newMockService()
		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}

		wg.Add(2)
		go mock1.Run(ctx, wg)
		go mock2.Run(ctx, wg)

		// 모든 서비스가 시작될 때까지 대기
		timeout := time.After(1 * time.Second)
		for _, m := range []*mockService{mock1, mock2} {
			select {
			case <-m.started:
				m.mu.Lock()
				assert.True(t, m.runCalled)
				m.mu.Unlock()
			case <-timeout:
				t.Fatal("서비스가 시간 내에 시작되지 않았습니다")
			}
		}

		// 서비스 중지
		cancel()

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// 정상 종료
		case <-time.After(1 * time.Second):
			t.Fatal("서비스가 시간 내에 종료되지 않았습니다")
		}
	})
}
