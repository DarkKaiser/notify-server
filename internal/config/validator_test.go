package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Unit Tests: Helper Functions (checkUniqueField, checkStruct)
// =============================================================================

// TestCheckUniqueField verifies duplicate detection logic within slices.
func TestCheckUniqueField(t *testing.T) {
	t.Parallel()

	type Item struct {
		ID   string `validate:"required"`
		Name string
	}

	tests := []struct {
		name          string
		data          interface{}
		fieldName     string
		contextName   string
		shouldError   bool
		errorContains string
	}{
		{
			name:        "Empty Slice",
			data:        []Item{},
			fieldName:   "ID",
			contextName: "Items",
			shouldError: false,
		},
		{
			name:        "Nil Slice",
			data:        []Item(nil),
			fieldName:   "ID",
			contextName: "Items",
			shouldError: false,
		},
		{
			name: "Single Item (Unique)",
			data: []Item{
				{ID: "1", Name: "A"},
			},
			fieldName:   "ID",
			contextName: "Items",
			shouldError: false,
		},
		{
			name: "Multiple Items (Unique)",
			data: []Item{
				{ID: "1", Name: "A"},
				{ID: "2", Name: "B"},
				{ID: "3", Name: "C"},
			},
			fieldName:   "ID",
			contextName: "Items",
			shouldError: false,
		},
		{
			name: "Duplicate Items",
			data: []Item{
				{ID: "1", Name: "A"},
				{ID: "1", Name: "B"}, // Duplicate ID
			},
			fieldName:     "ID",
			contextName:   "Items",
			shouldError:   true,
			errorContains: "중복된 Items ID가 존재합니다",
		},
		{
			name: "Triplicate Items",
			data: []Item{
				{ID: "1", Name: "A"},
				{ID: "1", Name: "B"},
				{ID: "1", Name: "C"},
			},
			fieldName:     "ID",
			contextName:   "Items",
			shouldError:   true,
			errorContains: "중복된 Items ID가 존재합니다",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := newValidator()
			err := checkUniqueField(v, tt.data, tt.fieldName, tt.contextName)
			if tt.shouldError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCheckStruct verifies structural validation and error formatting.
func TestCheckStruct(t *testing.T) {
	t.Parallel()

	type SubConfig struct {
		ID string `json:"id" validate:"required"`
	}

	type TestConfig struct {
		Name     string      `json:"name" validate:"required"`
		Age      int         `json:"age" validate:"min=18"`
		Optional string      `json:"optional"`
		Sub      []SubConfig `json:"sub" validate:"unique=ID"`
	}

	tests := []struct {
		name          string
		input         TestConfig
		contextName   string
		shouldError   bool
		errorContains string
	}{
		{
			name:        "Valid Struct",
			input:       TestConfig{Name: "John", Age: 20},
			contextName: "User",
			shouldError: false,
		},
		{
			name:          "Missing Required Field",
			input:         TestConfig{Age: 20}, // Name missing
			contextName:   "User",
			shouldError:   true,
			errorContains: "User의 설정이 올바르지 않습니다: name (조건: required)",
		},
		{
			name:          "Validation Failed (Min)",
			input:         TestConfig{Name: "John", Age: 10}, // Age < 18
			contextName:   "User",
			shouldError:   true,
			errorContains: "User의 설정이 올바르지 않습니다: age (조건: min)",
		},
		{
			name: "Duplicate ID in SubSlice (unique tag)",
			input: TestConfig{
				Name: "John", Age: 20,
				Sub: []SubConfig{{ID: "a"}, {ID: "a"}},
			},
			contextName:   "User",
			shouldError:   true,
			errorContains: "User 내에 중복된 ID가 존재합니다",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := newValidator()
			err := checkStruct(v, tt.input, tt.contextName)
			if tt.shouldError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// Unit Tests: Custom Validators & Infrastructure
// =============================================================================

// TestValidate_Infrastructure_JSONTagName checks if error messages use JSON tags.
func TestValidate_Infrastructure_JSONTagName(t *testing.T) {
	t.Parallel()

	type TestStruct struct {
		RequiredField string `json:"required_field" validate:"required"`
		OmitField     string `json:"omit_field,omitempty" validate:"required"`
		NoTagField    string `validate:"required"`
		DashTagField  string `json:"-" validate:"required"`
	}

	tests := []struct {
		name          string
		input         TestStruct
		expectedValid bool
		errorContains string
	}{
		{
			name:          "Required Field Missing",
			input:         TestStruct{},
			expectedValid: false,
			errorContains: "required_field", // json tag name
		},
		{
			name:          "No JSON Tag",
			input:         TestStruct{RequiredField: "valid", OmitField: "valid"}, // Fill prior required fields
			expectedValid: false,
			errorContains: "NoTagField", // fallback to field name
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := newValidator()
			err := checkStruct(v, tt.input, "TestStruct")
			if !tt.expectedValid {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidate_Unit_TelegramBotToken verifies the custom Telegram Bot Token validator.
func TestValidate_Unit_TelegramBotToken(t *testing.T) {
	t.Parallel()

	type BotTokenStruct struct {
		Token string `validate:"telegram_bot_token"`
	}

	tests := []struct {
		name  string
		token string
		valid bool
	}{
		// Valid cases
		{"Valid Token", "123456789:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", true},
		{"Valid Token (Minimum Length)", "123:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", true},
		{"Valid Token (Long ID)", "12345678901234567890:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", true},

		// Invalid cases
		{"Empty Token", "", false},
		{"No Separator", "123456789ABC-DEF1234ghIkl-zyx57W2v1u123ew11", false},
		{"ID Too Short", "12:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", false},
		{"ID Not Numeric", "ABC:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", false},
		{"Secret Too Short", "123456789:ShortSecret", false},
		{"Secret Contains Special Char", "123456789:ABC-DEF1234ghIkl-zyx57W2v1u123@#$", false}, // Only - and _ allowed
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := newValidator()
			err := v.Struct(BotTokenStruct{Token: tt.token})
			if tt.valid {
				assert.NoError(t, err, "Token '%s' should be valid", tt.token)
			} else {
				assert.Error(t, err, "Token '%s' should be invalid", tt.token)
			}
		})
	}
}

// TestValidate_Unit_CORSOrigin verifies the custom CORS origin validator.
func TestValidate_Unit_CORSOrigin(t *testing.T) {
	t.Parallel()

	type CORSStruct struct {
		Origin string `validate:"cors_origin"`
	}

	tests := []struct {
		name   string
		origin string
		valid  bool
	}{
		// Valid cases
		{"Wildcard", "*", true},
		{"HTTP Localhost", "http://localhost", true},
		{"HTTPS Example", "https://example.com", true},
		{"HTTP with Port", "http://localhost:8080", true},
		{"Subdomain", "https://api.example.com", true},
		{"IP Address", "http://127.0.0.1", true},
		{"IP with Port", "http://192.168.0.1:3000", true},

		// Invalid cases
		{"Missing Scheme", "example.com", false},
		{"Unsupported Scheme (FTP)", "ftp://example.com", false},
		{"Empty String", "", false},
		{"Just Scheme", "http://", false},
		{"Leading Whitespace", " https://example.com", false},
		{"Trailing Slash", "https://example.com/", false},
		{"Path Included", "https://example.com/api", false},
		{"Query String Included", "https://example.com?q=1", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Direct usage of validator to test custom tag registration
			v := newValidator()
			err := v.Struct(CORSStruct{Origin: tt.origin})
			if tt.valid {
				assert.NoError(t, err, "Origin '%s' should be valid", tt.origin)
			} else {
				assert.Error(t, err, "Origin '%s' should be invalid", tt.origin)
			}
		})
	}
}
