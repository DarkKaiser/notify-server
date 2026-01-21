package contract

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewNotification(t *testing.T) {
	t.Parallel()

	message := "Test Message"
	n := NewNotification(message)

	assert.Equal(t, message, n.Message)
	assert.False(t, n.ErrorOccurred, "Default notification should not be an error")
	assert.Empty(t, n.NotifierID, "Default notifier ID should be empty")
	assert.Empty(t, n.TaskID)
	assert.False(t, n.Cancelable)
}

func TestNewErrorNotification(t *testing.T) {
	t.Parallel()

	message := "Error Message"
	n := NewErrorNotification(message)

	assert.Equal(t, message, n.Message)
	assert.True(t, n.ErrorOccurred, "ErrorNotification must set ErrorOccurred to true")
	assert.Empty(t, n.NotifierID)
	assert.Empty(t, n.TaskID)
}

func TestNewTaskNotification(t *testing.T) {
	t.Parallel()

	// Given
	notifierID := NotifierID("telegram")
	taskID := TaskID("TEST_TASK")
	commandID := TaskCommandID("CMD_CHECK")
	instanceID := TaskInstanceID("INST_UUID")
	message := "Task Completed"
	elapsed := 500 * time.Millisecond
	isError := true
	isCancelable := true

	// When
	n := NewTaskNotification(
		notifierID,
		taskID,
		commandID,
		instanceID,
		message,
		elapsed,
		isError,
		isCancelable,
	)

	// Then
	assert.Equal(t, notifierID, n.NotifierID)
	assert.Equal(t, taskID, n.TaskID)
	assert.Equal(t, commandID, n.CommandID)
	assert.Equal(t, instanceID, n.InstanceID)
	assert.Equal(t, message, n.Message)
	assert.Equal(t, elapsed, n.ElapsedTime)
	assert.Equal(t, isError, n.ErrorOccurred)
	assert.Equal(t, isCancelable, n.Cancelable)
}

func TestNotification_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		message       string
		expectedError error
	}{
		{
			name:          "Valid message",
			message:       "Hello World",
			expectedError: nil,
		},
		{
			name:          "Empty message",
			message:       "",
			expectedError: ErrMessageRequired,
		},
		{
			name:          "Message with only spaces",
			message:       "      ",
			expectedError: ErrMessageRequired,
		},
		{
			name:          "Message with only tabs/newlines",
			message:       "\t\n",
			expectedError: ErrMessageRequired,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := NewNotification(tt.message)
			err := n.Validate()

			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNotification_String(t *testing.T) {
	t.Parallel()

	longMessage := strings.Repeat("A", 100)
	truncatedPreview := strings.Repeat("A", 47) + "..."

	tests := []struct {
		name         string
		notification Notification
		wantContains []string
		wantMissing  []string
	}{
		{
			name: "Full context notification",
			notification: NewTaskNotification(
				"slack",
				"TASK",
				"CMD",
				"INST",
				"Msg",
				time.Second,
				true,
				true,
			),
			wantContains: []string{
				"notifier=slack",
				"task=TASK/CMD",
				"instance=INST",
				"error=true",
				"cancelable=true",
				`msg="Msg"`,
			},
		},
		{
			name:         "Minimal notification (Defaults)",
			notification: NewNotification("Simple"),
			wantContains: []string{
				"notifier=default",
				`msg="Simple"`,
			},
			wantMissing: []string{
				"task=",
				"instance=",
				"error=true",
				"cancelable=true",
			},
		},
		{
			name: "Truncated message",
			notification: Notification{
				Message: longMessage,
			},
			wantContains: []string{
				fmt.Sprintf("msg=%q", truncatedPreview),
			},
		},
		{
			name: "Special characters in message",
			notification: Notification{
				Message: "Hello\nWorld\"",
			},
			wantContains: []string{
				// Go의 %q 포맷팅은 특수문자를 이스케이프합니다.
				`msg="Hello\nWorld\""`,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.notification.String()

			for _, want := range tt.wantContains {
				assert.Contains(t, got, want, "Log string should contain expected field")
			}

			for _, missing := range tt.wantMissing {
				assert.NotContains(t, got, missing, "Log string should NOT contain unexpected field")
			}
		})
	}
}
