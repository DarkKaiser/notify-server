package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestID_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		id          ID
		expectedErr bool
	}{
		{
			name:        "Valid ID",
			id:          "TASK-1",
			expectedErr: false,
		},
		{
			name:        "Empty ID",
			id:          "",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.id.Validate()
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestID_IsEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		id       ID
		expected bool
	}{
		{
			name:     "Not Empty",
			id:       "TASK-1",
			expected: false,
		},
		{
			name:     "Empty",
			id:       "",
			expected: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.id.IsEmpty())
		})
	}
}

func TestCommandID_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		id          CommandID
		expectedErr bool
	}{
		{
			name:        "Valid CommandID",
			id:          "CMD-1",
			expectedErr: false,
		},
		{
			name:        "Empty CommandID",
			id:          "",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.id.Validate()
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCommandID_IsEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		id       CommandID
		expected bool
	}{
		{
			name:     "Not Empty",
			id:       "CMD-1",
			expected: false,
		},
		{
			name:     "Empty",
			id:       "",
			expected: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.id.IsEmpty())
		})
	}
}

func TestTaskCommandID_Match(t *testing.T) {
	t.Parallel()

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
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.expectedResult, c.pattern.Match(c.target))
		})
	}
}

func TestInstanceID_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		id          InstanceID
		expectedErr bool
	}{
		{
			name:        "Valid InstanceID",
			id:          "INST-1",
			expectedErr: false,
		},
		{
			name:        "Empty InstanceID",
			id:          "",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.id.Validate()
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInstanceID_IsEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		id       InstanceID
		expected bool
	}{
		{
			name:     "Not Empty",
			id:       "INST-1",
			expected: false,
		},
		{
			name:     "Empty",
			id:       "",
			expected: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.id.IsEmpty())
		})
	}
}

func TestRunBy_String(t *testing.T) {
	t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.runBy.String())
		})
	}
}

func TestRunBy_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		runBy       RunBy
		expectedErr bool
	}{
		{
			name:        "User",
			runBy:       RunByUser,
			expectedErr: false,
		},
		{
			name:        "Scheduler",
			runBy:       RunByScheduler,
			expectedErr: false,
		},
		{
			name:        "Unknown",
			runBy:       RunByUnknown,
			expectedErr: true,
		},
		{
			name:        "Invalid",
			runBy:       RunBy(999),
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.runBy.Validate()
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRunRequest_Validate(t *testing.T) {
	t.Parallel()

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
		{
			name: "Valid Request with NotifierID",
			req: &RunRequest{
				TaskID:        "TASK-1",
				TaskCommandID: "CMD-1",
				RunBy:         RunByUser,
				NotifierID:    "TEST-NOTIFIER",
			},
			expectedErr: false,
		},
		{
			name: "Invalid NotifierID (Whitespace)",
			req: &RunRequest{
				TaskID:        "TASK-1",
				TaskCommandID: "CMD-1",
				RunBy:         RunByUser,
				NotifierID:    "   ",
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.req.Validate()
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
