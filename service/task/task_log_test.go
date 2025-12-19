package task

import (
	"errors"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestTask_Log(t *testing.T) {
	// 로거 훅 설정 (로그 캡처)
	hook := NewMemoryHook()
	log.AddHook(hook)
	defer func() {
		hook.Reset()
		// RemoveHook 기능이 없으므로... (동일)
	}()

	task := NewBaseTask("TEST_TASK", "TEST_CMD", "instance-1", "test-notifier", RunByScheduler)

	tests := []struct {
		name      string
		component string
		level     log.Level
		message   string
		fields    log.Fields
		err       error
		validate  func(t *testing.T, entry *log.Entry)
	}{
		{
			name:      "기본 로깅 (필드 없음, 에러 없음)",
			component: "test.component",
			level:     log.InfoLevel,
			message:   "info message",
			fields:    nil,
			err:       nil,
			validate: func(t *testing.T, entry *log.Entry) {
				assert.Equal(t, log.InfoLevel, entry.Level)
				assert.Equal(t, "info message", entry.Message)
				assert.Equal(t, "test.component", entry.Data["component"])
				assert.Equal(t, ID("TEST_TASK"), entry.Data["task_id"])
				assert.Equal(t, CommandID("TEST_CMD"), entry.Data["command_id"])
			},
		},
		{
			name:      "추가 필드 포함",
			component: "test.component",
			level:     log.WarnLevel,
			message:   "warn message",
			fields:    log.Fields{"custom_field": "value"},
			err:       nil,
			validate: func(t *testing.T, entry *log.Entry) {
				assert.Equal(t, log.WarnLevel, entry.Level)
				assert.Equal(t, "warn message", entry.Message)
				assert.Equal(t, "value", entry.Data["custom_field"])
			},
		},
		{
			name:      "에러 포함",
			component: "test.component",
			level:     log.ErrorLevel,
			message:   "error message",
			fields:    nil,
			err:       errors.New("test error"),
			validate: func(t *testing.T, entry *log.Entry) {
				assert.Equal(t, log.ErrorLevel, entry.Level)
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
	Entries []*log.Entry
}

func NewMemoryHook() *MemoryHook {
	return &MemoryHook{}
}

func (h *MemoryHook) Levels() []log.Level {
	return log.AllLevels
}

func (h *MemoryHook) Fire(entry *log.Entry) error {
	h.Entries = append(h.Entries, entry)
	return nil
}

func (h *MemoryHook) Reset() {
	h.Entries = make([]*log.Entry, 0)
}

func (h *MemoryHook) LastEntry() *log.Entry {
	if len(h.Entries) == 0 {
		return nil
	}
	return h.Entries[len(h.Entries)-1]
}
