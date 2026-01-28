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

// =============================================================================
// 1. Constructor & Initialization Tests
// =============================================================================

func TestNewService(t *testing.T) {
	mockExe := &contractmocks.MockTaskExecutor{}
	mockSend := notificationmocks.NewMockNotificationSender(t)
	tasks := []config.TaskConfig{}

	tests := []struct {
		name        string
		submitter   contract.TaskSubmitter
		sender      contract.NotificationSender
		expectPanic string
	}{
		{
			name:      "Success_ValidArgs",
			submitter: mockExe,
			sender:    mockSend,
		},
		{
			name:        "Panic_NilTaskSubmitter",
			submitter:   nil,
			sender:      mockSend,
			expectPanic: "TaskSubmitter는 필수입니다",
		},
		{
			name:        "Panic_NilNotificationSender",
			submitter:   mockExe,
			sender:      nil,
			expectPanic: "NotificationSender는 필수입니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic != "" {
				assert.PanicsWithValue(t, tt.expectPanic, func() {
					NewService(tasks, tt.submitter, tt.sender)
				})
			} else {
				assert.NotPanics(t, func() {
					s := NewService(tasks, tt.submitter, tt.sender)
					assert.NotNil(t, s)
					assert.Equal(t, tt.submitter, s.taskSubmitter)
					assert.Equal(t, tt.sender, s.notificationSender)
				})
			}
		})
	}
}

// =============================================================================
// 2. Lifecycle & Error Handling Tests (Start/Stop)
// =============================================================================

