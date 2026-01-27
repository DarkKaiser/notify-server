package scheduler

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	contractmocks "github.com/darkkaiser/notify-server/internal/service/contract/mocks"
	notificationmocks "github.com/darkkaiser/notify-server/internal/service/notification/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestNewService 생성자 함수가 필수 의존성(nil 체크)을 올바르게 검증하는지 테스트합니다.
func TestNewService(t *testing.T) {
	mockExe := &contractmocks.MockTaskExecutor{}
	mockSend := notificationmocks.NewMockNotificationSender(t)
	tasks := []config.TaskConfig{}

	t.Run("Success_ValidArguments", func(t *testing.T) {
		assert.NotPanics(t, func() {
			s := NewService(tasks, mockExe, mockSend)
			assert.NotNil(t, s)
			assert.Equal(t, tasks, s.taskConfigs)
			assert.Equal(t, mockExe, s.taskSubmitter)
			assert.Equal(t, mockSend, s.notificationSender)
		})
	})

	t.Run("Panic_NilTaskSubmitter", func(t *testing.T) {
		assert.PanicsWithValue(t, "TaskSubmitter는 필수입니다", func() {
			NewService(tasks, nil, mockSend)
		})
	})

	t.Run("Panic_NilNotificationSender", func(t *testing.T) {
		assert.PanicsWithValue(t, "NotificationSender는 필수입니다", func() {
			NewService(tasks, mockExe, nil)
		})
	})
}

// TestScheduler_Lifecycle 스케줄러의 시작, 중지, 재시작 및 멱등성(Idempotency)을 테스트합니다.
func TestScheduler_Lifecycle(t *testing.T) {
	mockExe := &contractmocks.MockTaskExecutor{}
	mockSend := notificationmocks.NewMockNotificationSender(t)
	s := NewService(nil, mockExe, mockSend)

	t.Run("Start_And_Stop_Normal", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var wg sync.WaitGroup

		wg.Add(1)
		err := s.Start(ctx, &wg)
		assert.NoError(t, err)
		assert.True(t, s.running)
		assert.NotNil(t, s.cron)

		s.stop()
		assert.False(t, s.running)
	})

	t.Run("Idempotency_DuplicateStart", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var wg sync.WaitGroup

		wg.Add(1)
		err := s.Start(ctx, &wg)
		assert.NoError(t, err)

		// 이미 실행 중일 때 다시 Start 호출
		// WaitGroup.Add(1)은 호출자가 관리하므로, 내부에서는 이미 실행 중이면 Done()을 호출해야 함
		wg.Add(1)
		err = s.Start(ctx, &wg)
		assert.NoError(t, err) // 에러가 발생하지 않아야 함 (로그만 출력)
		assert.True(t, s.running)

		s.stop()
	})

	t.Run("Idempotency_DuplicateStop", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var wg sync.WaitGroup

		wg.Add(1)
		err := s.Start(ctx, &wg)
		assert.NoError(t, err)

		s.stop()
		assert.False(t, s.running)

		assert.NotPanics(t, func() {
			s.stop() // 이미 중지된 상태에서 다시 호출
		})
		assert.False(t, s.running)
	})
}

