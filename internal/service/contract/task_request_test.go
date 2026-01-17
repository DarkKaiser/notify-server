package contract

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskRunBy_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		trb  TaskRunBy
		want bool
	}{
		{
			name: "UserShouldBeValid",
			trb:  TaskRunByUser,
			want: true,
		},
		{
			name: "SchedulerShouldBeValid",
			trb:  TaskRunByScheduler,
			want: true,
		},
		{
			name: "UnknownShouldBeInvalid",
			trb:  TaskRunByUnknown,
			want: false,
		},
		{
			name: "OutOfBoundsShouldBeInvalid",
			trb:  TaskRunBy(999),
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.trb.IsValid())
		})
	}
}

func TestTaskRunBy_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		trb            TaskRunBy
		wantErr        bool
		errMsgContains string
	}{
		{
			name:    "ValidUser",
			trb:     TaskRunByUser,
			wantErr: false,
		},
		{
			name:    "ValidScheduler",
			trb:     TaskRunByScheduler,
			wantErr: false,
		},
		{
			name:           "InvalidUnknown",
			trb:            TaskRunByUnknown,
			wantErr:        true,
			errMsgContains: "지원하지 않는 실행 주체",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.trb.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsgContains != "" {
					assert.Contains(t, err.Error(), tt.errMsgContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTaskRunBy_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		trb  TaskRunBy
		want string
	}{
		{
			name: "User",
			trb:  TaskRunByUser,
			want: "User",
		},
		{
			name: "Scheduler",
			trb:  TaskRunByScheduler,
			want: "Scheduler",
		},
		{
			name: "Unknown",
			trb:  TaskRunByUnknown,
			want: "Unknown",
		},
		{
			name: "Invalid",
			trb:  TaskRunBy(999),
			want: "Unknown",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.trb.String())
		})
	}
}

func TestTaskSubmitRequest_Validate(t *testing.T) {
	t.Parallel()

	// Helper to create a valid base request
	validReq := func() *TaskSubmitRequest {
		return &TaskSubmitRequest{
			TaskID:      "VALID_TASK",
			CommandID:   "VALID_CMD",
			TaskContext: NewTaskContext(),
			RunBy:       TaskRunByUser,
		}
	}

	tests := []struct {
		name        string
		reqModifier func(*TaskSubmitRequest)
		wantErr     bool
		errMsgPart  string
	}{
		{
			name:        "ValidRequest_Minimal",
			reqModifier: func(r *TaskSubmitRequest) {}, // No changes
			wantErr:     false,
		},
		{
			name: "ValidRequest_Full",
			reqModifier: func(r *TaskSubmitRequest) {
				r.NotifierID = "telegram-bot"
				r.NotifyOnStart = true
			},
			wantErr: false,
		},
		{
			name: "ValidRequest_WithWhiteSpaceNotifierID_ShouldError",
			// NotifierID logic: len > 0 checks for existence. If it exists but is empty after trim -> Error.
			reqModifier: func(r *TaskSubmitRequest) {
				r.NotifierID = "   "
			},
			wantErr:    true,
			errMsgPart: "NotifierID",
		},
		{
			name: "Invalid_MissingTaskID",
			reqModifier: func(r *TaskSubmitRequest) {
				r.TaskID = ""
			},
			wantErr:    true,
			errMsgPart: "TaskID",
		},
		{
			name: "Invalid_MissingCommandID",
			reqModifier: func(r *TaskSubmitRequest) {
				r.CommandID = ""
			},
			wantErr:    true,
			errMsgPart: "CommandID",
		},
		{
			name: "Invalid_NilTaskContext",
			reqModifier: func(r *TaskSubmitRequest) {
				r.TaskContext = nil
			},
			wantErr:    true,
			errMsgPart: "TaskContext",
		},
		{
			name: "Invalid_RunBy",
			reqModifier: func(r *TaskSubmitRequest) {
				r.RunBy = TaskRunByUnknown
			},
			wantErr:    true,
			errMsgPart: "지원하지 않는 실행 주체",
		},
		{
			name: "Invalid_TaskID_Whitespace",
			reqModifier: func(r *TaskSubmitRequest) {
				r.TaskID = "   "
			},
			wantErr:    true,
			errMsgPart: "TaskID",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := validReq()
			tt.reqModifier(req)

			err := req.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsgPart != "" {
					assert.Contains(t, err.Error(), tt.errMsgPart)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
