package maputil

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Unit Tests - StringToSliceHookFunc
// =============================================================================

func TestHooks_StringToSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		trimSpace bool
		input     any
		target    any // slice pointer
		want      any // slice value
		wantErr   bool
	}{
		{
			name:      "Basic CSV Splitting",
			trimSpace: true,
			input:     "a,b,c",
			target:    &[]string{},
			want:      []string{"a", "b", "c"},
		},
		{
			name:      "Whitespace Trimming Enabled",
			trimSpace: true,
			input:     " a , b , c ",
			target:    &[]string{},
			want:      []string{"a", "b", "c"},
		},
		{
			name:      "Whitespace Trimming Disabled",
			trimSpace: false,
			input:     " a , b , c ",
			target:    &[]string{},
			want:      []string{" a ", " b ", " c "},
		},
		{
			name:      "Empty String",
			trimSpace: true,
			input:     "",
			target:    &[]string{},
			want:      []string{},
		},
		{
			name:      "Integer CSV (Weakly Typed)",
			trimSpace: true,
			input:     "1, 2, 3",
			target:    &[]int{},
			want:      []int{1, 2, 3}, // mapstructure handles string->int conversion after split
		},
		{
			name:      "Ignore Non-String Input",
			trimSpace: true,
			input:     123,
			target:    &[]string{},
			want:      123, // Hook should pass through non-string input
		},
		{
			name:      "Preserve Byte Slice (Security)",
			trimSpace: true,
			input:     "data",
			target:    &[]byte{},
			want:      "data", // Hook should NOT split []byte target, pass through
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Manually invoke hook for unit testing logic
			hook := stringToSliceHookFunc(tt.trimSpace)
			hookFunc := hook.(func(reflect.Type, reflect.Type, any) (any, error))

			// Determine target type
			targetType := reflect.TypeOf(tt.target)
			if targetType.Kind() == reflect.Ptr {
				targetType = targetType.Elem()
			}

			got, err := hookFunc(reflect.TypeOf(tt.input), targetType, tt.input)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.name == "Ignore Non-String Input" || tt.name == "Preserve Byte Slice (Security)" {
					// Input passed through unmodified
					assert.Equal(t, tt.input, got)
				} else {
					if slice, ok := got.([]string); ok {
						// For string comparison
						if wantStringSlice, ok2 := tt.want.([]string); ok2 {
							assert.Equal(t, wantStringSlice, slice)
							return
						}
						// For int test, we expect []string from hook, then mapstructure converts.
						if tt.name == "Integer CSV (Weakly Typed)" {
							assert.Equal(t, []string{"1", "2", "3"}, slice)
							return
						}
					}
					// Fallback
					assert.Equal(t, tt.want, got)
				}
			}
		})
	}
}

// =============================================================================
// Unit Tests - StringToDurationHookFunc
// =============================================================================

func TestHooks_StringToDuration(t *testing.T) {
	t.Parallel()

	type MyDuration time.Duration

	tests := []struct {
		name    string
		input   any
		target  any // target TYPE check
		want    any
		wantErr bool
	}{
		{
			name:   "Valid Duration",
			input:  "10s",
			target: time.Duration(0),
			want:   10 * time.Second,
		},
		{
			name:   "Duration with Whitespace",
			input:  " 5m ",
			target: time.Duration(0),
			want:   5 * time.Minute,
		},
		{
			name:   "Invalid Format (Pass Through)",
			input:  "invalid",
			target: time.Duration(0),
			want:   "invalid", // Hook returns nil err on parse fail, passing data through
		},
		{
			name:   "Alias Type (Current Strict Behavior)",
			input:  "10s",
			target: MyDuration(0),
			want:   "10s", // Hook strictly checks type, ignores Alias -> Pass through
		},
		{
			name:   "Ignore Non-String",
			input:  123,
			target: time.Duration(0),
			want:   123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := stringToDurationHookFunc()
			hookFunc := hook.(func(reflect.Type, reflect.Type, any) (any, error))

			targetType := reflect.TypeOf(tt.target)
			got, err := hookFunc(reflect.TypeOf(tt.input), targetType, tt.input)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// =============================================================================
// Unit Tests - StringToBytesHookFunc
// =============================================================================

func TestHooks_StringToBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   any
		target  any
		want    any
		wantErr bool
	}{
		{
			name:   "Plain String (UTF-8)",
			input:  "hello",
			target: []byte{},
			want:   []byte("hello"),
		},
		{
			name:   "Base64 Prefix Valid",
			input:  "base64:aGVsbG8=", // "hello"
			target: []byte{},
			want:   []byte("hello"),
		},
		{
			name:    "Base64 Prefix Invalid",
			input:   "base64:INVALID!!!",
			target:  []byte{},
			want:    nil,
			wantErr: true,
		},
		{
			name:   "No Prefix (Ambiguous) -> Treat as Plain",
			input:  "aGVsbG8=", // Looks like base64 but no prefix
			target: []byte{},
			want:   []byte("aGVsbG8="),
		},
		{
			name:   "Target is Array",
			input:  "1234",
			target: [4]byte{},
			want:   []byte("1234"), // Hook returns slice, mapstructure copies to array
		},
		{
			name:   "Ignore Non-Byte Slice Target",
			input:  "hello",
			target: []string{},
			want:   "hello", // Pass through
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := stringToBytesHookFunc()
			hookFunc := hook.(func(reflect.Type, reflect.Type, any) (any, error))

			targetType := reflect.TypeOf(tt.target)
			got, err := hookFunc(reflect.TypeOf(tt.input), targetType, tt.input)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// =============================================================================
// Integration Tests (DecodeTo with Hooks)
// =============================================================================

// Named string type for robust type assertion testing
type MyString string

func TestHooks_Integration(t *testing.T) {
	t.Parallel()

	t.Run("Safe String Type Assertion", func(t *testing.T) {
		// Ensures hooks don't panic on Named String types (MyString)
		input := map[string]any{
			"tags": MyString("a,b"),
		}
		type Target struct {
			Tags []string `json:"tags"`
		}
		var got Target
		err := DecodeTo(input, &got)
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, got.Tags)
	})

	t.Run("Base64 Decode Error Propagation", func(t *testing.T) {
		input := map[string]any{
			"data": "base64:Broken",
		}
		type Target struct {
			Data []byte `json:"data"`
		}
		var got Target
		err := DecodeTo(input, &got)
		require.Error(t, err)

		assert.Contains(t, err.Error(), "base64", "Error message should mention base64 failure")

		// Mapstructure errors are often wrapped, but let's just check containing text as it's safer.
	})
}
