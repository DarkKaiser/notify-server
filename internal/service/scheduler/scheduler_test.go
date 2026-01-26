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
		// 테스트를 위해 강제로 nil 할당 (내부 필드 접근)
		s.taskSubmitter = nil
		s.notificationSender = mockSend // 복구

		wg.Add(1)
		err := s.Start(ctx, wg)
		assert.ErrorIs(t, err, ErrTaskSubmitterNotInitialized)
		s.taskSubmitter = mockExe // 복구
	})

	t.Run("Error_NotificationSenderNil", func(t *testing.T) {
		s.notificationSender = nil
		s.taskSubmitter = mockExe // 복구

		wg.Add(1)
		err := s.Start(ctx, wg)
		assert.ErrorIs(t, err, ErrNotificationSenderNotInitialized)
		s.notificationSender = mockSend // 복구
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
				s.Stop()
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
				s.Stop()
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
				s.Stop()
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
			cfg := &config.AppConfig{}
			s := NewService(cfg.Tasks, mockExe, mockSend)

			if tt.initialState != nil {
				tt.initialState(s)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var wg sync.WaitGroup

			// Cleanup safely at end of test
			defer s.Stop()

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
							}{Runnable: true, TimeSpec: "* * * * * *"},
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
							}{Runnable: false, TimeSpec: "* * * * * *"},
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
			s.Start(ctx, &wg)
			defer s.Stop()

			if s.cron != nil {
				assert.Equal(t, tt.expectedCount, len(s.cron.Entries()))
			} else {
				assert.Equal(t, 0, tt.expectedCount)
			}
		})
	}
}

// TestScheduler_Execution 작업 실행 요청 및 실패 시 알림 전송을 통합 테스트합니다.
func TestScheduler_Execution(t *testing.T) {
	tests := []struct {
		name            string
		taskConfig      config.TaskConfig
		mockSetup       func(*contractmocks.MockTaskExecutor, *notificationmocks.MockNotificationSender, *sync.WaitGroup)
		shouldFailNotif bool
	}{
		{
			name: "Successful Execution",
			taskConfig: config.TaskConfig{
				ID: "T1",
				Commands: []config.CommandConfig{
					{
						ID: "C1",
						Scheduler: struct {
							Runnable bool   `json:"runnable"`
							TimeSpec string `json:"time_spec"`
						}{Runnable: true, TimeSpec: "* * * * * *"},
						DefaultNotifierID: "N1",
					},
				},
			},
			mockSetup: func(exe *contractmocks.MockTaskExecutor, send *notificationmocks.MockNotificationSender, wg *sync.WaitGroup) {
				// Submit 호출 시 context.Background()가 전달되는지도 확인 가능하지만,
				// 여기서는 mock.Anything으로 유연하게 처리
				exe.On("Submit", mock.Anything, mock.MatchedBy(func(req *contract.TaskSubmitRequest) bool {
					return req.TaskID == "T1" && req.CommandID == "C1" && req.RunBy == contract.TaskRunByScheduler
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(nil).Once()

				// 성공 시 알림은 선택 사항 (Maybe)
				send.On("Notify", mock.Anything, mock.Anything).Return(nil).Maybe()
			},
		},
		{
			name: "Failed Execution with Notification",
			taskConfig: config.TaskConfig{
				ID: "T2",
				Commands: []config.CommandConfig{
					{
						ID: "C2",
						Scheduler: struct {
							Runnable bool   `json:"runnable"`
							TimeSpec string `json:"time_spec"`
						}{Runnable: true, TimeSpec: "* * * * * *"},
						DefaultNotifierID: "N2",
					},
				},
			},
			mockSetup: func(exe *contractmocks.MockTaskExecutor, send *notificationmocks.MockNotificationSender, wg *sync.WaitGroup) {
				// Submit이 실패를 반환하도록 설정
				exe.On("Submit", mock.Anything, mock.MatchedBy(func(req *contract.TaskSubmitRequest) bool {
					return req.TaskID == "T2" && req.CommandID == "C2"
				})).Return(assert.AnError).Once()

				// 알림 전송 확인
				send.On("Notify", mock.Anything, mock.MatchedBy(func(n contract.Notification) bool {
					return n.ErrorOccurred && strings.Contains(n.Message, "TaskSubmitter 실행 중 오류")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(nil).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExe := &contractmocks.MockTaskExecutor{}
			mockSend := notificationmocks.NewMockNotificationSender(t)

			// Mock 설정
			// 비동기 작업 완료를 기다리기 위한 별도 WaitGroup
			var executionWg sync.WaitGroup
			executionWg.Add(1)

			tt.mockSetup(mockExe, mockSend, &executionWg)

			// 서비스 시작
			cfg := &config.AppConfig{Tasks: []config.TaskConfig{tt.taskConfig}}
			s := NewService(cfg.Tasks, mockExe, mockSend)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var serviceWg sync.WaitGroup

			serviceWg.Add(1)
			s.Start(ctx, &serviceWg)
			defer s.Stop()

			// 실행 대기 (타임아웃 적용)
			done := make(chan struct{})
			go func() {
				executionWg.Wait()
				close(done)
			}()

			select {
			case <-done:
				// Success
			case <-time.After(2 * time.Second):
				t.Fatal("Timeout waiting for task execution or notification")
			}

			mockExe.AssertExpectations(t)
			mockSend.AssertExpectations(t)
		})
	}
}

// TestScheduler_InvalidCronSpec 잘못된 Cron 표현식 입력 시 알림이 전송되는지 테스트합니다.
// 버그 수정 사항: 에러 메시지 매칭 문자열 수정 ("Cron 스케줄 파싱 실패" -> "스케줄 등록 실패")
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

	// 수정됨: 실제 구현 코드의 에러 메시지와 일치하도록 수정
	mockSend.On("Notify", mock.Anything, mock.MatchedBy(func(n contract.Notification) bool {
		return strings.Contains(n.Message, "스케줄 등록 실패") && n.ErrorOccurred
	})).Return(nil).Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup

	wg.Add(1)
	s.Start(ctx, &wg)
	defer s.Stop()

	mockSend.AssertExpectations(t)
}
