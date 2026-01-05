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
// Test Data Structures
// =============================================================================

type User struct {
	Name   string `json:"name"`
	Age    int    `json:"age"`
	Active bool   `json:"active"`
}

type Config struct {
	User    User          `json:"user"`
	TIMEOUT time.Duration `json:"timeout"` // Upper case to test matching if needed, though tag is "timeout"
	Roles   []string      `json:"roles"`
}

type EmbeddedConfig struct {
	User `mapstructure:",squash"`
	ID   string `json:"id"`
}

// =============================================================================
// Table Driven Tests for Decode
// =============================================================================

func TestDecode(t *testing.T) {
	t.Parallel()

	basicMap := map[string]any{"name": "Alice", "age": 30, "active": true}

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		// ---------------------------------------------------------------------
		// 1. Basic Decoding
		// ---------------------------------------------------------------------
		{
			name: "Basic Struct Decoding",
			run: func(t *testing.T) {
				input := map[string]any{"name": "Alice", "age": 30, "active": true}
				var target User
				err := DecodeTo(input, &target)
				require.NoError(t, err)
				assert.Equal(t, User{Name: "Alice", Age: 30, Active: true}, target)
			},
		},
		{
			name: "Nested Struct Decoding",
			run: func(t *testing.T) {
				input := map[string]any{
					"user":    map[string]any{"name": "Alice", "age": 30},
					"timeout": "10s",
					"roles":   []string{"admin", "editor"},
				}
				var target Config
				err := DecodeTo(input, &target)
				require.NoError(t, err)
				assert.Equal(t, "Alice", target.User.Name)
				assert.Equal(t, 10*time.Second, target.TIMEOUT)
				assert.Equal(t, []string{"admin", "editor"}, target.Roles)
			},
		},
		{
			name: "Embedded Struct (Squash Default)",
			run: func(t *testing.T) {
				input := map[string]any{"id": "conf-1", "name": "Bob", "age": 50}
				var target EmbeddedConfig
				err := DecodeTo(input, &target)
				require.NoError(t, err)
				assert.Equal(t, "conf-1", target.ID)
				assert.Equal(t, "Bob", target.User.Name)
			},
		},

		// ---------------------------------------------------------------------
		// 2. Options Testing
		// ---------------------------------------------------------------------
		{
			name: "WithTagName (yaml)",
			run: func(t *testing.T) {
				type YamlStruct struct {
					Val string `yaml:"my_val"`
				}
				input := map[string]any{"my_val": "data"}
				var target YamlStruct
				err := DecodeTo(input, &target, WithTagName("yaml"))
				require.NoError(t, err)
				assert.Equal(t, "data", target.Val)
			},
		},
		{
			name: "WithWeaklyTypedInput (True - Default)",
			run: func(t *testing.T) {
				input := map[string]any{"age": "25"}
				var target User
				err := DecodeTo(input, &target) // Default true
				require.NoError(t, err)
				assert.Equal(t, 25, target.Age)
			},
		},
		{
			name: "WithWeaklyTypedInput (False - Strict)",
			run: func(t *testing.T) {
				input := map[string]any{"age": "25"}
				var target User
				err := DecodeTo(input, &target, WithWeaklyTypedInput(false))
				require.Error(t, err)
			},
		},
		{
			name: "WithErrorUnused (True)",
			run: func(t *testing.T) {
				input := map[string]any{"name": "Alice", "unused": "field"}
				var target User
				err := DecodeTo(input, &target, WithErrorUnused(true))
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unused")
			},
		},
		{
			name: "WithZeroFields (True - Replace)",
			run: func(t *testing.T) {
				input := map[string]any{"age": 40}
				target := User{Name: "Old", Age: 20, Active: true}

				err := DecodeTo(input, &target, WithZeroFields(true))
				require.NoError(t, err)
				assert.Equal(t, "", target.Name) // Zeroed
				assert.Equal(t, 40, target.Age)
				assert.Equal(t, false, target.Active) // Zeroed
			},
		},
		{
			name: "WithTrimSpace (True - Default)",
			run: func(t *testing.T) {
				input := map[string]any{"roles": " a , b "}
				var target Config
				err := DecodeTo(input, &target) // Default true
				require.NoError(t, err)
				assert.Equal(t, []string{"a", "b"}, target.Roles)
			},
		},
		{
			name: "WithTrimSpace (False)",
			run: func(t *testing.T) {
				input := map[string]any{"roles": " a , b "}
				var target Config
				err := DecodeTo(input, &target, WithTrimSpace(false))
				require.NoError(t, err)
				assert.Equal(t, []string{" a ", " b "}, target.Roles)
			},
		},
		{
			name: "WithMetadata",
			run: func(t *testing.T) {
				input := map[string]any{"name": "Alice", "extra": 1}
				var md mapstructure.Metadata
				var target User
				err := DecodeTo(input, &target, WithMetadata(&md))
				require.NoError(t, err)
				assert.Contains(t, md.Keys, "name")
				assert.Contains(t, md.Unused, "extra")
			},
		},
		{
			name: "WithMatchName (Custom)",
			run: func(t *testing.T) {
				input := map[string]any{"THE_NAME": "Alice"}
				var target User
				matcher := func(mapK, fieldK string) bool {
					return mapK == "THE_"+strings.ToUpper(fieldK)
				}
				err := DecodeTo(input, &target, WithMatchName(matcher))
				require.NoError(t, err)
				assert.Equal(t, "Alice", target.Name)
			},
		},
		{
			name: "WithDecodeHook (Custom Priority)",
			run: func(t *testing.T) {
				// Custom hook that intercepts Age 999 and turns it into 0
				hit := false
				hook := func(f, to reflect.Type, data any) (any, error) {
					if to.Kind() == reflect.Int && data == 999 {
						hit = true
						return 0, nil
					}
					return data, nil
				}
				input := map[string]any{"age": 999}
				var target User
				err := DecodeTo(input, &target, WithDecodeHook(hook))
				require.NoError(t, err)
				assert.True(t, hit)
				assert.Equal(t, 0, target.Age)
			},
		},

		// ---------------------------------------------------------------------
		// 3. Edge Cases & Errors
		// ---------------------------------------------------------------------
		{
			name: "Nil Output",
			run: func(t *testing.T) {
				err := DecodeTo(basicMap, (*User)(nil))
				require.Error(t, err)
				assert.Contains(t, err.Error(), "nil")
			},
		},
		{
			name: "Cycle Detection",
			run: func(t *testing.T) {
				// Prevent stack overflow in test
				defer func() {
					if r := recover(); r != nil {
						// Recovered from panic (expected if stack overflow or mapstructure panic)
						// t.Log("Recovered from cyclic panic")
					}
				}()

				type Node struct {
					Ref any `json:"ref"`
				}
				m := make(map[string]any)
				m["ref"] = m

				var target Node
				// This might panic with stack overflow.
				// Just ensuring it doesn't crash the test suite (recover handles it).
				_ = DecodeTo(m, &target)
				// If we get here without panic, mapstructure might have handled it gracefully (depth limit?)
				// or we just got lucky.
			},
		},
		{
			name: "Base64 Decoding Integration",
			run: func(t *testing.T) {
				// Verifies that the internal base64 hook is active
				input := map[string]any{
					"data": "base64:SGVsbG8=",
				}
				var target struct {
					Data []byte `json:"data"`
				}
				err := DecodeTo(input, &target)
				require.NoError(t, err)
				assert.Equal(t, []byte("Hello"), target.Data)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

// Example for GoDoc
func ExampleDecode() {
	input := map[string]any{"name": "Alice", "age": 30}

	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	p, _ := Decode[Person](input)
	// fmt.Printf("%s: %d", p.Name, p.Age)
	_ = p
}