func TestService_Start(t *testing.T) {
	mockExe := &contractmocks.MockTaskExecutor{}
	mockSend := notificationmocks.NewMockNotificationSender(t)

	tests := []struct {
		name          string
		setupService  func() *Service
		cancelContext bool // If true, cancels context before checking result (for graceful shutdown tests)
		expectError   error
		verify        func(t *testing.T, s *Service, wg *sync.WaitGroup)
	}{
		{
			name: "Success_NormalStart",
			setupService: func() *Service {
				return NewService(nil, mockExe, mockSend)
			},
			expectError: nil,
			verify: func(t *testing.T, s *Service, wg *sync.WaitGroup) {
				assert.True(t, s.running)
				assert.NotNil(t, s.cron)
				s.stop() // Cleanup
				assert.False(t, s.running)
			},
		},
		{
			name: "Success_Idempotency_DoubleStart",
			setupService: func() *Service {
				return NewService(nil, mockExe, mockSend)
			},
			expectError: nil,
			verify: func(t *testing.T, s *Service, wg *sync.WaitGroup) {
				// Second Start call
				wg.Add(1)
				err := s.Start(context.Background(), wg)
				assert.NoError(t, err)
				assert.True(t, s.running)

				// Ensure WaitGroup Done was called for the duplicate start
				// (It's hard to verify explicitly without channel, but no deadlock implies done)
				s.stop()
			},
		},
		{
			name: "Error_NilTaskSubmitter",
			setupService: func() *Service {
				s := NewService(nil, mockExe, mockSend)
				s.taskSubmitter = nil // Inject Fault
				return s
			},
			expectError: ErrTaskSubmitterNotInitialized,
			verify: func(t *testing.T, s *Service, wg *sync.WaitGroup) {
				// WaitGroup should be done immediately
				checkWaitGroupDone(t, wg)
			},
		},
		{
			name: "Error_NilNotificationSender",
			setupService: func() *Service {
				s := NewService(nil, mockExe, mockSend)
				s.notificationSender = nil // Inject Fault
				return s
			},
			expectError: ErrNotificationSenderNotInitialized,
			verify: func(t *testing.T, s *Service, wg *sync.WaitGroup) {
				checkWaitGroupDone(t, wg)
			},
		},
		{
			name: "Error_InvalidCronSpec",
			setupService: func() *Service {
				cfg := []config.TaskConfig{{
					ID: "T1", Commands: []config.CommandConfig{{
						ID: "C1", Scheduler: struct {
							Runnable bool   `json:"runnable"`
							TimeSpec string `json:"time_spec"`
						}{true, "INVALID-SPEC"},
					}},
				}}
				return NewService(cfg, mockExe, mockSend)
			},
			// Expects error containing proper message
			// (We check specific error type or string in execution loop)
			expectError: nil,
			verify: func(t *testing.T, s *Service, wg *sync.WaitGroup) {
				checkWaitGroupDone(t, wg)
				assert.Nil(t, s.cron)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.setupService()
			var wg sync.WaitGroup
			wg.Add(1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := s.Start(ctx, &wg)

			if tt.name == "Error_InvalidCronSpec" {
				assert.Error(t, err)
			} else if tt.expectError != nil {
				assert.ErrorIs(t, err, tt.expectError)
			} else {
				assert.NoError(t, err)
			}

			if tt.verify != nil {
				tt.verify(t, s, &wg)
			}
		})
	}
}

// =============================================================================
// 3. Execution Logic Tests (Cron Job -> Submit/Notify)
// =============================================================================

func TestService_Run_Execution(t *testing.T) {
	// Valid Cron Spec: "0 0 0 1 1 *" (Every Jan 1st) - won't run automatically
	validSpec := "0 0 0 1 1 *"

	tests := []struct {
		name       string
		taskConfig config.TaskConfig
		setupMocks func(*contractmocks.MockTaskExecutor, *notificationmocks.MockNotificationSender)
	}{
		{
			name: "Success_SubmitTask",
			taskConfig: config.TaskConfig{
				ID: "T1",
				Commands: []config.CommandConfig{{
					ID:                "C1",
					DefaultNotifierID: "N1",
					Scheduler: struct {
						Runnable bool   `json:"runnable"`
						TimeSpec string `json:"time_spec"`
					}{Runnable: true, TimeSpec: validSpec},
				}},
			},
			setupMocks: func(exe *contractmocks.MockTaskExecutor, send *notificationmocks.MockNotificationSender) {
				exe.On("Submit", mock.MatchedBy(func(ctx context.Context) bool {
					d, ok := ctx.Deadline()
					return ok && !d.IsZero() // Must have timeout
				}), mock.MatchedBy(func(req *contract.TaskSubmitRequest) bool {
					return req.TaskID == "T1" &&
						req.CommandID == "C1" &&
						req.NotifierID == "N1" &&
						req.RunBy == contract.TaskRunByScheduler
				})).Return(nil).Once()
			},
		},
		{
			name: "Failure_Submit_SendsNotification",
			taskConfig: config.TaskConfig{
				ID: "T2",
				Commands: []config.CommandConfig{{
					ID:                "C2",
					DefaultNotifierID: "N2",
					Scheduler: struct {
						Runnable bool   `json:"runnable"`
						TimeSpec string `json:"time_spec"`
					}{Runnable: true, TimeSpec: validSpec},
				}},
			},
			setupMocks: func(exe *contractmocks.MockTaskExecutor, send *notificationmocks.MockNotificationSender) {
				// 1. Submit fails
				exe.On("Submit", mock.Anything, mock.Anything).Return(assert.AnError).Once()

				// 2. Notification sent
				send.On("Notify", mock.MatchedBy(func(ctx context.Context) bool {
					d, ok := ctx.Deadline()
					return ok && !d.IsZero() // Must have timeout
				}), mock.MatchedBy(func(n contract.Notification) bool {
					return n.NotifierID == "N2" &&
						n.TaskID == "T2" &&
						n.ErrorOccurred &&
						strings.Contains(n.Message, "TaskSubmitter 실행 중 오류")
				})).Return(nil).Once()
			},
		},
		{
			name: "Skip_NotRunnable",
			taskConfig: config.TaskConfig{
				ID: "T3",
				Commands: []config.CommandConfig{{
					ID: "C3",
					Scheduler: struct {
						Runnable bool   `json:"runnable"`
						TimeSpec string `json:"time_spec"`
					}{Runnable: false, TimeSpec: validSpec},
				}},
			},
			setupMocks: func(exe *contractmocks.MockTaskExecutor, send *notificationmocks.MockNotificationSender) {
				// No calls expected
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExe := &contractmocks.MockTaskExecutor{}
			mockSend := notificationmocks.NewMockNotificationSender(t)

			if tt.setupMocks != nil {
				tt.setupMocks(mockExe, mockSend)
			}

			s := NewService([]config.TaskConfig{tt.taskConfig}, mockExe, mockSend)

			// Start service to register tasks
			var wg sync.WaitGroup
			wg.Add(1)
			err := s.Start(context.Background(), &wg)
			assert.NoError(t, err)
			defer s.stop()

			// Manually find and run the entry if it exists
			isRunnable := tt.taskConfig.Commands[0].Scheduler.Runnable
			if isRunnable {
				entries := s.cron.Entries()
				assert.Len(t, entries, 1)
				entries[0].Job.Run() // Trigger logic immediately
			} else {
				if s.cron != nil {
					assert.Empty(t, s.cron.Entries())
				}
			}

			mockExe.AssertExpectations(t)
			mockSend.AssertExpectations(t)
		})
	}
}

// =============================================================================
// 4. Concurrency & Closure Tests
// =============================================================================

func TestService_ClosureCapture(t *testing.T) {
	mockExe := &contractmocks.MockTaskExecutor{}
	mockSend := notificationmocks.NewMockNotificationSender(t)

	// Two tasks with same structure but different IDs
	// Two tasks with same structure but different IDs
	tasks := []config.TaskConfig{
		{ID: "T1", Commands: []config.CommandConfig{{ID: "C1", Scheduler: config.SchedulerConfig{Runnable: true, TimeSpec: "0 0 0 * * *"}}}},
		{ID: "T2", Commands: []config.CommandConfig{{ID: "C2", Scheduler: config.SchedulerConfig{Runnable: true, TimeSpec: "0 0 0 * * *"}}}},
	}

	// Verify T1 is called
	mockExe.On("Submit", mock.Anything, mock.MatchedBy(func(req *contract.TaskSubmitRequest) bool {
		return req.TaskID == "T1"
	})).Return(nil).Once()

	// Verify T2 is called
	mockExe.On("Submit", mock.Anything, mock.MatchedBy(func(req *contract.TaskSubmitRequest) bool {
		return req.TaskID == "T2"
	})).Return(nil).Once()

	s := NewService(tasks, mockExe, mockSend)
	var wg sync.WaitGroup
	wg.Add(1)
	s.Start(context.Background(), &wg)
	defer s.stop()

	// Run all registered jobs
	for _, e := range s.cron.Entries() {
		e.Job.Run()
	}

	mockExe.AssertExpectations(t)
}

func TestService_Concurrency(t *testing.T) {
	mockExe := &contractmocks.MockTaskExecutor{}
	mockSend := notificationmocks.NewMockNotificationSender(t)
	s := NewService(nil, mockExe, mockSend)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	concurrency := 20
	startCh := make(chan struct{})
	doneCh := make(chan struct{})

	// Massive parallel Start/Stop
	for i := 0; i < concurrency; i++ {
		go func() {
			<-startCh // Sync start

			// Try to start
			var wg sync.WaitGroup
			wg.Add(1)
			_ = s.Start(ctx, &wg) // Error ignored for race check

			// Short sleep to allow state change
			time.Sleep(time.Millisecond) // Don't use 0, yield needed

			// Try to stop
			s.stop()

			doneCh <- struct{}{}
		}()
	}

	close(startCh) // Go!
	for i := 0; i < concurrency; i++ {
		<-doneCh
	}

	// Final cleanup
	s.stop()
	// No panic implies pass (handled by -race flag)
}

// =============================================================================
// 5. Graceful Shutdown Tests
// =============================================================================

func TestService_GracefulShutdown(t *testing.T) {
	mockExe := &contractmocks.MockTaskExecutor{}
	mockSend := notificationmocks.NewMockNotificationSender(t)
	s := NewService(nil, mockExe, mockSend)

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)

	err := s.Start(ctx, &wg)
	assert.NoError(t, err)

	// Trigger shutdown via context
	done := make(chan struct{})
	go func() {
		cancel()  // 1. Cancel context
		wg.Wait() // 2. Wait for service to finish cleanup
		close(done)
	}()

	select {
	case <-done:
		assert.False(t, s.running)
		assert.Nil(t, s.cron)
	case <-time.After(time.Second):
		t.Fatal("Service did not shut down gracefully within 1s")
	}
}

// =============================================================================
// Helpers
// =============================================================================

func checkWaitGroupDone(t *testing.T, wg *sync.WaitGroup) {
	t.Helper()
	c := make(chan struct{})
	go func() {
		wg.Wait()
		close(c)
	}()
	select {
	case <-c:
		return
	case <-time.After(100 * time.Millisecond):
		t.Fatal("WaitGroup was not Done as expected")
	}
}
