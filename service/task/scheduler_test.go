package task

import (
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestScheduler_StartStop(t *testing.T) {
	t.Run("스케줄러 시작 및 중지", func(t *testing.T) {
		s := &scheduler{}

		// Mock 객체 생성
		mockSender := &MockNotificationSender{}
		mockExecutor := &MockTaskExecutor{}

		// 빈 설정으로 시작
		appConfig := &config.AppConfig{}

		// 스케줄러 시작
		s.Start(appConfig, mockExecutor, mockSender)
		defer s.Stop() // Ensure cleanup

		assert.True(t, s.running, "스케줄러가 실행 중이어야 합니다")
		assert.NotNil(t, s.cron, "cron 객체가 생성되어야 합니다")

		// 스케줄러 중지
		s.Stop()
		assert.False(t, s.running, "스케줄러가 중지되어야 합니다")

		// 중지 후 다시 시작 가능한지 확인
		s.Start(appConfig, mockExecutor, mockSender)
		assert.True(t, s.running, "스케줄러가 다시 실행 중이어야 합니다")
	})

	t.Run("이미 실행 중인 스케줄러 재시작 시도", func(t *testing.T) {
		s := &scheduler{}
		mockSender := &MockNotificationSender{}
		mockExecutor := &MockTaskExecutor{}
		appConfig := &config.AppConfig{}

		// 첫 번째 시작
		s.Start(appConfig, mockExecutor, mockSender)
		defer s.Stop()

		assert.True(t, s.running, "스케줄러가 실행 중이어야 합니다")

		// 두 번째 시작 시도 (중복)
		s.Start(appConfig, mockExecutor, mockSender)
		assert.True(t, s.running, "스케줄러가 여전히 실행 중이어야 합니다")
	})

	t.Run("이미 중지된 스케줄러 재중지 시도", func(t *testing.T) {
		s := &scheduler{}
		mockSender := &MockNotificationSender{}
		mockExecutor := &MockTaskExecutor{}
		appConfig := &config.AppConfig{}

		// 시작 후 중지
		s.Start(appConfig, mockExecutor, mockSender)
		s.Stop()
		assert.False(t, s.running, "스케줄러가 중지되어야 합니다")

		// 두 번째 중지 시도 (중복)
		s.Stop()
		assert.False(t, s.running, "스케줄러가 여전히 중지 상태여야 합니다")
	})

	t.Run("스케줄 작업 등록 - Runnable이 true인 작업만", func(t *testing.T) {
		s := &scheduler{}
		mockSender := &MockNotificationSender{}
		mockExecutor := &MockTaskExecutor{}

		appConfig := &config.AppConfig{
			Tasks: []config.TaskConfig{
				{
					ID:    "TestTask",
					Title: "테스트 작업",
					Commands: []config.TaskCommandConfig{
						{
							ID:    "RunnableCommand",
							Title: "실행 가능한 명령",
							Scheduler: struct {
								Runnable bool   `json:"runnable"`
								TimeSpec string `json:"time_spec"`
							}{
								Runnable: true,
								TimeSpec: "*/5 * * * * *", // 5초마다
							},
							DefaultNotifierID: "test-notifier",
						},
						{
							ID:    "NonRunnableCommand",
							Title: "실행 불가능한 명령",
							Scheduler: struct {
								Runnable bool   `json:"runnable"`
								TimeSpec string `json:"time_spec"`
							}{
								Runnable: false,
								TimeSpec: "*/5 * * * * *",
							},
							DefaultNotifierID: "test-notifier",
						},
					},
				},
			},
		}

		// 스케줄러 시작
		s.Start(appConfig, mockExecutor, mockSender)
		defer s.Stop()

		assert.True(t, s.running, "스케줄러가 실행 중이어야 합니다")
		assert.NotNil(t, s.cron, "cron 객체가 생성되어야 합니다")

		// 등록된 스케줄 작업 수 확인 (Runnable이 true인 것만)
		entries := s.cron.Entries()
		assert.Equal(t, 1, len(entries), "Runnable이 true인 작업만 등록되어야 합니다")
	})

	t.Run("스케줄 실행 시 TaskExecutor 호출 검증", func(t *testing.T) {
		s := &scheduler{}
		mockSender := &MockNotificationSender{}
		mockExecutor := &MockTaskExecutor{}

		appConfig := &config.AppConfig{
			Tasks: []config.TaskConfig{
				{
					ID:    "TestTask",
					Title: "테스트 작업",
					Commands: []config.TaskCommandConfig{
						{
							ID:    "QuickCommand",
							Title: "빠른 실행 명령",
							Scheduler: struct {
								Runnable bool   `json:"runnable"`
								TimeSpec string `json:"time_spec"`
							}{
								Runnable: true,
								TimeSpec: "* * * * * *", // 매초 실행
							},
							DefaultNotifierID: "test-notifier",
						},
					},
				},
			},
		}

		done := make(chan struct{})

		// Expect TaskRun call
		mockExecutor.On("Run", mock.MatchedBy(func(req *RunRequest) bool {
			return req.TaskID == "TestTask" &&
				req.TaskCommandID == "QuickCommand" &&
				req.NotifierID == "test-notifier" &&
				req.NotifyOnStart == false &&
				req.RunBy == RunByScheduler
		})).Run(func(args mock.Arguments) {
			close(done)
		}).Return(nil).Once()

		// 스케줄러 시작
		s.Start(appConfig, mockExecutor, mockSender)
		defer s.Stop()

		// Wait for execution
		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for TaskRun")
		}

		mockExecutor.AssertExpectations(t)
	})

	t.Run("스케줄 실행 실패 시 알림 발송", func(t *testing.T) {
		s := &scheduler{}
		mockSender := &MockNotificationSender{}
		mockExecutor := &MockTaskExecutor{}

		appConfig := &config.AppConfig{
			Tasks: []config.TaskConfig{
				{
					ID:    "FailTask",
					Title: "실패 작업",
					Commands: []config.TaskCommandConfig{
						{
							ID:    "FailCommand",
							Title: "실패 명령",
							Scheduler: struct {
								Runnable bool   `json:"runnable"`
								TimeSpec string `json:"time_spec"`
							}{
								Runnable: true,
								TimeSpec: "* * * * * *", // 매초 실행
							},
							DefaultNotifierID: "fail-notifier",
						},
					},
				},
			},
		}

		var wg sync.WaitGroup
		wg.Add(2) // 1 for TaskRun (fail), 1 for Notify

		// Expect TaskRun call (return false for failure)
		mockExecutor.On("Run", mock.MatchedBy(func(req *RunRequest) bool {
			return req.TaskID == "FailTask" &&
				req.TaskCommandID == "FailCommand" &&
				req.NotifierID == "fail-notifier" &&
				req.NotifyOnStart == false &&
				req.RunBy == RunByScheduler
		})).Run(func(args mock.Arguments) {
			wg.Done()
		}).Return(assert.AnError).Once()

		// Expect Notify call
		mockSender.On("NotifyWithTaskContext", "fail-notifier", mock.MatchedBy(func(msg string) bool {
			return assert.Contains(t, msg, "작업 스케쥴러에서의 작업 실행 요청이 실패하였습니다")
		}), mock.Anything).
			Run(func(args mock.Arguments) {
				wg.Done()
			}).Return().Once()

		// 스케줄러 시작
		s.Start(appConfig, mockExecutor, mockSender)
		defer s.Stop()

		// Wait for completion
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for failure notification")
		}

		mockExecutor.AssertExpectations(t)
		mockSender.AssertExpectations(t)
	})

	t.Run("스케줄 등록 실패 - 잘못된 TimeSpec", func(t *testing.T) {
		s := &scheduler{}
		mockSender := &MockNotificationSender{}
		mockExecutor := &MockTaskExecutor{}

		appConfig := &config.AppConfig{
			Tasks: []config.TaskConfig{
				{
					ID:    "InvalidTask",
					Title: "잘못된 작업",
					Commands: []config.TaskCommandConfig{
						{
							ID:    "InvalidCommand",
							Title: "잘못된 명령",
							Scheduler: struct {
								Runnable bool   `json:"runnable"`
								TimeSpec string `json:"time_spec"`
							}{
								Runnable: true,
								TimeSpec: "invalid-cron-spec", // 잘못된 표현식
							},
							DefaultNotifierID: "test-notifier",
						},
					},
				},
			},
		}

		// Note: The original code handled error immediately in Start loop because Start is synchronous regarding config parsing.
		// However, Start method loop iterates configs, if Parse fails, it calls handleError.
		// Since we want to verify this call, and it happens during Start execution (synchronously relative to config processing loop),
		// we can just set expectation and assert after Start.

		mockSender.On("NotifyWithTaskContext", "test-notifier", mock.MatchedBy(func(msg string) bool {
			return assert.Contains(t, msg, "Cron 스케줄 파싱 실패")
		}), mock.Anything).Return().Once()

		// 스케줄러 시작
		s.Start(appConfig, mockExecutor, mockSender)
		defer s.Stop()

		mockSender.AssertExpectations(t)
	})
}
