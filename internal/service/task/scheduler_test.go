package task

import (
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestScheduler_Lifecycle_Table(t *testing.T) {
	tests := []struct {
		name         string
		initialState func(*scheduler) // Setup before action
		action       func(*scheduler, *config.AppConfig, *MockTaskExecutor, *MockTestifyNotificationSender)
		verify       func(*testing.T, *scheduler)
		doubleAction bool // Repeats action to check idempotency logic
	}{
		{
			name: "Start Scheduler",
			action: func(s *scheduler, cfg *config.AppConfig, exec *MockTaskExecutor, sender *MockTestifyNotificationSender) {
				s.Start(cfg, exec, sender)
			},
			verify: func(t *testing.T, s *scheduler) {
				assert.True(t, s.running)
				assert.NotNil(t, s.cron)
			},
		},
		{
			name: "Stop Scheduler Safely",
			initialState: func(s *scheduler) {
				// With defensive Stop(), this should not panic even if cron is nil but running is true
				// (simulating inconsistent state or just testing robustness)
				s.running = true
				s.cron = nil
			},
			action: func(s *scheduler, cfg *config.AppConfig, exec *MockTaskExecutor, sender *MockTestifyNotificationSender) {
				s.Stop()
			},
			verify: func(t *testing.T, s *scheduler) {
				assert.False(t, s.running)
			},
		},
		{
			name: "Restart Scheduler",
			action: func(s *scheduler, cfg *config.AppConfig, exec *MockTaskExecutor, sender *MockTestifyNotificationSender) {
				s.Start(cfg, exec, sender)
				s.Stop()
				s.Start(cfg, exec, sender)
			},
			verify: func(t *testing.T, s *scheduler) {
				assert.True(t, s.running)
			},
		},
		{
			name:         "Duplicate Start",
			doubleAction: true,
			action: func(s *scheduler, cfg *config.AppConfig, exec *MockTaskExecutor, sender *MockTestifyNotificationSender) {
				s.Start(cfg, exec, sender)
			},
			verify: func(t *testing.T, s *scheduler) {
				assert.True(t, s.running)
			},
		},
		{
			name:         "Duplicate Stop",
			doubleAction: true,
			action: func(s *scheduler, cfg *config.AppConfig, exec *MockTaskExecutor, sender *MockTestifyNotificationSender) {
				s.Start(cfg, exec, sender)
				s.Stop()
			},
			verify: func(t *testing.T, s *scheduler) {
				assert.False(t, s.running)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &scheduler{}
			mockExe := &MockTaskExecutor{}
			mockSend := &MockTestifyNotificationSender{}
			cfg := &config.AppConfig{}

			if tt.initialState != nil {
				tt.initialState(s)
			}

			tt.action(s, cfg, mockExe, mockSend)
			if tt.doubleAction {
				tt.action(s, cfg, mockExe, mockSend)
			}

			// Always cleanup
			defer s.Stop()

			if tt.verify != nil {
				tt.verify(t, s)
			}
		})
	}
}

func TestScheduler_TaskRegistration_Table(t *testing.T) {
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
		{
			name: "Mixed Tasks",
			tasks: []config.TaskConfig{
				{
					ID: "T1",
					Commands: []config.CommandConfig{
						{
							ID: "C1",
							Scheduler: struct {
								Runnable bool   `json:"runnable"`
								TimeSpec string `json:"time_spec"`
							}{Runnable: true, TimeSpec: "* * * * * *"}, // 1
						},
						{
							ID: "C2",
							Scheduler: struct {
								Runnable bool   `json:"runnable"`
								TimeSpec string `json:"time_spec"`
							}{Runnable: false, TimeSpec: "* * * * * *"}, // 0
						},
					},
				},
			},
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &scheduler{}
			mockExe := &MockTaskExecutor{}
			mockSend := &MockTestifyNotificationSender{}
			cfg := &config.AppConfig{Tasks: tt.tasks}

			s.Start(cfg, mockExe, mockSend)
			defer s.Stop()

			if s.cron != nil {
				assert.Equal(t, tt.expectedCount, len(s.cron.Entries()))
			} else {
				assert.Equal(t, 0, tt.expectedCount)
			}
		})
	}
}

func TestScheduler_Execution_Table(t *testing.T) {
	tests := []struct {
		name            string
		taskConfig      config.TaskConfig
		mockSetup       func(*MockTaskExecutor, *MockTestifyNotificationSender, *sync.WaitGroup)
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
			mockSetup: func(exe *MockTaskExecutor, send *MockTestifyNotificationSender, wg *sync.WaitGroup) {
				exe.On("Submit", mock.MatchedBy(func(req *contract.TaskSubmitRequest) bool {
					return req.TaskID == "T1" && req.CommandID == "C1" && req.RunBy == contract.TaskRunByScheduler
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(nil).Once()
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
			mockSetup: func(exe *MockTaskExecutor, send *MockTestifyNotificationSender, wg *sync.WaitGroup) {
				exe.On("Submit", mock.MatchedBy(func(req *contract.TaskSubmitRequest) bool {
					return req.TaskID == "T2" && req.CommandID == "C2" && req.RunBy == contract.TaskRunByScheduler
				})).Run(func(args mock.Arguments) {
					// We don't call wg.Done here because we wait for Notify
				}).Return(assert.AnError).Once()

				send.On("Notify", mock.Anything, types.NotifierID("N2"), mock.MatchedBy(func(msg string) bool {
					return msg != "" && assert.Contains(nil, msg, "작업 스케쥴러에서의 작업 실행 요청이 실패하였습니다") // nil passed to assert helper which is weird but works for Contains if t is not needed or we check bool
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(nil).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &scheduler{}
			mockExe := &MockTaskExecutor{}
			mockSend := &MockTestifyNotificationSender{}
			cfg := &config.AppConfig{Tasks: []config.TaskConfig{tt.taskConfig}}

			var wg sync.WaitGroup
			wg.Add(1)

			if tt.mockSetup != nil {
				tt.mockSetup(mockExe, mockSend, &wg)
			}

			s.Start(cfg, mockExe, mockSend)
			defer s.Stop()

			// Wait with timeout
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
				// Success
			case <-time.After(2 * time.Second):
				t.Fatal("Timeout waiting for execution/notification")
			}

			mockExe.AssertExpectations(t)
			mockSend.AssertExpectations(t)
		})
	}
}

func TestScheduler_InvalidCronSpec(t *testing.T) {
	// Not easy to table-drive since it's a specific error handling case logged via notification
	// But we can verify it cleanly.
	s := &scheduler{}
	mockExe := &MockTaskExecutor{}
	mockSend := &MockTestifyNotificationSender{}

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

	mockSend.On("Notify", mock.Anything, types.NotifierID("N1"), mock.MatchedBy(func(msg string) bool {
		return assert.Contains(t, msg, "Cron 스케줄 파싱 실패")
	})).Return(nil).Once()

	s.Start(cfg, mockExe, mockSend)
	defer s.Stop()

	mockSend.AssertExpectations(t)
}
