package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunBy_String(t *testing.T) {
	tests := []struct {
		name     string
		runBy    RunBy
		expected string
	}{
		{
			name:     "User",
			runBy:    RunByUser,
			expected: "User",
		},
		{
			name:     "Scheduler",
			runBy:    RunByScheduler,
			expected: "Scheduler",
		},
		{
			name:     "Unknown",
			runBy:    RunBy(999),
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.runBy.String())
		})
	}
}

func TestTaskCommandID_Match(t *testing.T) {
	cases := []struct {
		name           string
		pattern        CommandID
		target         CommandID
		expectedResult bool
	}{
		{
			name:           "Exact Match",
			pattern:        "WatchPrice",
			target:         "WatchPrice",
			expectedResult: true,
		},
		{
			name:           "Exact Mismatch",
			pattern:        "WatchPrice",
			target:         "WatchStock",
			expectedResult: false,
		},
		{
			name:           "Wildcard Match",
			pattern:        "WatchPrice_*",
			target:         "WatchPrice_Product1",
			expectedResult: true,
		},
		{
			name:           "Wildcard Mismatch",
			pattern:        "WatchPrice_*",
			target:         "WatchStock_Product1",
			expectedResult: false,
		},
		{
			name:           "Wildcard Short Target",
			pattern:        "WatchPrice_*",
			target:         "Watch",
			expectedResult: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.expectedResult, c.pattern.Match(c.target))
		})
	}
}

func TestRunBy_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		runBy    RunBy
		expected bool
	}{
		{
			name:     "User",
			runBy:    RunByUser,
			expected: true,
		},
		{
			name:     "Scheduler",
			runBy:    RunByScheduler,
			expected: true,
		},
		{
			name:     "Unknown",
			runBy:    RunByUnknown,
			expected: false,
		},
		{
			name:     "Invalid",
			runBy:    RunBy(999),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.runBy.IsValid())
		})
	}
}

func TestRunRequest_Validate(t *testing.T) {
	tests := []struct {
		name        string
		req         *RunRequest
		expectedErr bool
	}{
		{
			name: "Valid Request",
			req: &RunRequest{
				TaskID:        "TASK-1",
				TaskCommandID: "CMD-1",
				RunBy:         RunByUser,
			},
			expectedErr: false,
		},
		{
			name: "Missing TaskID",
			req: &RunRequest{
				TaskID:        "",
				TaskCommandID: "CMD-1",
				RunBy:         RunByUser,
			},
			expectedErr: true,
		},
		{
			name: "Missing CommandID",
			req: &RunRequest{
				TaskID:        "TASK-1",
				TaskCommandID: "",
				RunBy:         RunByUser,
			},
			expectedErr: true,
		},
		{
			name: "Invalid RunBy",
			req: &RunRequest{
				TaskID:        "TASK-1",
				TaskCommandID: "CMD-1",
				RunBy:         RunByUnknown,
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