// TestScheduler_Start_Errors Start 메서드 실패 시 에러 반환 및 WaitGroup 관리 여부를 테스트합니다.
// 중요: 에러 발생 시 반드시 WaitGroup.Done()이 호출되어야 메인 고루틴이 데드락에 빠지지 않습니다.
func TestScheduler_Start_Errors(t *testing.T) {
	mockExe := &contractmocks.MockTaskExecutor{}
	mockSend := notificationmocks.NewMockNotificationSender(t)

	// 테스트 커버리지를 위해 의도적으로 내부 필드를 조작할 수 있는 구조체 복사본 사용 또는 Setter 사용이 필요하나
	// 현재 구조에서는 Start 내부에서 nil 체크를 다시 수행하므로, 생성자 검증과는 별개로
	// 런타임 중 필드 변경(가능성은 낮지만)에 대한 방어 로직을 테스트합니다.
	// 실제로는 NewService 생성자를 통해 생성하면 nil일 수 없지만,
	// Start 메서드 자체의 방어 로직을 테스트하기 위해 reflection 등으로 nil 주입이 필요할 수 있습니다.
	// 여기서는 Start 메서드의 '방어 로직'이 존재하므로, 이를 테스트하기 위해 s.taskSubmitter를 강제로 nil로 만듭니다.

	t.Run("Error_TaskSubmitterNil_CheckWaitGroup", func(t *testing.T) {
		s := NewService(nil, mockExe, mockSend)
		s.taskSubmitter = nil // 강제 주입

		ctx := context.Background()
		var wg sync.WaitGroup
		wg.Add(1)

		err := s.Start(ctx, &wg)
		assert.ErrorIs(t, err, ErrTaskSubmitterNotInitialized)

		// WaitGroup이 제대로 감소되었는지 확인 (데드락 방지)
		checkWaitGroupDone(t, &wg)
	})

	t.Run("Error_NotificationSenderNil_CheckWaitGroup", func(t *testing.T) {
		s := NewService(nil, mockExe, mockSend)
		s.notificationSender = nil // 강제 주입

		ctx := context.Background()
		var wg sync.WaitGroup
		wg.Add(1)

		err := s.Start(ctx, &wg)
		assert.ErrorIs(t, err, ErrNotificationSenderNotInitialized)

		checkWaitGroupDone(t, &wg)
	})

	t.Run("Error_InvalidCronSpec_CheckWaitGroup", func(t *testing.T) {
		cfg := []config.TaskConfig{
			{
				ID: "InvalidTask",
				Commands: []config.CommandConfig{{
					ID: "C1",
					Scheduler: struct {
						Runnable bool   `json:"runnable"`
						TimeSpec string `json:"time_spec"`
					}{Runnable: true, TimeSpec: "invalid-cron-spec"}, // 유효하지 않은 스펙
				}},
			},
		}
		s := NewService(cfg, mockExe, mockSend)

		ctx := context.Background()
		var wg sync.WaitGroup
		wg.Add(1)

		err := s.Start(ctx, &wg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "스케줄 등록 실패")

		checkWaitGroupDone(t, &wg)
	})
}

// TestScheduler_Concurrency Start/Stop을 여러 고루틴에서 동시에 호출하여 경쟁 상태(Race Condition)를 테스트합니다.
func TestScheduler_Concurrency(t *testing.T) {
	mockExe := &contractmocks.MockTaskExecutor{}
	mockSend := notificationmocks.NewMockNotificationSender(t)
	s := NewService(nil, mockExe, mockSend)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 10개의 고루틴이 동시에 Start/Stop을 시도
	concurrency := 10
	done := make(chan bool)

	for i := 0; i < concurrency; i++ {
		go func() {
			var wg sync.WaitGroup
			wg.Add(1)
			_ = s.Start(ctx, &wg) // 에러 무시 (중복 실행 등 가능)

			// 짧은 대기 후 Stop
			time.Sleep(1 * time.Millisecond)
			s.stop()

			// Start가 성공했다면 wg.Done()이 호출되지 않았을 수도 있으므로 (Stop이 호출 안되는 타이밍 이슈)
			// 여기서는 Race detector가 panic을 일으키지 않는지 확인하는 것이 주 목적입니다.
			// 정확한 WG 관리는 Start/Stop의 쌍이 맞아야 하지만, 무작위 호출 테스트이므로
			// WG 완료 여부보다는 메모리 접근 충돌 여부를 봅니다.
			// 다만 테스트 종료를 위해 여기서 Cleanup은 하지 않습니다.
			done <- true
		}()
	}

	for i := 0; i < concurrency; i++ {
		<-done
	}

	// 최종 cleanup
	s.stop()
}

