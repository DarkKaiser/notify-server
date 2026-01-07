package maputil

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Data Models
// =============================================================================

type User struct {
	Name   string `json:"name"`
	Age    int    `json:"age"`
	Active bool   `json:"active"`
}

type NestedConfig struct {
	Database struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"database"`
}

type EmbeddedStruct struct {
	User `mapstructure:",squash"`
	Role string `json:"role"`
}

type SliceConfig struct {
	Tags []string `json:"tags"`
}

// =============================================================================
// Test Suite: Decode / DecodeTo
// =============================================================================

func TestDecode_Basic(t *testing.T) {
	t.Parallel()

	t.Run("Primitive Types", func(t *testing.T) {
		t.Parallel()
		input := map[string]any{"name": "Alice", "age": 30, "active": true}
		got, err := Decode[User](input)
		require.NoError(t, err)
		assert.Equal(t, User{Name: "Alice", Age: 30, Active: true}, *got)
	})

	t.Run("Nil Input -> Zero Value", func(t *testing.T) {
		t.Parallel()
		got, err := Decode[User](nil)
		require.NoError(t, err)
		assert.Equal(t, User{}, *got)
	})

	t.Run("Partial Fields", func(t *testing.T) {
		t.Parallel()
		input := map[string]any{"name": "Bob"}
		got, err := Decode[User](input)
		require.NoError(t, err)
		assert.Equal(t, User{Name: "Bob"}, *got)
	})
}

func TestDecode_Options(t *testing.T) {
	t.Parallel()

	// 1. WithTagName
	t.Run("WithTagName", func(t *testing.T) {
		type YamlTarget struct {
			Val string `yaml:"val"`
		}
		input := map[string]any{"val": "data"}
		got, err := Decode[YamlTarget](input, WithTagName("yaml"))
		require.NoError(t, err)
		assert.Equal(t, "data", got.Val)
	})

	// 2. WithWeaklyTypedInput
	t.Run("WithWeaklyTypedInput", func(t *testing.T) {
		input := map[string]any{
			"age":    "40", // string -> int
			"active": 1,    // int -> bool
		}

		// Default: Enabled
		got, err := Decode[User](input)
		require.NoError(t, err)
		assert.Equal(t, 40, got.Age)
		assert.True(t, got.Active)

		// Disabled
		_, err = Decode[User](input, WithWeaklyTypedInput(false))
		require.Error(t, err)
	})

	// 3. WithZeroFields (Replace vs Merge)
	t.Run("WithZeroFields", func(t *testing.T) {
		// Setup: Pre-filled struct
		target := User{Name: "Old", Age: 99}
		input := map[string]any{"name": "New"}

		// Default: Merge (fields not in input are preserved)
		var mergeTarget User = target
		err := DecodeTo(input, &mergeTarget)
		require.NoError(t, err)
		assert.Equal(t, "New", mergeTarget.Name)
		assert.Equal(t, 99, mergeTarget.Age)

		// Option: Replace (struct is zeroed before decode)
		var replaceTarget User = target
		err = DecodeTo(input, &replaceTarget, WithZeroFields(true))
		require.NoError(t, err)
		assert.Equal(t, "New", replaceTarget.Name)
		assert.Equal(t, 0, replaceTarget.Age)
	})

	// 4. WithSquash
	t.Run("WithSquash", func(t *testing.T) {
		input := map[string]any{
			"name": "Admin",
			"role": "Super",
		}

		// Default: Squash Enabled
		got, err := Decode[EmbeddedStruct](input)
		require.NoError(t, err)
		assert.Equal(t, "Admin", got.Name)
		assert.Equal(t, "Super", got.Role)

		// Disabled
		// Without squash, mapstructure expects "User": map[string]any{"name": ...}
		// Since input is flat, it won't find "User" key and nested fields remain empty.
		got2, err := Decode[EmbeddedStruct](input, WithSquash(false))
		require.NoError(t, err)
		assert.Equal(t, "", got2.Name)
		assert.Equal(t, "Super", got2.Role)
	})

	// 5. WithErrorUnused
	t.Run("WithErrorUnused", func(t *testing.T) {
		input := map[string]any{"name": "Alice", "unexpected": "value"}

		// Default: Ignored (Success)
		_, err := Decode[User](input)
		require.NoError(t, err)

		// Enabled: Error
		_, err = Decode[User](input, WithErrorUnused(true))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected")
	})

	// 6. WithTrimSpace
	t.Run("WithTrimSpace", func(t *testing.T) {
		input := map[string]any{"tags": " a , b "}

		// Default: Trimmed
		got, err := Decode[SliceConfig](input)
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, got.Tags)

		// Disabled: Not Trimmed
		got2, err := Decode[SliceConfig](input, WithTrimSpace(false))
		require.NoError(t, err)
		assert.Equal(t, []string{" a ", " b "}, got2.Tags)
	})

	// 7. WithDecodeHook (Custom Hook Priority)
	t.Run("WithDecodeHook", func(t *testing.T) {
		// Custom hook to convert string "admin" to age 100
		hook := func(f, t reflect.Type, data any) (any, error) {
			if f.Kind() == reflect.String && t.Kind() == reflect.Int {
				if data.(string) == "admin" {
					return 100, nil
				}
			}
			return data, nil
		}
		input := map[string]any{"age": "admin"}

		got, err := Decode[User](input, WithDecodeHook(hook))
		require.NoError(t, err)
		assert.Equal(t, 100, got.Age)
	})
}

func TestDecode_Metadata(t *testing.T) {
	t.Parallel()

	t.Run("Metadata Collection", func(t *testing.T) {
		input := map[string]any{
			"name":   "Alice", // Used
			"unused": "foo",   // Unused
		}
		var md mapstructure.Metadata
		_, err := Decode[User](input, WithMetadata(&md))
		require.NoError(t, err)

		assert.Contains(t, md.Keys, "name")
		assert.Contains(t, md.Unused, "unused")
	})

	t.Run("Custom MatchName", func(t *testing.T) {
		// Custom matcher: ignore underscores
		matcher := func(key, field string) bool {
			return strings.ReplaceAll(key, "_", "") == strings.ToLower(field)
		}
		input := map[string]any{"n_a_m_e": "Alice"}

		got, err := Decode[User](input, WithMatchName(matcher))
		require.NoError(t, err)
		assert.Equal(t, "Alice", got.Name)
	})
}

func TestDecode_Errors(t *testing.T) {
	t.Parallel()

	t.Run("Nil Output Pointer", func(t *testing.T) {
		err := DecodeTo(map[string]any{}, (*User)(nil))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "초기화되지 않았습니다")
	})

	t.Run("Invalid Type Conversion", func(t *testing.T) {
		input := map[string]any{"age": "not-a-number"}
		_, err := Decode[User](input)
		require.Error(t, err)
		// Should contain type mismatch details
		assert.Contains(t, err.Error(), "cannot parse")
	})
}

func TestDecode_Integration_Hooks(t *testing.T) {
	t.Parallel()

	// Verify that internal default hooks (Time, Strings, etc.) are active by default
	type Config struct {
		Duration time.Duration `json:"duration"`
		Tags     []string      `json:"tags"`
	}
	input := map[string]any{
		"duration": "10s",
		"tags":     "a,b",
	}

	got, err := Decode[Config](input)
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, got.Duration)
	assert.Equal(t, []string{"a", "b"}, got.Tags)
}
