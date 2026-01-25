package provider

import (
	"errors"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/stretchr/testify/assert"
)

func TestTask_Log(t *testing.T) {
	// 로거 훅 설정 (로그 캡처)
	hook := NewMemoryHook()
	applog.StandardLogger().AddHook(hook)
	defer func() {
		hook.Reset()
		// RemoveHook 기능이 없으므로... (동일)
	}()

	task := NewBaseTask("TEST_TASK", "TEST_CMD", "instance-1", "test-notifier", contract.TaskRunByScheduler)

	tests := []struct {
		name      string
		component string
		level     applog.Level
		message   string
		fields    applog.Fields
		err       error
		validate  func(t *testing.T, entry *applog.Entry)
	}{
		{
			name:      "기본 로깅 (필드 없음, 에러 없음)",
			component: "test.component",
			level:     applog.InfoLevel,
			message:   "info message",
			fields:    nil,
			err:       nil,
			validate: func(t *testing.T, entry *applog.Entry) {
				assert.Equal(t, applog.InfoLevel, entry.Level)
				assert.Equal(t, "info message", entry.Message)
				assert.Equal(t, "test.component", entry.Data["component"])
				assert.Equal(t, contract.TaskID("TEST_TASK"), entry.Data["task_id"])
				assert.Equal(t, contract.TaskCommandID("TEST_CMD"), entry.Data["command_id"])
			},
		},
		{
			name:      "추가 필드 포함",
			component: "test.component",
			level:     applog.WarnLevel,
			message:   "warn message",
			fields:    applog.Fields{"custom_field": "value"},
			err:       nil,
			validate: func(t *testing.T, entry *applog.Entry) {
				assert.Equal(t, applog.WarnLevel, entry.Level)
				assert.Equal(t, "warn message", entry.Message)
				assert.Equal(t, "value", entry.Data["custom_field"])
			},
		},
		{
			name:      "에러 포함",
			component: "test.component",
			level:     applog.ErrorLevel,
			message:   "error message",
			fields:    nil,
			err:       errors.New("test error"),
			validate: func(t *testing.T, entry *applog.Entry) {
				assert.Equal(t, applog.ErrorLevel, entry.Level)
				assert.Equal(t, "error message", entry.Message)
				assert.Equal(t, "test error", entry.Data["error"].(error).Error())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook.Reset()
			task.LogWithContext(tt.component, tt.level, tt.message, tt.fields, tt.err)

			requireEntry(t, hook)
			tt.validate(t, hook.LastEntry())
		})
	}
}

func requireEntry(t *testing.T, hook *MemoryHook) {
	if len(hook.Entries) == 0 {
		t.Fatal("로그가 기록되지 않았습니다")
	}
}

// MemoryHook 테스트용 로그 훅 구현체
type MemoryHook struct {
	Entries []*applog.Entry
}

func NewMemoryHook() *MemoryHook {
	return &MemoryHook{}
}

func (h *MemoryHook) Levels() []applog.Level {
	return applog.AllLevels
}

func (h *MemoryHook) Fire(entry *applog.Entry) error {
	h.Entries = append(h.Entries, entry)
	return nil
}

func (h *MemoryHook) Reset() {
	h.Entries = make([]*applog.Entry, 0)
}

func (h *MemoryHook) LastEntry() *applog.Entry {
	if len(h.Entries) == 0 {
		return nil
	}
	return h.Entries[len(h.Entries)-1]
}