// TestScheduler_Run_Execution 실제 스케줄링 로직(등록 -> 실행 -> 알림)을 검증합니다.
func TestScheduler_Run_Execution(t *testing.T) {
	// 정상적인 스케줄 설정 (초 분 시 일 월 요일)
	validSchedule := "0 0 0 1 1 *"

	tests := []struct {
		name        string
		taskConfig  config.TaskConfig
		setupMocks  func(*contractmocks.MockTaskExecutor, *notificationmocks.MockNotificationSender)
		expectError bool
	}{
		{
			name: "Success_SubmitTask",
			taskConfig: config.TaskConfig{
				ID: "T1",
				Commands: []config.CommandConfig{{
					ID: "C1",
					Scheduler: struct {
						Runnable bool   `json:"runnable"`
						TimeSpec string `json:"time_spec"`
					}{Runnable: true, TimeSpec: validSchedule},
					DefaultNotifierID: "N1",
				}},
			},
			setupMocks: func(exe *contractmocks.MockTaskExecutor, send *notificationmocks.MockNotificationSender) {
				// Submit 호출 시 Deadline이 설정되어 있는지 검증 (5초)
				exe.On("Submit", mock.MatchedBy(func(ctx context.Context) bool {
					deadline, ok := ctx.Deadline()
					return ok && !deadline.IsZero()
				}), mock.MatchedBy(func(req *contract.TaskSubmitRequest) bool {
					return req.TaskID == "T1" && req.CommandID == "C1" &&
						req.NotifierID == "N1" && req.RunBy == contract.TaskRunByScheduler
				})).Return(nil).Once()
			},
			expectError: false,
		},
		{
			name: "Failure_SubmitTask_ShouldNotify",
			taskConfig: config.TaskConfig{
				ID: "T2",
				Commands: []config.CommandConfig{{
					ID: "C2",
					Scheduler: struct {
						Runnable bool   `json:"runnable"`
						TimeSpec string `json:"time_spec"`
					}{Runnable: true, TimeSpec: validSchedule},
					DefaultNotifierID: "N2",
				}},
			},
			setupMocks: func(exe *contractmocks.MockTaskExecutor, send *notificationmocks.MockNotificationSender) {
				// Submit 실패
				exe.On("Submit", mock.Anything, mock.Anything).Return(assert.AnError).Once()

				// 알림 전송 호출 검증 (Deadline 1초 포함)
				send.On("Notify", mock.MatchedBy(func(ctx context.Context) bool {
					deadline, ok := ctx.Deadline()
					return ok && !deadline.IsZero()
				}), mock.MatchedBy(func(n contract.Notification) bool {
					return n.NotifierID == "N2" && n.ErrorOccurred &&
						strings.Contains(n.Message, "TaskSubmitter 실행 중 오류")
				})).Return(nil).Once()
			},
			expectError: true, // 로직 상 에러는 처리되지만 테스트 시나리오 상 '에러 상황'임
		},
		{
			name: "Skip_IfNotRunnable",
			taskConfig: config.TaskConfig{
				ID: "T3",
				Commands: []config.CommandConfig{{
					ID: "C3",
					Scheduler: struct {
						Runnable bool   `json:"runnable"`
						TimeSpec string `json:"time_spec"`
					}{Runnable: false, TimeSpec: validSchedule},
				}},
			},
			setupMocks: func(exe *contractmocks.MockTaskExecutor, send *notificationmocks.MockNotificationSender) {
				// 아무것도 호출되지 않아야 함
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExe := &contractmocks.MockTaskExecutor{}
			mockSend := notificationmocks.NewMockNotificationSender(t)

			if tt.setupMocks != nil {
				tt.setupMocks(mockExe, mockSend)
			}

			cfg := []config.TaskConfig{tt.taskConfig}
			s := NewService(cfg, mockExe, mockSend)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var wg sync.WaitGroup

			wg.Add(1)
			err := s.Start(ctx, &wg)
			assert.NoError(t, err)

			// Runnable인 경우에만 작업이 등록됨
			isRunnable := tt.taskConfig.Commands[0].Scheduler.Runnable
			if isRunnable {
				entries := s.cron.Entries()
				assert.Len(t, entries, 1, "스케줄이 등록되어야 합니다")

				// 등록된 작업을 수동으로 즉시 실행 (시간 대기 없이 로직 검증)
				entries[0].Job.Run()
			} else if s.cron != nil {
				assert.Empty(t, s.cron.Entries(), "Runnable이 false면 스케줄이 등록되지 않아야 합니다")
			}

			mockExe.AssertExpectations(t)
			mockSend.AssertExpectations(t)

			s.stop()
		})
	}
}

