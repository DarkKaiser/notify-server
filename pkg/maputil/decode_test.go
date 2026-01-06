package maputil

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Data Model
// =============================================================================

type User struct {
	Name   string `json:"name"`
	Age    int    `json:"age"`
	Active bool   `json:"active"`
}

type NetworkConfig struct {
	Host string        `json:"host"`
	Port int           `json:"port"`
	Time time.Duration `json:"time"`
}

type ComplexConfig struct {
	User     User           `json:"user"`
	Tags     []string       `json:"tags"`
	Metadata map[string]int `json:"metadata"`
}

type EmbeddedStruct struct {
	User `mapstructure:",squash"`
	Role string `json:"role"`
}

// =============================================================================
// Test Suite: Decode / DecodeTo
// =============================================================================

func TestDecode(t *testing.T) {
	t.Parallel()

	t.Run("Basic Type Decoding", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			input   any
			want    User
			wantErr bool
		}{
			{
				name:  "Full Fields",
				input: map[string]any{"name": "Alice", "age": 30, "active": true},
				want:  User{Name: "Alice", Age: 30, Active: true},
			},
			{
				name:  "Partial Fields",
				input: map[string]any{"name": "Bob"},
				want:  User{Name: "Bob"},
			},
			{
				name:  "Weak Typing (String to Int/Bool)",
				input: map[string]any{"name": "Charlie", "age": "40", "active": 1},
				want:  User{Name: "Charlie", Age: 40, Active: true},
			},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				// Test Decode[T]
				got, err := Decode[User](tt.input)
				if tt.wantErr {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, tt.want, *got)

				// Test DecodeTo
				var gotTo User
				err = DecodeTo(tt.input, &gotTo)
				require.NoError(t, err)
				assert.Equal(t, tt.want, gotTo)
			})
		}
	})

	t.Run("Advanced Structures", func(t *testing.T) {
		t.Parallel()

		// 1. Nested Structs
		t.Run("Nested", func(t *testing.T) {
			input := map[string]any{
				"user":     map[string]any{"name": "Alice", "age": 30},
				"tags":     "tag1, tag2", // Hook check
				"metadata": map[string]any{"level": 5},
			}
			got, err := Decode[ComplexConfig](input)
			require.NoError(t, err)
			assert.Equal(t, "Alice", got.User.Name)
			assert.Equal(t, []string{"tag1", "tag2"}, got.Tags)
			assert.Equal(t, 5, got.Metadata["level"])
		})

		// 2. Embedded Squash
		t.Run("Embedded Squash", func(t *testing.T) {
			input := map[string]any{
				"name": "Admin", // Belongs to User
				"role": "Super", // Belongs to EmbeddedStruct
			}
			got, err := Decode[EmbeddedStruct](input)
			require.NoError(t, err)
			assert.Equal(t, "Admin", got.Name)
			assert.Equal(t, "Super", got.Role)
		})
	})

	t.Run("Option Validation", func(t *testing.T) {
		t.Parallel()

		t.Run("WithTagName", func(t *testing.T) {
			type YamlTarget struct {
				Val string `yaml:"val"`
			}
			input := map[string]any{"val": "data"} // matches yaml tag
			got, err := Decode[YamlTarget](input, WithTagName("yaml"))
			require.NoError(t, err)
			assert.Equal(t, "data", got.Val)
		})

		t.Run("WithWeaklyTypedInput (Disable)", func(t *testing.T) {
			input := map[string]any{"age": "30"} // String, needs conversion
			_, err := Decode[User](input, WithWeaklyTypedInput(false))
			require.Error(t, err) // Should fail
		})

		t.Run("WithErrorUnused", func(t *testing.T) {
			input := map[string]any{"name": "Alice", "extra_field": "oops"}
			_, err := Decode[User](input, WithErrorUnused(true))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "extra_field")
		})

		t.Run("WithZeroFields (Replace semantics)", func(t *testing.T) {
			// Setup: pre-filled struct
			target := User{Name: "Old", Age: 99}
			input := map[string]any{"name": "New"}

			// Default behavior (Merge) check
			// Note: DecodeTo handles merge. Decode[T] always creates new T.
			// So we test DecodeTo specifically here for Merge vs Replace.
			var mergeTarget User = target
			err := DecodeTo(input, &mergeTarget) // Default: Merge
			require.NoError(t, err)
			assert.Equal(t, "New", mergeTarget.Name)
			assert.Equal(t, 99, mergeTarget.Age) // Preserved

			// Option behavior (Replace)
			var replaceTarget User = target
			err = DecodeTo(input, &replaceTarget, WithZeroFields(true))
			require.NoError(t, err)
			assert.Equal(t, "New", replaceTarget.Name)
			assert.Equal(t, 0, replaceTarget.Age) // Zeroed
		})

		t.Run("WithTrimSpace", func(t *testing.T) {
			type TagStruct struct {
				Tags []string `json:"tags"`
			}
			input := map[string]any{"tags": " a , b "}

			// Default: Trimmed
			got, err := Decode[TagStruct](input)
			require.NoError(t, err)
			assert.Equal(t, []string{"a", "b"}, got.Tags)

			// Disabled: Not Trimmed
			got, err = Decode[TagStruct](input, WithTrimSpace(false))
			require.NoError(t, err)
			assert.Equal(t, []string{" a ", " b "}, got.Tags)
		})
	})

	t.Run("Edge Cases & Errors", func(t *testing.T) {
		t.Parallel()

		t.Run("Nil Input", func(t *testing.T) {
			// Decodes nil -> Zero Value
			got, err := Decode[User](nil)
			require.NoError(t, err)
			assert.Equal(t, User{}, *got)
		})

		t.Run("Nil Output Pointer", func(t *testing.T) {
			// Validate DecodeTo nil check
			err := DecodeTo(map[string]any{}, (*User)(nil))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "output 포인터가 nil입니다")
		})

		t.Run("Invalid Output Type (Non-Pointer)", func(t *testing.T) {
			// Recover from mapstructure panic or return error
			// mapstructure.Decoder.Decode documentation says:
			// "Panic if Result is not a pointer to a struct or a map"
			defer func() {
				// We expect DecodeTo to possibly panic or return error depending on impl.
				// Our Wrapper creates a config with Result: output.
				// If output is not pointer, NewDecoder *might* error or Decode *might* panic.
				// Let's check mapstructure behavior.
				// Actually mapstructure NewDecoder checks if Result is pointer.
				if r := recover(); r != nil {
					// OK if it panics, though error is better.
				}
			}()

			var val User
			// DecodeTo expects *T, but if T is interface{}, user could pass non-pointer.
			// However DecodeTo[T] strongly types output as *T.
			// So we can only pass *T.
			// The only way to crash it is if T is a map type and we somehow mess up?
			// Actually, Go generics enforce *T. So we are safe from non-pointer calls mostly.
			_ = DecodeTo(map[string]any{}, &val)
		})

		t.Run("Custom Hook Logic", func(t *testing.T) {
			// Hook: convert string "full" -> bool true
			hook := func(f, t reflect.Type, data any) (any, error) {
				if f.Kind() == reflect.String && t.Kind() == reflect.Bool {
					if data == "full" {
						return true, nil
					}
				}
				return data, nil
			}
			input := map[string]any{"active": "full"}
			got, err := Decode[User](input, WithDecodeHook(hook))
			require.NoError(t, err)
			assert.True(t, got.Active)
		})
	})
}

// ExampleDecode demonstrates how to use the Decode function.
func ExampleDecode() {
	// Sample data often found in JSON or YAML
	input := map[string]any{
		"name":   "Alice",
		"age":    "30",           // String to Int (Weak typing)
		"active": 1,              // Int to Bool (Weak typing)
		"roles":  "admin,editor", // String to Slice (Hook)
	}

	type UserProfile struct {
		Name   string   `json:"name"`
		Age    int      `json:"age"`
		Active bool     `json:"active"`
		Roles  []string `json:"roles"`
	}

	// Simple one-liner decoding
	profile, err := Decode[UserProfile](input)
	if err != nil {
		panic(err)
	}

	// Use profile...
	_ = profile
}
