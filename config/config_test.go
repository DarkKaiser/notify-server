package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: 임시 설정 파일 생성
func createTempConfigFile(t *testing.T, content interface{}) string {
	t.Helper()
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test_config.json")

	var data []byte
	var err error

	switch v := content.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		data, err = json.Marshal(v)
		require.NoError(t, err)
	}

	err = os.WriteFile(configFile, data, 0644)
	require.NoError(t, err)

	return configFile
}

func TestAppConstants(t *testing.T) {
	assert.Equal(t, "notify-server", AppName)
	assert.Equal(t, "notify-server.json", AppConfigFileName)
}

func TestSetDefaults(t *testing.T) {
	t.Run("Default values", func(t *testing.T) {
		config := &AppConfig{}
		config.SetDefaults()

		assert.Equal(t, DefaultMaxRetries, config.HTTPRetry.MaxRetries)
		assert.Equal(t, DefaultRetryDelay, config.HTTPRetry.RetryDelay)
	})

	t.Run("Keep existing values", func(t *testing.T) {
		config := &AppConfig{
			HTTPRetry: HTTPRetryConfig{
				MaxRetries: 5,
				RetryDelay: "10s",
			},
		}
		config.SetDefaults()

		assert.Equal(t, 5, config.HTTPRetry.MaxRetries)
		assert.Equal(t, "10s", config.HTTPRetry.RetryDelay)
	})
}

func TestInitAppConfig_Success(t *testing.T) {
	validConfig := map[string]interface{}{
		"debug": true,
		"notifiers": map[string]interface{}{
			"default_notifier_id": "test-telegram",
			"telegrams": []map[string]interface{}{
				{"id": "test-telegram", "bot_token": "token", "chat_id": 123},
			},
		},
		"tasks": []interface{}{},
		"notify_api": map[string]interface{}{
			"ws":   map[string]interface{}{"listen_port": 8080},
			"cors": map[string]interface{}{"allow_origins": []string{"*"}},
			"applications": []map[string]interface{}{
				{"id": "app1", "app_key": "key1", "default_notifier_id": "test-telegram"},
			},
		},
	}

	configFile := createTempConfigFile(t, validConfig)

	cfg, err := InitAppConfigWithFile(configFile)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.True(t, cfg.Debug)
	assert.Equal(t, "test-telegram", cfg.Notifiers.DefaultNotifierID)
}

func TestInitAppConfig_Failure(t *testing.T) {
	tests := []struct {
		name          string
		setupFile     func(t *testing.T) string // Returns path to config file
		errorContains string
	}{
		{
			name: "File not found",
			setupFile: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "non-existent.json")
			},
			errorContains: "파일을 열 수 없습니다",
		},
		{
			name: "Invalid JSON",
			setupFile: func(t *testing.T) string {
				return createTempConfigFile(t, "{ invalid json")
			},
			errorContains: "JSON 파싱이 실패하였습니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setupFile(t)
			cfg, err := InitAppConfigWithFile(path)
			assert.Error(t, err)
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tt.errorContains)
		})
	}
}

func TestAppConfig_JSONMarshaling(t *testing.T) {
	jsonData := `{
		"debug": true,
		"notifiers": {
			"default_notifier_id": "telegram1",
			"telegrams": [
				{
					"id": "telegram1",
					"bot_token": "token123",
					"chat_id": 123456
				}
			]
		},
		"tasks": [],
		"notify_api": {
			"ws": { "listen_port": 2443 },
			"cors": { "allow_origins": ["*"] },
			"applications": []
		}
	}`

	var appConfig AppConfig
	err := json.Unmarshal([]byte(jsonData), &appConfig)

	assert.NoError(t, err)
	assert.True(t, appConfig.Debug)
	assert.Equal(t, "telegram1", appConfig.Notifiers.DefaultNotifierID)
	assert.Len(t, appConfig.Notifiers.Telegrams, 1)
	assert.Equal(t, int64(123456), appConfig.Notifiers.Telegrams[0].ChatID)
}