// TestScheduler_ClosureCapture_LoopVariable 여러 작업 등록 시 for-loop 변수 캡처 문제가 없는지 검증합니다.
// Go 1.22 이전 버전에서는 for 루프 변수 공유 문제가 있었으므로 명시적 검증이 필요합니다.
func TestScheduler_ClosureCapture_LoopVariable(t *testing.T) {
	mockExe := &contractmocks.MockTaskExecutor{}
	mockSend := notificationmocks.NewMockNotificationSender(t)

	tasks := []config.TaskConfig{
		{ID: "T1", Commands: []config.CommandConfig{{ID: "C1", Scheduler: struct {
			Runnable bool   `json:"runnable"`
			TimeSpec string `json:"time_spec"`
		}{true, "0 0 0 1 1 *"}, DefaultNotifierID: "N"}}},
		{ID: "T2", Commands: []config.CommandConfig{{ID: "C2", Scheduler: struct {
			Runnable bool   `json:"runnable"`
			TimeSpec string `json:"time_spec"`
		}{true, "0 0 0 1 1 *"}, DefaultNotifierID: "N"}}},
	}

	// 각기 다른 ID로 호출되는지 확인
	mockExe.On("Submit", mock.Anything, mock.MatchedBy(func(req *contract.TaskSubmitRequest) bool {
		return req.TaskID == "T1" && req.CommandID == "C1"
	})).Return(nil).Once()
	mockExe.On("Submit", mock.Anything, mock.MatchedBy(func(req *contract.TaskSubmitRequest) bool {
		return req.TaskID == "T2" && req.CommandID == "C2"
	})).Return(nil).Once()

	s := NewService(tasks, mockExe, mockSend)
	var wg sync.WaitGroup
	wg.Add(1)
	_ = s.Start(context.Background(), &wg)
	defer s.stop()

	assert.Equal(t, 2, len(s.cron.Entries()))

	// 모든 작업을 실행해봅니다.
	for _, e := range s.cron.Entries() {
		e.Job.Run()
	}

	mockExe.AssertExpectations(t)
}

// TestScheduler_GracefulShutdown 서비스 컨텍스트 취소 시 정상 종료되는지 확인합니다.
func TestScheduler_GracefulShutdown(t *testing.T) {
	mockExe := &contractmocks.MockTaskExecutor{}
	mockSend := notificationmocks.NewMockNotificationSender(t)
	s := NewService(nil, mockExe, mockSend)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	wg.Add(1)
	err := s.Start(ctx, &wg)
	assert.NoError(t, err)

	// 비동기로 종료 신호 전송
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	// WaitGroup이 Done 될 때까지 대기 (타임아웃 설정)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 정상 종료됨
		assert.False(t, s.running, "종료 후 running 상태는 false여야 합니다")
	case <-time.After(1 * time.Second):
		t.Fatal("서비스가 제한 시간 내에 종료되지 않았습니다 (Deadlock 가능성)")
	}
}

// helper function: Check WaitGroup Done
func checkWaitGroupDone(t *testing.T, wg *sync.WaitGroup) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return
	case <-time.After(100 * time.Millisecond):
		t.Fatal("WaitGroup.Done()이 호출되지 않았습니다")
	}
}
