package contract

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskSubmitRequest_Validate(t *testing.T) {
	t.Parallel()

	// Helper to create a valid base request
	validReq := func() *TaskSubmitRequest {
		return &TaskSubmitRequest{
			TaskID:    "VALID_TASK",
			CommandID: "VALID_CMD",
			RunBy:     TaskRunByUser,
		}
	}

	tests := []struct {
		name        string
		reqModifier func(*TaskSubmitRequest)
		wantErr     bool
		errMsgPart  string
	}{
		// ---------------------------------------------------------------------
		// Success Cases
		// ---------------------------------------------------------------------
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
				r.RunBy = TaskRunByScheduler
			},
			wantErr: false,
		},

		// ---------------------------------------------------------------------
		// Failure Cases: Required Fields
		// ---------------------------------------------------------------------
		{
			name: "Invalid_MissingTaskID",
			reqModifier: func(r *TaskSubmitRequest) {
				r.TaskID = ""
			},
			wantErr:    true,
			errMsgPart: "TaskID",
		},
		{
			name: "Invalid_TaskID_Whitespace",
			reqModifier: func(r *TaskSubmitRequest) {
				r.TaskID = "   "
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
			name: "Invalid_RunBy_Unknown",
			reqModifier: func(r *TaskSubmitRequest) {
				r.RunBy = TaskRunByUnknown
			},
			wantErr:    true,
			errMsgPart: "지원하지 않는 실행 주체",
		},

		// ---------------------------------------------------------------------
		// Failure Cases: Optional Fields Validation
		// ---------------------------------------------------------------------
		{
			name: "Invalid_NotifierID_Whitespace",
			// NotifierID is optional, but if provided, it must not be just whitespace
			reqModifier: func(r *TaskSubmitRequest) {
				r.NotifierID = "   "
			},
			wantErr:    true,
			errMsgPart: "NotifierID",
		},
		{
			name: "Valid_NotifierID_Empty",
			// Empty string implies "Use Default", which is valid
			reqModifier: func(r *TaskSubmitRequest) {
				r.NotifierID = ""
			},
			wantErr: false,
		},
		{
			name: "Failure: TaskID/CommandID Validation Interop",
			// Check if TaskID validation error comes first or handled correctly
			reqModifier: func(r *TaskSubmitRequest) {
				r.TaskID = TaskID(strings.Repeat("A", 100)) // Allowing delegation check if any
				r.TaskID = ""                               // Force error
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
