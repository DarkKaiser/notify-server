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
		target    any // Slice pointer expected by mapstructure
		want      any // Expected result in the slice
		wantErr   bool
	}{
		// ---------------------------------------------------------------------
		// Happy Paths
		// ---------------------------------------------------------------------
		{
			name:      "Standard CSV",
			trimSpace: true,
			input:     "apple,banana,cherry",
			target:    &[]string{},
			want:      []string{"apple", "banana", "cherry"},
		},
		{
			name:      "With Whitespace - Trimmed",
			trimSpace: true,
			input:     "  apple ,  banana  , cherry  ",
			target:    &[]string{},
			want:      []string{"apple", "banana", "cherry"},
		},
		{
			name:      "With Whitespace - Untrimmed",
			trimSpace: false,
			input:     "  apple ,  banana  , cherry  ",
			target:    &[]string{},
			want:      []string{"  apple ", "  banana  ", " cherry  "},
		},
		{
			name:      "Multi-line String (Environment Variable Sim)",
			trimSpace: true,
			input: `apple
banana
cherry`,
			target: &[]string{},
			want:   []string{"apple", "banana", "cherry"},
		},
		{
			name:      "Single Value",
			trimSpace: true,
			input:     "apple",
			target:    &[]string{},
			want:      []string{"apple"},
		},
		{
			name:      "Empty String",
			trimSpace: true,
			input:     "",
			target:    &[]string{},
			want:      []string{},
		},
		{
			name:      "Whitespace Only String",
			trimSpace: true,
			input:     "   ",
			target:    &[]string{},
			want:      []string{}, // Split on empty string returns []
		},
		{
			name:      "Numeric CSV (Int Slice)",
			trimSpace: true,
			input:     "1, 20, 300",
			target:    &[]int{},
			// Hook splits to []string, mapstructure converts to []int later
			want: []string{"1", "20", "300"},
		},
		// ---------------------------------------------------------------------
		// CSV Edge Cases
		// ---------------------------------------------------------------------
		{
			name:      "Quoted String (CSS Selector)",
			trimSpace: true,
			input:     `"div.a, div.b", span.c`,
			target:    &[]string{},
			want:      []string{"div.a, div.b", "span.c"},
		},
		{
			name:      "Escaped Quotes",
			trimSpace: true,
			input:     `"Foo ""Bar"" Baz", Qux`,
			target:    &[]string{},
			want:      []string{`Foo "Bar" Baz`, "Qux"},
		},
		{
			name:      "Trailing Comma",
			trimSpace: true,
			input:     "a,b,",
			target:    &[]string{},
			want:      []string{"a", "b", ""},
		},
		// ---------------------------------------------------------------------
		// Ignored Cases (Pass-through)
		// ---------------------------------------------------------------------
		{
			name:      "Non-String Input",
			trimSpace: true,
			input:     12345,
			target:    &[]string{},
			want:      12345,
		},
		{
			name:      "Byte Slice Target (Security)",
			trimSpace: true,
			input:     "some_data",
			target:    &[]byte{},
			want:      "some_data", // Should not split []byte
		},
		{
			name:      "Byte Array Target (Security)",
			trimSpace: true,
			input:     "1234",
			target:    &[4]byte{},
			want:      "1234",
		},
		{
			name:      "Non-Slice Target",
			trimSpace: true,
			input:     "a,b",
			target:    &struct{}{}, // Not a slice
			want:      "a,b",
		},
		// ---------------------------------------------------------------------
		// Failure Cases
		// ---------------------------------------------------------------------
		{
			name:      "Unclosed Quote Error",
			trimSpace: true,
			input:     `"open quote`,
			target:    &[]string{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create the hook
			hook := stringToSliceHookFunc(tt.trimSpace)
			hookFunc := hook.(func(reflect.Type, reflect.Type, any) (any, error))

			// Prepare types
			inputType := reflect.TypeOf(tt.input)

			// Handle pointer to slice vs slice type
			targetType := reflect.TypeOf(tt.target)
			if targetType.Kind() == reflect.Ptr {
				targetType = targetType.Elem()
			}

			// Execute Hook
			got, err := hookFunc(inputType, targetType, tt.input)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Assertions
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// Unit Tests - StringToDurationHookFunc
// =============================================================================

func TestHooks_StringToDuration(t *testing.T) {
	t.Parallel()

	type CustomDuration time.Duration

	tests := []struct {
		name    string
		input   any
		target  any // instance of target type
		want    any // expected return
		wantErr bool
	}{
		// ---------------------------------------------------------------------
		// Happy Paths
		// ---------------------------------------------------------------------
		{
			name:   "Standard Duration",
			input:  "10s",
			target: time.Duration(0),
			want:   10 * time.Second,
		},
		{
			name:   "Zero Duration",
			input:  "0",
			target: time.Duration(0),
			want:   time.Duration(0),
		},
		{
			name:   "Zero Duration (Unit)",
			input:  "0s",
			target: time.Duration(0),
			want:   time.Duration(0),
		},
		{
			name:   "Negative Duration",
			input:  "-5m",
			target: time.Duration(0),
			want:   -5 * time.Minute,
		},
		{
			name:   "Fractional Duration",
			input:  "1.5h",
			target: time.Duration(0),
			want:   90 * time.Minute,
		},
		{
			name:   "Duration with Whitespace - Trimmed",
			input:  "  5m  ",
			target: time.Duration(0),
			want:   5 * time.Minute,
		},
		// ---------------------------------------------------------------------
		// Ignored Cases
		// ---------------------------------------------------------------------
		{
			name:   "Ignored: Invalid Format (Pass-through)",
			input:  "invalid-time",
			target: time.Duration(0),
			want:   "invalid-time", // Hook returns nil error, passes data along
		},
		{
			name:   "Ignored: Custom Alias Type",
			input:  "10s",
			target: CustomDuration(0), // strict check should ignore this
			want:   "10s",
		},
		{
			name:   "Ignored: Non-String Input",
			input:  123,
			target: time.Duration(0),
			want:   123,
		},
		{
			name:   "Ignored: Non-Duration Target (int64)",
			input:  "10s",
			target: int64(0),
			want:   "10s",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hook := stringToDurationHookFunc()
			hookFunc := hook.(func(reflect.Type, reflect.Type, any) (any, error))

			inputType := reflect.TypeOf(tt.input)
			targetType := reflect.TypeOf(tt.target)

			got, err := hookFunc(inputType, targetType, tt.input)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// Unit Tests - StringToBytesHookFunc
// =============================================================================

func TestHooks_StringToBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     any
		target    any
		want      any
		wantErr   bool
		errMatch  string
		trimSpace bool
	}{
		// ---------------------------------------------------------------------
		// Base64 Decoding
		// ---------------------------------------------------------------------
		{
			name:   "Base64 Prefix - Standard",
			input:  "base64:SGVsbG8=", // "Hello"
			target: []byte{},
			want:   []byte("Hello"),
		},
		{
			name:   "Base64 Prefix - With Whitespace (Internal Trim)",
			input:  "  base64:SGVsbG8=  ",
			target: []byte{},
			want:   []byte("Hello"), // Logic: Trims -> Checks Prefix -> Decodes payload
		},
		{
			name:      "Base64 Prefix - With TrimSpace=False",
			input:     "  base64:SGVsbG8=  ",
			target:    []byte{},
			want:      []byte("Hello"), // Even with trimSpace=false, base64 logic should handle prefix check robustly
			trimSpace: false,
		},
		{
			name:     "Base64 Prefix - Invalid Content",
			input:    "base64:!!!INVALID!!!",
			target:   []byte{},
			wantErr:  true,
			errMatch: "base64",
		},
		{
			name:     "Double Prefix (Invalid Content)",
			input:    "base64:base64:SGVsbG8=", // Payload "base64:..." has invalid char ':'
			target:   []byte{},
			wantErr:  true,
			errMatch: "illegal base64 data",
		},
		// ---------------------------------------------------------------------
		// Raw String Conversion
		// ---------------------------------------------------------------------
		{
			name:      "With Whitespace - Untrimmed",
			input:     "  val  ",
			target:    []byte{},
			want:      []byte("  val  "),
			trimSpace: false,
		},
		{
			name:   "Standard String",
			input:  "hello world",
			target: []byte{},
			want:   []byte("hello world"),
		},
		{
			name:   "No Prefix (Ambiguous) - Treat as String",
			input:  "SGVsbG8=", // Looks like base64 but missing prefix
			target: []byte{},
			want:   []byte("SGVsbG8="),
		},
		{
			name:   "Case Sensitivity - BASE64:",
			input:  "BASE64:SGVsbG8=",
			target: []byte{},
			want:   []byte("BASE64:SGVsbG8="), // Prefix must be lowercase "base64:"
		},
		{
			name:   "Target Array [N]byte",
			input:  "1234",
			target: [4]byte{},
			want:   []byte("1234"), // Hook returns slice, structure adapts
		},
		// ---------------------------------------------------------------------
		// Ignored Cases
		// ---------------------------------------------------------------------
		{
			name:   "Ignored: Non-String Input",
			input:  123,
			target: []byte{},
			want:   123,
		},
		{
			name:   "Ignored: Non-ByteSlice Target",
			input:  "base64:SGVsbG8=",
			target: []string{},
			want:   "base64:SGVsbG8=",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hook := stringToBytesHookFunc(tt.trimSpace)
			hookFunc := hook.(func(reflect.Type, reflect.Type, any) (any, error))

			inputType := reflect.TypeOf(tt.input)
			targetType := reflect.TypeOf(tt.target)

			got, err := hookFunc(inputType, targetType, tt.input)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMatch != "" {
					assert.Contains(t, err.Error(), tt.errMatch)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
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
		assert.Contains(t, err.Error(), "illegal base64 data")
	})

	t.Run("Byte Slice TrimSpace Support", func(t *testing.T) {
		// Verifies that WithTrimSpace(false) is correctly propagated to []byte decoding
		input := map[string]any{
			"bytes": "  val  ",
		}
		type Target struct {
			Bytes []byte `json:"bytes"`
		}

		// Case 1: Default (TrimSpace = true)
		var got1 Target
		err1 := DecodeTo(input, &got1)
		require.NoError(t, err1)
		assert.Equal(t, []byte("val"), got1.Bytes, "Default should trim spaces")

		// Case 2: Explicit TrimSpace = false
		var got2 Target
		err2 := DecodeTo(input, &got2, WithTrimSpace(false))
		require.NoError(t, err2)
		assert.Equal(t, []byte("  val  "), got2.Bytes, "WithTrimSpace(false) should preserve spaces")
	})
}

// =============================================================================
// Regression Tests
// =============================================================================

func TestFix_DurationHookScope(t *testing.T) {
	t.Parallel()

	// 1. Regression: Int64 field should NOT be hijacked by Duration hook
	t.Run("Regression: Int64 field safety", func(t *testing.T) {
		type Data struct {
			Count int64 `json:"count"`
		}
		// Input is "10s", which is invalid for a plain int64
		input := map[string]any{
			"count": "10s",
		}

		var target Data
		err := DecodeTo(input, &target)

		require.Error(t, err, "Should fail to decode '10s' into int64 without the duration hook")
		assert.Contains(t, err.Error(), "parsing \"10s\": invalid syntax")
	})

	// 2. Verification: time.Duration field SHOULD be handled by Duration hook
	t.Run("Verification: time.Duration field support", func(t *testing.T) {
		type Config struct {
			Timeout time.Duration `json:"timeout"`
		}
		input := map[string]any{
			"timeout": "10s",
		}

		var target Config
		err := DecodeTo(input, &target)
		require.NoError(t, err)
		assert.Equal(t, 10*time.Second, target.Timeout)
	})

	// 3. Verification: duration alias mismatch (Strict check)
	t.Run("Verification: duration alias strict check", func(t *testing.T) {
		type MyDuration time.Duration
		type Wrapper struct {
			D MyDuration `json:"d"`
		}
		input := map[string]any{"d": "10s"}

		var target Wrapper
		err := DecodeTo(input, &target)

		// Strict check means alias is not handled by the hook
		// And since string "10s" cannot be directly assigned to int64 based type, it errors.
		require.Error(t, err)
	})
}
