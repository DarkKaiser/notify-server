package scheduler

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	contractmocks "github.com/darkkaiser/notify-server/internal/service/contract/mocks"
	notificationmocks "github.com/darkkaiser/notify-server/internal/service/notification/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestNewService_Validation 생성자가 필수 의존성을 검증하는지 테스트합니다.
func TestNewService_Validation(t *testing.T) {
	mockExe := &contractmocks.MockTaskExecutor{}
	mockSend := notificationmocks.NewMockNotificationSender(t)
	tasks := []config.TaskConfig{}

	t.Run("Success", func(t *testing.T) {
		assert.NotPanics(t, func() {
			NewService(tasks, mockExe, mockSend)
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

// TestScheduler_Start_Errors Start 메서드의 에러 반환 경로를 테스트합니다.
func TestScheduler_Start_Errors(t *testing.T) {
	mockExe := &contractmocks.MockTaskExecutor{}
	mockSend := notificationmocks.NewMockNotificationSender(t)
	s := NewService(nil, mockExe, mockSend)

	ctx := context.Background()
	wg := &sync.WaitGroup{}

	t.Run("Error_TaskSubmitterNil", func(t *testing.T) {
		s.taskSubmitter = nil
		s.notificationSender = mockSend
		wg.Add(1)
		err := s.Start(ctx, wg)
		assert.ErrorIs(t, err, ErrTaskSubmitterNotInitialized)
		s.taskSubmitter = mockExe
	})

	t.Run("Error_NotificationSenderNil", func(t *testing.T) {
		s.notificationSender = nil
		s.taskSubmitter = mockExe
		wg.Add(1)
		err := s.Start(ctx, wg)
		assert.ErrorIs(t, err, ErrNotificationSenderNotInitialized)
		s.notificationSender = mockSend
	})
}

// TestScheduler_Lifecycle 스케줄러의 시작/중지 및 중복 호출에 대한 생명주기를 테스트합니다.
func TestScheduler_Lifecycle(t *testing.T) {
	tests := []struct {
		name         string
		initialState func(*Scheduler)
		action       func(*Scheduler, context.Context, *sync.WaitGroup)
		verify       func(*testing.T, *Scheduler)
		doubleAction bool
	}{
		{
			name: "Start Scheduler",
			action: func(s *Scheduler, ctx context.Context, wg *sync.WaitGroup) {
				wg.Add(1)
				s.Start(ctx, wg)
			},
			verify: func(t *testing.T, s *Scheduler) {
				assert.True(t, s.running)
				assert.NotNil(t, s.cron)
			},
		},
		{
			name: "Stop Scheduler Safely",
			initialState: func(s *Scheduler) {
				s.running = true
				s.cron = nil
			},
			action: func(s *Scheduler, ctx context.Context, wg *sync.WaitGroup) {
				s.stop()
			},
			verify: func(t *testing.T, s *Scheduler) {
				assert.False(t, s.running)
			},
		},
		{
			name: "Restart Scheduler",
			action: func(s *Scheduler, ctx context.Context, wg *sync.WaitGroup) {
				wg.Add(1)
				s.Start(ctx, wg)
				s.stop()
				wg.Add(1)
				s.Start(ctx, wg)
			},
			verify: func(t *testing.T, s *Scheduler) {
				assert.True(t, s.running)
			},
		},
		{
			name:         "Duplicate Start",
			doubleAction: true,
			action: func(s *Scheduler, ctx context.Context, wg *sync.WaitGroup) {
				wg.Add(1)
				s.Start(ctx, wg)
			},
			verify: func(t *testing.T, s *Scheduler) {
				assert.True(t, s.running)
			},
		},
		{
			name:         "Duplicate Stop",
			doubleAction: true,
			action: func(s *Scheduler, ctx context.Context, wg *sync.WaitGroup) {
				wg.Add(1)
				s.Start(ctx, wg)
				s.stop()
			},
			verify: func(t *testing.T, s *Scheduler) {
				assert.False(t, s.running)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExe := &contractmocks.MockTaskExecutor{}
			mockSend := notificationmocks.NewMockNotificationSender(t)

			// 유효한 설정 (6필드 형식이므로 Seconds 포함)
			defaultTask := []config.TaskConfig{
				{
					ID: "DummyTask",
					Commands: []config.CommandConfig{
						{
							ID: "DummyCmd",
							Scheduler: struct {
								Runnable bool   `json:"runnable"`
								TimeSpec string `json:"time_spec"`
							}{Runnable: true, TimeSpec: "0 0 0 1 1 *"}, // 0초 0분 0시 1일 1월 *요일
							DefaultNotifierID: "N1",
						},
					},
				},
			}
			cfg := &config.AppConfig{Tasks: defaultTask}
			s := NewService(cfg.Tasks, mockExe, mockSend)

			if tt.initialState != nil {
				tt.initialState(s)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var wg sync.WaitGroup

			defer s.stop()

			tt.action(s, ctx, &wg)
			if tt.doubleAction {
				tt.action(s, ctx, &wg)
			}

			if tt.verify != nil {
				tt.verify(t, s)
			}
		})
	}
}

// TestScheduler_TaskRegistration Runnable 플래그에 따른 작업 등록 여부를 테스트합니다.
func TestScheduler_TaskRegistration(t *testing.T) {
	tests := []struct {
		name          string
		tasks         []config.TaskConfig
		expectedCount int
	}{
		{
			name: "Runnable Task",
			tasks: []config.TaskConfig{
				{
					ID: "T1",
					Commands: []config.CommandConfig{
						{
							ID: "C1",
							Scheduler: struct {
								Runnable bool   `json:"runnable"`
								TimeSpec string `json:"time_spec"`
							}{Runnable: true, TimeSpec: "0 0 0 1 1 *"},
						},
					},
				},
			},
			expectedCount: 1,
		},
		{
			name: "Non-Runnable Task",
			tasks: []config.TaskConfig{
				{
					ID: "T1",
					Commands: []config.CommandConfig{
						{
							ID: "C1",
							Scheduler: struct {
								Runnable bool   `json:"runnable"`
								TimeSpec string `json:"time_spec"`
							}{Runnable: false, TimeSpec: "0 0 0 1 1 *"},
						},
					},
				},
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExe := &contractmocks.MockTaskExecutor{}
			mockSend := notificationmocks.NewMockNotificationSender(t)
			cfg := &config.AppConfig{Tasks: tt.tasks}
			s := NewService(cfg.Tasks, mockExe, mockSend)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var wg sync.WaitGroup

			wg.Add(1)
			err := s.Start(ctx, &wg)
			defer s.stop()

			assert.NoError(t, err)
			if tt.expectedCount > 0 {
				assert.NotNil(t, s.cron)
				assert.Equal(t, tt.expectedCount, len(s.cron.Entries()))
			} else {
				if s.cron != nil {
					assert.Equal(t, 0, len(s.cron.Entries()))
				}
			}
		})
	}
}

// TestScheduler_Execution_ManualTrigger Cron의 Job.Run 메서드를 수동으로 호출하여
// 작업 실행 및 알림 전송 로직을 결정적(Deterministic)으로 테스트합니다.
func TestScheduler_Execution_ManualTrigger(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(*contractmocks.MockTaskExecutor, *notificationmocks.MockNotificationSender)
		wantErrLogs bool
	}{
		{
			name: "Success_Submission",
			setupMock: func(exe *contractmocks.MockTaskExecutor, send *notificationmocks.MockNotificationSender) {
				// Submit 호출 시 Deadline이 설정되어 있는지 검증 (taskSubmitTimeout=5s)
				exe.On("Submit", mock.MatchedBy(func(ctx context.Context) bool {
					deadline, ok := ctx.Deadline()
					return ok && !deadline.IsZero()
				}), mock.MatchedBy(func(req *contract.TaskSubmitRequest) bool {
					return req.TaskID == "T1" && req.CommandID == "C1" && req.RunBy == contract.TaskRunByScheduler
				})).Return(nil).Once()
			},
			wantErrLogs: false,
		},
		{
			name: "Fail_Submission_SendNotification",
			setupMock: func(exe *contractmocks.MockTaskExecutor, send *notificationmocks.MockNotificationSender) {
				// Submit 실패 설정
				exe.On("Submit", mock.Anything, mock.Anything).Return(assert.AnError).Once()

				// 알림 전송 검증
				// Notify 호출 시 Deadline이 설정되어 있는지 검증 (1초 타임아웃)
				send.On("Notify", mock.MatchedBy(func(ctx context.Context) bool {
					deadline, ok := ctx.Deadline()
					return ok && !deadline.IsZero()
				}), mock.MatchedBy(func(n contract.Notification) bool {
					return n.ErrorOccurred && strings.Contains(n.Message, "TaskSubmitter 실행 중 오류")
				})).Return(nil).Once()
			},
			wantErrLogs: true,
		},
		{
			name: "Fail_Submission_And_Fail_Notification",
			setupMock: func(exe *contractmocks.MockTaskExecutor, send *notificationmocks.MockNotificationSender) {
				// Submit 실패
				exe.On("Submit", mock.Anything, mock.Anything).Return(assert.AnError).Once()

				// 알림 전송도 실패 (에러 반환)
				send.On("Notify", mock.Anything, mock.Anything).Return(assert.AnError).Once()
			},
			wantErrLogs: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExe := &contractmocks.MockTaskExecutor{}
			mockSend := notificationmocks.NewMockNotificationSender(t)

			tt.setupMock(mockExe, mockSend)

			// 1. 테스트용 Task 설정 (먼 미래 시간으로 설정하여 자동 실행 방지)
			// cronx 패키지는 6필드를 요구하므로 초 단위를 포함해야 함: "0 0 0 1 1 *"
			taskConfig := config.TaskConfig{
				ID: "T1",
				Commands: []config.CommandConfig{
					{
						ID: "C1",
						Scheduler: struct {
							Runnable bool   `json:"runnable"`
							TimeSpec string `json:"time_spec"`
						}{Runnable: true, TimeSpec: "0 0 0 1 1 *"},
						DefaultNotifierID: "N1",
					},
				},
			}
			cfg := &config.AppConfig{Tasks: []config.TaskConfig{taskConfig}}
			s := NewService(cfg.Tasks, mockExe, mockSend)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var wg sync.WaitGroup

			// 2. 서비스 시작 (스케줄 등록)
			wg.Add(1)
			err := s.Start(ctx, &wg)
			if !assert.NoError(t, err) {
				return
			}

			// 3. 등록된 엔트리 가져오기
			entries := s.cron.Entries()
			if !assert.Len(t, entries, 1, "스케줄이 1개 등록되어야 합니다") {
				return // Panic 방지
			}

			// 4. 수동으로 작업 실행 (익명 함수 실행됨)
			entries[0].Job.Run()

			// 5. 검증
			mockExe.AssertExpectations(t)
			mockSend.AssertExpectations(t)

			s.stop()
		})
	}
}

// TestScheduler_ContextCancellation 서비스 종료 컨텍스트(Graceful Shutdown)가 정상 작동하는지 확인합니다.
func TestScheduler_ContextCancellation(t *testing.T) {
	mockExe := &contractmocks.MockTaskExecutor{}
	mockSend := notificationmocks.NewMockNotificationSender(t)
	s := NewService([]config.TaskConfig{}, mockExe, mockSend)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	wg.Add(1)
	err := s.Start(ctx, &wg)
	assert.NoError(t, err)
	assert.True(t, s.running)

	// Context 취소 (시뮬레이션: 서비스 종료 시그널)
	cancel()

	// WaitGroup이 Done 될 때까지 대기
	wg.Wait()

	// 서비스가 중지 상태로 변경되었는지 확인
	// Start 내부의 고루틴이 ctx.Done()을 감지하고 s.stop()을 호출해야 함
	// 비동기 실행이므로 약간의 지연이 있을 수 있지만, wg.Wait()으로 동기화됨
	s.runningMu.Lock()
	isRunning := s.running
	s.runningMu.Unlock()

	assert.False(t, isRunning, "Context 취소 시 서비스가 중지되어야 합니다")
}

// TestScheduler_Execution_MultipleTasks 여러 작업이 등록될 때 루프 변수 캡처(Closure Capture) 문제가 없는지 검증합니다.
func TestScheduler_Execution_MultipleTasks(t *testing.T) {
	mockExe := &contractmocks.MockTaskExecutor{}
	mockSend := notificationmocks.NewMockNotificationSender(t)

	// 서로 다른 3개의 태스크 정의
	tasks := []config.TaskConfig{
		{ID: "T1", Commands: []config.CommandConfig{{ID: "C1", Scheduler: struct {
			Runnable bool   `json:"runnable"`
			TimeSpec string `json:"time_spec"`
		}{true, "0 0 0 1 1 *"}, DefaultNotifierID: "N"}}},
		{ID: "T2", Commands: []config.CommandConfig{{ID: "C2", Scheduler: struct {
			Runnable bool   `json:"runnable"`
			TimeSpec string `json:"time_spec"`
		}{true, "0 0 0 1 1 *"}, DefaultNotifierID: "N"}}},
		{ID: "T3", Commands: []config.CommandConfig{{ID: "C3", Scheduler: struct {
			Runnable bool   `json:"runnable"`
			TimeSpec string `json:"time_spec"`
		}{true, "0 0 0 1 1 *"}, DefaultNotifierID: "N"}}},
	}

	// 각 태스크가 올바른 ID로 Submit 되는지 검증
	mockExe.On("Submit", mock.Anything, mock.MatchedBy(func(req *contract.TaskSubmitRequest) bool {
		return req.TaskID == "T1" && req.CommandID == "C1"
	})).Return(nil).Once()
	mockExe.On("Submit", mock.Anything, mock.MatchedBy(func(req *contract.TaskSubmitRequest) bool {
		return req.TaskID == "T2" && req.CommandID == "C2"
	})).Return(nil).Once()
	mockExe.On("Submit", mock.Anything, mock.MatchedBy(func(req *contract.TaskSubmitRequest) bool {
		return req.TaskID == "T3" && req.CommandID == "C3"
	})).Return(nil).Once()

	cfg := &config.AppConfig{Tasks: tasks}
	s := NewService(cfg.Tasks, mockExe, mockSend)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup

	wg.Add(1)
	err := s.Start(ctx, &wg)
	assert.NoError(t, err)
	defer s.stop()

	assert.Equal(t, 3, len(s.cron.Entries()))

	// 등록된 순서와 상관없이, 모든 Job을 한 번씩 수동 실행
	for _, entry := range s.cron.Entries() {
		entry.Job.Run()
	}

	mockExe.AssertExpectations(t)
}

// TestScheduler_InvalidCronSpec 잘못된 Cron 표현식 테스트
func TestScheduler_InvalidCronSpec(t *testing.T) {
	mockExe := &contractmocks.MockTaskExecutor{}
	mockSend := notificationmocks.NewMockNotificationSender(t)

	cfg := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: "InvalidTask",
				Commands: []config.CommandConfig{
					{
						ID: "InvalidCmd",
						Scheduler: struct {
							Runnable bool   `json:"runnable"`
							TimeSpec string `json:"time_spec"`
						}{Runnable: true, TimeSpec: "invalid-spec"},
						DefaultNotifierID: "N1",
					},
				},
			},
		},
	}
	s := NewService(cfg.Tasks, mockExe, mockSend)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup

	wg.Add(1)
	err := s.Start(ctx, &wg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "스케줄 등록 실패")
	assert.Contains(t, err.Error(), "invalid-spec")

	s.stop()
	mockSend.AssertExpectations(t)
}
