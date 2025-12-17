package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRobfigCronExpression(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		wantErr bool
	}{
		{
			name:    "Standard Cron (5 fields - invalid due to strict 6 fields setting)",
			spec:    "0 5 * * *", // 5 fields
			wantErr: true,
		},
		{
			name:    "Extended Cron (6 fields - with seconds)",
			spec:    "0 */5 * * * *", // 5분마다 (0초)
			wantErr: false,
		},
		{
			name:    "Daily at midnight",
			spec:    "@daily",
			wantErr: false,
		},
		{
			name:    "Invalid Cron (too few fields)",
			spec:    "* * *",
			wantErr: true,
		},
		{
			name:    "Invalid Cron (garbage)",
			spec:    "invalid-cron",
			wantErr: true,
		},
		{
			name:    "Empty string",
			spec:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRobfigCronExpression(tt.spec)
			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, apperrors.Is(err, apperrors.InvalidInput))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Valid seconds", "10s", false},
		{"Valid milliseconds", "500ms", false},
		{"Valid minutes", "5m", false},
		{"Valid combined", "1h30m", false},
		{"Invalid format", "10seconds", true},
		{"Invalid number", "invalid", true},
		{"Empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDuration(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, apperrors.Is(err, apperrors.InvalidInput))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		urlStr  string
		wantErr bool
	}{
		{"Valid HTTP", "http://example.com", false},
		{"Valid HTTPS", "https://example.com", false},
		{"Valid with port", "https://example.com:8080", false},
		{"Valid with path", "https://example.com/api/v1", false},
		{"Valid with query", "https://example.com/search?q=test", false},
		{"Valid Localhost", "http://localhost:3000", false},
		{"Valid IP", "http://192.168.0.1", false},
		{"Invalid Scheme (ftp)", "ftp://example.com", true},
		{"Invalid Scheme (missing)", "example.com", true},
		{"Invalid Format (spaces)", "http://exa mple.com", true},
		{"Missing Host", "http://", true},
		{"Empty String", "", false}, // Empty is allowed by design (optional)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.urlStr)
			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, apperrors.Is(err, apperrors.InvalidInput))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCORSOrigin(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		wantErr bool
	}{
		{"Valid Wildcard", "*", false},
		{"Valid HTTP", "http://example.com", false},
		{"Valid HTTPS", "https://example.com", false},
		{"Valid with port", "http://localhost:3000", false},
		{"Valid Subdomain", "https://api.example.com", false},
		{"Trailing Slash", "https://example.com/", true},
		{"With Path", "https://example.com/api", true},
		{"With Query", "https://example.com?q=1", true},
		{"Invalid Scheme", "ftp://example.com", true},
		{"No Scheme", "example.com", true},
		{"Empty String", "", true},
		{"Whitespace", "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCORSOrigin(tt.origin)
			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, apperrors.Is(err, apperrors.InvalidInput))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{"Valid Port", 8080, false},
		{"Valid Port (Min)", 1, false},
		{"Valid Port (Max)", 65535, false},
		{"System Port (Allowed but logs warning)", 80, false},
		{"Invalid Port (Zero)", 0, true},
		{"Invalid Port (Negative)", -1, true},
		{"Invalid Port (High)", 65536, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePort(tt.port)
			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, apperrors.Is(err, apperrors.InvalidInput))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFileExists(t *testing.T) {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", "testfile")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "testdir")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		path     string
		warnOnly bool
		wantErr  bool
		errType  apperrors.ErrorType
	}{
		{"Existing File", tmpFile.Name(), false, false, ""},
		{"Existing Directory", tmpDir, false, false, ""},
		{"Non-existing File", filepath.Join(tmpDir, "nonexistent"), false, true, apperrors.NotFound},
		{"Non-existing File (WarnOnly)", filepath.Join(tmpDir, "nonexistent"), true, false, ""}, // Error logged but nil returned
		{"Empty Path", "", false, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFileExists(tt.path, tt.warnOnly)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errType != "" {
					assert.True(t, apperrors.Is(err, tt.errType), "Expected error type %s, got %v", tt.errType, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFileExistsOrURL(t *testing.T) {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", "testfile")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tests := []struct {
		name     string
		path     string
		warnOnly bool
		wantErr  bool
	}{
		{"Valid URL", "https://example.com", false, false},
		{"Invalid URL", "http://", false, true},
		{"Existing File", tmpFile.Name(), false, false},
		{"Non-existing File", "nonexistent_file", false, true},
		{"Non-existing File (WarnOnly)", "nonexistent_file", true, false},
		{"Empty Path", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFileExistsOrURL(tt.path, tt.warnOnly)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateNoDuplicate(t *testing.T) {
	tests := []struct {
		name      string
		list      []string
		value     string
		valueType string
		wantErr   bool
	}{
		{"No Duplicate", []string{"a", "b"}, "c", "item", false},
		{"Duplicate", []string{"a", "b", "c"}, "b", "item", true},
		{"Empty List", []string{}, "a", "item", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNoDuplicate(tt.list, tt.value, tt.valueType)
			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, apperrors.Is(err, apperrors.InvalidInput))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// Examples
// ----------------------------------------------------------------------------

func ExampleValidateDuration() {
	if err := ValidateDuration("10m"); err == nil {
		fmt.Println("Valid duration")
	}
	// Output: Valid duration
}

func ExampleValidateURL() {
	if err := ValidateURL("https://example.com"); err == nil {
		fmt.Println("Valid URL")
	}
	// Output: Valid URL
}
