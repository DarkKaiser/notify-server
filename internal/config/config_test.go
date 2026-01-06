package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Unit Tests: Configuration Logic & Helpers
// =============================================================================

func TestNormalizeEnvKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"NOTIFY_DEBUG", "debug"},
		{"NOTIFY_HTTP_RETRY__MAX_RETRIES", "http_retry.max_retries"},
		{"NOTIFY_NOTIFY_API__CORS__ALLOW_ORIGINS", "notify_api.cors.allow_origins"},
		{"NOTIFY_DEBUG", "debug"}, // This line was added based on the instruction's content
		{"DEBUG", "debug"},        // Prefix가 없어도 동작은 하지만, 실제 호출부는 prefix를 제거하고 넘길 수도 있음 (현재 구현상 TrimPrefix는 한번만 동작)
		{"NOTIFY_DEBUG", "debug"},
		{"NOTIFY_Mixed_Case__Key", "mixed_case.key"},
	}

	for _, tt := range tests {
		got := normalizeEnvKey(tt.input)
		assert.Equal(t, tt.expected, got, "Input: %s", tt.input)
	}
}

func TestNewDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := newDefaultConfig()

	// Assert Default Values
	assert.True(t, cfg.Debug, "Default debug should be true")
	assert.Equal(t, DefaultMaxRetries, cfg.HTTPRetry.MaxRetries)
	assert.Equal(t, DefaultRetryDelay, cfg.HTTPRetry.RetryDelay)
	assert.Empty(t, cfg.Notifiers.Telegrams)
	assert.Empty(t, cfg.Tasks)
}

// =============================================================================
// Integration Tests: Load Logic
// =============================================================================

func TestLoad_Integration(t *testing.T) {
	// 통합 테스트는 Env 설정 등으로 인해 병렬 실행 시 간섭이 발생할 수 있으므로 t.Parallel() 사용에 주의해야 합니다.
	// t.Setenv는 해당 테스트 범위 내에서만 Env를 변경하므로 안전하지만,
	// 다른 병렬 테스트가 Env를 읽는다면 문제가 될 수 있습니다.
	// 여기서는 순차 실행을 보장하기 위해 t.Parallel()을 생략하거나 주의해서 사용합니다.

	createTempConfig := func(t *testing.T, content string) string {
		t.Helper()
		dir := t.TempDir()
		path := filepath.Join(dir, "config.json")
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
		return path
	}

	t.Run("Priority: Env > File > Defaults", func(t *testing.T) {
		// 1. File Config (Overrides Defaults)
		jsonContent := `{
			"http_retry": {"max_retries": 10},
			"notifiers": {
				"default_notifier_id": "file-tele",
				"telegrams": [{"id": "file-tele", "bot_token": "123456789:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", "chat_id": 1}]
			},
			"notify_api": { "ws": {"listen_port": 9000}, "cors": {"allow_origins": ["*"]}, "applications": [] }
		}`
		path := createTempConfig(t, jsonContent)

		// 2. Env Config (Overrides File)
		t.Setenv("NOTIFY_HTTP_RETRY__MAX_RETRIES", "50")

		// 3. Load
		cfg, err := LoadWithFile(path)
		require.NoError(t, err)

		// 4. Verification
		assert.Equal(t, 50, cfg.HTTPRetry.MaxRetries, "Environment variable should take precedence over file")
		assert.Equal(t, 9000, cfg.NotifyAPI.WS.ListenPort, "File config should take precedence over defaults")
		assert.True(t, cfg.Debug, "Default value should persist if not overridden")
	})

	t.Run("Error: File Not Found", func(t *testing.T) {
		cfg, err := LoadWithFile("non-existent-config.json")
		require.Error(t, err)
		assert.Nil(t, cfg)

		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			assert.Equal(t, apperrors.System, appErr.Type)
			assert.Contains(t, err.Error(), "설정 파일을 찾을 수 없습니다")
		}
	})

	t.Run("Error: Malformed JSON", func(t *testing.T) {
		path := createTempConfig(t, "{ invalid_json: ... }")
		cfg, err := LoadWithFile(path)
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "설정 파일 로드 중 오류")
	})

	t.Run("Error: Unknown Fields (Strict Unmarshal)", func(t *testing.T) {
		jsonContent := `{
			"unknown_field": "hacking",
			"debug": true
		}`
		path := createTempConfig(t, jsonContent)
		cfg, err := LoadWithFile(path)
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "구조체로 변환하는데 실패했습니다")
	})

	t.Run("Error: Validation Failure After Load", func(t *testing.T) {
		// Valid JSON structure but invalid logic (e.g., negative port)
		// Validation fails fast, so we need minimally valid config for previous steps (Notifiers)
		jsonContent := `{
			"notifiers": {
				"default_notifier_id": "test-notifier",
				"telegrams": [{"id": "test-notifier", "bot_token": "123456789:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", "chat_id": 12345}]
			},
			"notify_api": { "ws": {"listen_port": -1}, "cors": {"allow_origins": ["*"]}, "applications": [] }
		}`
		path := createTempConfig(t, jsonContent)
		cfg, err := LoadWithFile(path)
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "웹 서버 포트(listen_port)는 1에서 65535 사이의 값이어야 합니다")
	})
}
