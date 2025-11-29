package task

import (
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/g"
	"github.com/stretchr/testify/assert"
)

func TestScheduler_StartStop(t *testing.T) {
	t.Run("스케줄러 시작 및 중지", func(t *testing.T) {
		s := &scheduler{}

		// Mock 객체 생성
		mockSender := NewMockTaskNotificationSender()
		mockRunner := &mockTaskRunner{}

		// 빈 설정으로 시작
		config := &g.AppConfig{}

		// 스케줄러 시작
		s.Start(config, mockRunner, mockSender)

		// 스케줄러 중지
		s.Stop()

		// 중지 후 다시 시작 가능한지 확인
		s.Start(config, mockRunner, mockSender)
		s.Stop()
	})

	t.Run("이미 실행 중인 스케줄러 재시작 시도", func(t *testing.T) {
		s := &scheduler{}
		mockSender := NewMockTaskNotificationSender()
		mockRunner := &mockTaskRunner{}
		config := &g.AppConfig{}

		// 첫 번째 시작
		s.Start(config, mockRunner, mockSender)
		assert.True(t, s.running, "스케줄러가 실행 중이어야 합니다")

		// 두 번째 시작 시도 (중복)
		s.Start(config, mockRunner, mockSender)
		assert.True(t, s.running, "스케줄러가 여전히 실행 중이어야 합니다")

		// 정리
		s.Stop()
	})

	t.Run("이미 중지된 스케줄러 재중지 시도", func(t *testing.T) {
		s := &scheduler{}
		mockSender := NewMockTaskNotificationSender()
		mockRunner := &mockTaskRunner{}
		config := &g.AppConfig{}

		// 시작 후 중지
		s.Start(config, mockRunner, mockSender)
		s.Stop()
		assert.False(t, s.running, "스케줄러가 중지되어야 합니다")

		// 두 번째 중지 시도 (중복)
		s.Stop()
		assert.False(t, s.running, "스케줄러가 여전히 중지 상태여야 합니다")
	})

	t.Run("스케줄 작업 등록 - Runnable이 true인 작업만", func(t *testing.T) {
		s := &scheduler{}
		mockSender := NewMockTaskNotificationSender()
		mockRunner := &mockTaskRunner{}

		config := &g.AppConfig{
			Tasks: []struct {
				ID       string `json:"id"`
				Title    string `json:"title"`
				Commands []struct {
					ID          string `json:"id"`
					Title       string `json:"title"`
					Description string `json:"description"`
					Scheduler   struct {
						Runnable bool   `json:"runnable"`
						TimeSpec string `json:"time_spec"`
					} `json:"scheduler"`
					Notifier struct {
						Usable bool `json:"usable"`
					} `json:"notifier"`
					DefaultNotifierID string                 `json:"default_notifier_id"`
					Data              map[string]interface{} `json:"data"`
				} `json:"commands"`
				Data map[string]interface{} `json:"data"`
			}{
				{
					ID:    "TestTask",
					Title: "테스트 작업",
					Commands: []struct {
						ID          string `json:"id"`
						Title       string `json:"title"`
						Description string `json:"description"`
						Scheduler   struct {
							Runnable bool   `json:"runnable"`
							TimeSpec string `json:"time_spec"`
						} `json:"scheduler"`
						Notifier struct {
							Usable bool `json:"usable"`
						} `json:"notifier"`
						DefaultNotifierID string                 `json:"default_notifier_id"`
						Data              map[string]interface{} `json:"data"`
					}{
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
		s.Start(config, mockRunner, mockSender)
		assert.True(t, s.running, "스케줄러가 실행 중이어야 합니다")
		assert.NotNil(t, s.cron, "cron 객체가 생성되어야 합니다")

		// 등록된 스케줄 작업 수 확인 (Runnable이 true인 것만)
		entries := s.cron.Entries()
		assert.Equal(t, 1, len(entries), "Runnable이 true인 작업만 등록되어야 합니다")

		// 정리
		s.Stop()
	})

	t.Run("스케줄 실행 시 TaskRunner 호출 검증", func(t *testing.T) {
		s := &scheduler{}
		mockSender := NewMockTaskNotificationSender()
		mockRunner := &mockTaskRunner{}

		config := &g.AppConfig{
			Tasks: []struct {
				ID       string `json:"id"`
				Title    string `json:"title"`
				Commands []struct {
					ID          string `json:"id"`
					Title       string `json:"title"`
					Description string `json:"description"`
					Scheduler   struct {
						Runnable bool   `json:"runnable"`
						TimeSpec string `json:"time_spec"`
					} `json:"scheduler"`
					Notifier struct {
						Usable bool `json:"usable"`
					} `json:"notifier"`
					DefaultNotifierID string                 `json:"default_notifier_id"`
					Data              map[string]interface{} `json:"data"`
				} `json:"commands"`
				Data map[string]interface{} `json:"data"`
			}{
				{
					ID:    "TestTask",
					Title: "테스트 작업",
					Commands: []struct {
						ID          string `json:"id"`
						Title       string `json:"title"`
						Description string `json:"description"`
						Scheduler   struct {
							Runnable bool   `json:"runnable"`
							TimeSpec string `json:"time_spec"`
						} `json:"scheduler"`
						Notifier struct {
							Usable bool `json:"usable"`
						} `json:"notifier"`
						DefaultNotifierID string                 `json:"default_notifier_id"`
						Data              map[string]interface{} `json:"data"`
					}{
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

		// 스케줄러 시작
		s.Start(config, mockRunner, mockSender)

		// 스케줄이 실행될 때까지 대기 (최대 2초)
		time.Sleep(2 * time.Second)

		// TaskRunner가 호출되었는지 확인
		calls := mockRunner.GetTaskRunCalls()
		assert.Greater(t, len(calls), 0, "TaskRunner.TaskRun이 최소 1번 호출되어야 합니다")

		// 호출 파라미터 검증
		if len(calls) > 0 {
			call := calls[0]
			assert.Equal(t, TaskID("TestTask"), call.taskID, "TaskID가 일치해야 합니다")
			assert.Equal(t, TaskCommandID("QuickCommand"), call.taskCommandID, "TaskCommandID가 일치해야 합니다")
			assert.Equal(t, "test-notifier", call.notifierID, "NotifierID가 일치해야 합니다")
			assert.Equal(t, TaskRunByScheduler, call.taskRunBy, "TaskRunBy가 Scheduler여야 합니다")
		}

		// 정리
		s.Stop()
	})

	t.Run("스케줄 실행 실패 시 알림 발송", func(t *testing.T) {
		s := &scheduler{}
		mockSender := NewMockTaskNotificationSender()
		mockRunner := &mockTaskRunnerWithFailure{} // 실패하는 TaskRunner

		config := &g.AppConfig{
			Tasks: []struct {
				ID       string `json:"id"`
				Title    string `json:"title"`
				Commands []struct {
					ID          string `json:"id"`
					Title       string `json:"title"`
					Description string `json:"description"`
					Scheduler   struct {
						Runnable bool   `json:"runnable"`
						TimeSpec string `json:"time_spec"`
					} `json:"scheduler"`
					Notifier struct {
						Usable bool `json:"usable"`
					} `json:"notifier"`
					DefaultNotifierID string                 `json:"default_notifier_id"`
					Data              map[string]interface{} `json:"data"`
				} `json:"commands"`
				Data map[string]interface{} `json:"data"`
			}{
				{
					ID:    "FailTask",
					Title: "실패 작업",
					Commands: []struct {
						ID          string `json:"id"`
						Title       string `json:"title"`
						Description string `json:"description"`
						Scheduler   struct {
							Runnable bool   `json:"runnable"`
							TimeSpec string `json:"time_spec"`
						} `json:"scheduler"`
						Notifier struct {
							Usable bool `json:"usable"`
						} `json:"notifier"`
						DefaultNotifierID string                 `json:"default_notifier_id"`
						Data              map[string]interface{} `json:"data"`
					}{
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

		// 스케줄러 시작
		s.Start(config, mockRunner, mockSender)

		// 스케줄이 실행되고 실패할 때까지 대기
		time.Sleep(2 * time.Second)

		// 알림이 발송되었는지 확인
		assert.Greater(t, mockSender.GetNotifyWithTaskContextCallCount(), 0, "실패 시 알림이 발송되어야 합니다")

		// 알림 내용 검증
		if mockSender.GetNotifyWithTaskContextCallCount() > 0 {
			call := mockSender.NotifyWithTaskContextCalls[0]
			assert.Equal(t, "fail-notifier", call.NotifierID, "NotifierID가 일치해야 합니다")
			assert.Contains(t, call.Message, "작업 스케쥴러에서의 작업 실행 요청이 실패하였습니다", "실패 메시지가 포함되어야 합니다")
			assert.NotNil(t, call.TaskCtx, "TaskContext가 있어야 합니다")
		}

		// 정리
		s.Stop()
	})
}

// mockTaskRunner는 테스트용 TaskRunner 구현체입니다.
type mockTaskRunner struct {
	mu           sync.Mutex
	taskRunCalls []taskRunCall
}

type taskRunCall struct {
	taskID        TaskID
	taskCommandID TaskCommandID
	notifierID    string
	taskRunBy     TaskRunBy
}

func (m *mockTaskRunner) TaskRun(taskID TaskID, taskCommandID TaskCommandID, notifierID string, notifyResultOfTaskRunRequest bool, taskRunBy TaskRunBy) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.taskRunCalls = append(m.taskRunCalls, taskRunCall{
		taskID:        taskID,
		taskCommandID: taskCommandID,
		notifierID:    notifierID,
		taskRunBy:     taskRunBy,
	})
	return true
}

func (m *mockTaskRunner) GetTaskRunCalls() []taskRunCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a copy to avoid race conditions on the slice itself if modified later (though append creates new underlying array usually, better safe)
	calls := make([]taskRunCall, len(m.taskRunCalls))
	copy(calls, m.taskRunCalls)
	return calls
}

func (m *mockTaskRunner) TaskRunWithContext(taskID TaskID, taskCommandID TaskCommandID, taskCtx TaskContext, notifierID string, notifyResultOfTaskRunRequest bool, taskRunBy TaskRunBy) bool {
	return m.TaskRun(taskID, taskCommandID, notifierID, notifyResultOfTaskRunRequest, taskRunBy)
}

func (m *mockTaskRunner) TaskCancel(taskInstanceID TaskInstanceID) bool {
	return true
}

// mockTaskRunnerWithFailure는 항상 실패하는 TaskRunner 구현체입니다.
type mockTaskRunnerWithFailure struct{}

func (m *mockTaskRunnerWithFailure) TaskRun(taskID TaskID, taskCommandID TaskCommandID, notifierID string, notifyResultOfTaskRunRequest bool, taskRunBy TaskRunBy) bool {
	return false // 항상 실패
}

func (m *mockTaskRunnerWithFailure) TaskRunWithContext(taskID TaskID, taskCommandID TaskCommandID, taskCtx TaskContext, notifierID string, notifyResultOfTaskRunRequest bool, taskRunBy TaskRunBy) bool {
	return false // 항상 실패
}

func (m *mockTaskRunnerWithFailure) TaskCancel(taskInstanceID TaskInstanceID) bool {
	return true
}
