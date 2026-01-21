package contract

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskRunBy_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		trb  TaskRunBy
		want bool
	}{
		{"UserShouldBeValid", TaskRunByUser, true},
		{"SchedulerShouldBeValid", TaskRunByScheduler, true},
		{"UnknownShouldBeInvalid", TaskRunByUnknown, false},
		{"OutOfBoundsShouldBeInvalid", TaskRunBy(999), false},
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
		{"ValidUser", TaskRunByUser, false, ""},
		{"ValidScheduler", TaskRunByScheduler, false, ""},
		{"InvalidUnknown", TaskRunByUnknown, true, "지원하지 않는 실행 주체"},
		{"InvalidOutOfBounds", TaskRunBy(999), true, "지원하지 않는 실행 주체"},
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
		{"User", TaskRunByUser, "User"},
		{"Scheduler", TaskRunByScheduler, "Scheduler"},
		{"Unknown", TaskRunByUnknown, "Unknown"},
		{"Invalid", TaskRunBy(999), "Unknown"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.trb.String())
		})
	}
}

func TestTaskRunBy_JSON(t *testing.T) {
	t.Parallel()

	// Struct used to test embedding/unmarshaling
	type Container struct {
		RunBy TaskRunBy `json:"run_by"`
	}

	t.Run("Marshal", func(t *testing.T) {
		c := Container{RunBy: TaskRunByUser}
		data, err := json.Marshal(c)
		require.NoError(t, err)
		assert.JSONEq(t, `{"run_by": 1}`, string(data), "User should marshal to 1")

		c2 := Container{RunBy: TaskRunByScheduler}
		data2, err := json.Marshal(c2)
		require.NoError(t, err)
		assert.JSONEq(t, `{"run_by": 2}`, string(data2), "Scheduler should marshal to 2")
	})

	t.Run("Unmarshal_Valid", func(t *testing.T) {
		jsonStr := `{"run_by": 2}`
		var c Container
		err := json.Unmarshal([]byte(jsonStr), &c)
		require.NoError(t, err)
		assert.Equal(t, TaskRunByScheduler, c.RunBy)
		assert.NoError(t, c.RunBy.Validate())
	})

	t.Run("Unmarshal_InvalidValue", func(t *testing.T) {
		jsonStr := `{"run_by": 99}`
		var c Container
		err := json.Unmarshal([]byte(jsonStr), &c)
		require.NoError(t, err) // Unmarshal itself works for int
		assert.Equal(t, TaskRunBy(99), c.RunBy)
		assert.Error(t, c.RunBy.Validate(), "Should fail validation after unmarshaling invalid value")
	})

	t.Run("Unmarshal_TypeMismatch", func(t *testing.T) {
		jsonStr := `{"run_by": "User"}` // String instead of int
		var c Container
		err := json.Unmarshal([]byte(jsonStr), &c)
		assert.Error(t, err, "Should fail unmarshaling string to int enum")
	})
}
