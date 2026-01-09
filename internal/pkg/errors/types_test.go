package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestErrorType_String tests the String method of ErrorType.
func TestErrorType_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		errType  ErrorType
		expected string
	}{
		{"Unknown", Unknown, "Unknown"},
		{"Internal", Internal, "Internal"},
		{"System", System, "System"},
		{"Unauthorized", Unauthorized, "Unauthorized"},
		{"Forbidden", Forbidden, "Forbidden"},
		{"InvalidInput", InvalidInput, "InvalidInput"},
		{"Conflict", Conflict, "Conflict"},
		{"NotFound", NotFound, "NotFound"},
		{"ExecutionFailed", ExecutionFailed, "ExecutionFailed"},
		{"Timeout", Timeout, "Timeout"},
		{"Unavailable", Unavailable, "Unavailable"},
		{"Invalid(999)", ErrorType(999), "ErrorType(999)"}, // Edge case for undefined value
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.errType.String())
		})
	}
}

// TestErrorType_Error tests the Error method of ErrorType.
func TestErrorType_Error(t *testing.T) {
	t.Parallel()

	// Verify ErrorType implements the error interface
	var _ error = ErrorType(0)

	tests := []struct {
		name     string
		errType  ErrorType
		expected string
	}{
		{"Unknown", Unknown, "Unknown"},
		{"Internal", Internal, "Internal"},
		{"NotFound", NotFound, "NotFound"},
		{"Invalid Value", ErrorType(-1), "ErrorType(-1)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Error() should return the same result as String()
			assert.Equal(t, tt.expected, tt.errType.Error())
			assert.Equal(t, tt.errType.String(), tt.errType.Error())
		})
	}
}

// TestErrorType_Compatiblity validates that ErrorType constants are distinct.
func TestErrorType_Distinct(t *testing.T) {
	t.Parallel()

	types := []ErrorType{
		Unknown, Internal, System, Unauthorized, Forbidden,
		InvalidInput, Conflict, NotFound, ExecutionFailed,
		Timeout, Unavailable,
	}

	// Ensure all defined types are distinct
	seen := make(map[ErrorType]bool)
	for _, et := range types {
		if seen[et] {
			t.Errorf("Duplicate ErrorType found: %v", et)
		}
		seen[et] = true
	}
}

// ExampleErrorType_Error demonstrates how ErrorType works as an error.
func ExampleErrorType_Error() {
	var err error = NotFound
	fmt.Println(err.Error())
	// Output: NotFound
}
