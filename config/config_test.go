package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppConstants(t *testing.T) {
	t.Run("앱 상수 확인", func(t *testing.T) {
		assert.Equal(t, "notify-server", AppName, "AppName이 일치해야 합니다")
		assert.Equal(t, "notify-server.json", AppConfigFileName, "AppConfigFileName이 일치해야 합니다")
	})
}

func TestSetDefaults(t *testing.T) {
	t.Run("기본값 설정 확인", func(t *testing.T) {
		config := &AppConfig{}
		config.SetDefaults()

		assert.Equal(t, DefaultMaxRetries, config.HTTPRetry.MaxRetries)
		assert.Equal(t, DefaultRetryDelay, config.HTTPRetry.RetryDelay)
	})

	t.Run("기이 설정된 값 유지 확인", func(t *testing.T) {
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

func TestInitAppConfig_ValidConfig(t *testing.T) {
	t.Run("유효한 설정 파일 로드", func(t *testing.T) {
		// 임시 디렉토리 생성
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "test_config.json")

		testConfig := map[string]interface{}{
			"debug": true,
			"notifiers": map[string]interface{}{
				"default_notifier_id": "test-telegram",
				"telegrams": []map[string]interface{}{
					{
						"id":        "test-telegram",
						"bot_token": "test-token",
						"chat_id":   float64(123456),
					},
				},
			},
			"tasks": []map[string]interface{}{},
			"notify_api": map[string]interface{}{
				"ws": map[string]interface{}{
					"tls_server":    false,
					"tls_cert_file": "",
					"tls_key_file":  "",
					"listen_port":   float64(2443),
				},
				"cors": map[string]interface{}{
					"allow_origins": []string{"*"},
				},
				"applications": []map[string]interface{}{
					{
						"id":                  "test-app",
						"title":               "Test App",
						"description":         "Test Description",
						"default_notifier_id": "test-telegram",
						"app_key":             "test-key",
					},
				},
			},
		}

		// JSON 파일 생성
		file, err := os.Create(configFile)
		assert.NoError(t, err)

		encoder := json.NewEncoder(file)
		err = encoder.Encode(testConfig)
		assert.NoError(t, err)
		file.Close()

		// 설정 로드 테스트
		appConfig, err := InitAppConfigWithFile(configFile)
		assert.NoError(t, err)

		assert.NotNil(t, appConfig, "설정이 로드되어야 합니다")
		assert.True(t, appConfig.Debug, "Debug 모드가 활성화되어야 합니다")
		assert.Equal(t, "test-telegram", appConfig.Notifiers.DefaultNotifierID, "기본 Notifier ID가 일치해야 합니다")
		assert.Equal(t, 1, len(appConfig.Notifiers.Telegrams), "Telegram 설정이 1개여야 합니다")
		assert.Equal(t, 1, len(appConfig.NotifyAPI.Applications), "Application이 1개여야 합니다")

		// 기본값 적용 확인
		assert.Equal(t, DefaultMaxRetries, appConfig.HTTPRetry.MaxRetries)
		assert.Equal(t, DefaultRetryDelay, appConfig.HTTPRetry.RetryDelay)
	})
}

func TestInitAppConfig_FileNotFound(t *testing.T) {
	t.Run("파일이 존재하지 않을 때", func(t *testing.T) {
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "non_existent.json")

		appConfig, err := InitAppConfigWithFile(configFile)
		assert.Error(t, err)
		assert.Nil(t, appConfig)
		assert.Contains(t, err.Error(), "파일을 열 수 없습니다")
	})
}

func TestInitAppConfig_InvalidJSON(t *testing.T) {
	t.Run("잘못된 JSON 형식", func(t *testing.T) {
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "invalid.json")

		err := os.WriteFile(configFile, []byte("{ invalid json"), 0644)
		assert.NoError(t, err)

		appConfig, err := InitAppConfigWithFile(configFile)
		assert.Error(t, err)
		assert.Nil(t, appConfig)
		assert.Contains(t, err.Error(), "JSON 파싱이 실패하였습니다")
	})
}

func TestAppConfig_Structure(t *testing.T) {
	t.Run("AppConfig 구조체 필드 확인", func(t *testing.T) {
		appConfig := &AppConfig{}

		// 구조체가 정상적으로 생성되는지 확인
		assert.NotNil(t, appConfig, "AppConfig 구조체가 생성되어야 합니다")

		// 기본값 확인
		assert.False(t, appConfig.Debug, "Debug 기본값은 false여야 합니다")
		assert.Empty(t, appConfig.Notifiers.DefaultNotifierID, "기본 Notifier ID는 비어있어야 합니다")
		assert.Empty(t, appConfig.Tasks, "Tasks는 비어있어야 합니다")
	})
}

func TestAppConfig_JSONMarshaling(t *testing.T) {
	t.Run("JSON 마샬링/언마샬링", func(t *testing.T) {
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
				"ws": {
					"tls_server": false,
					"tls_cert_file": "",
					"tls_key_file": "",
					"listen_port": 2443
				},
				"cors": {
					"allow_origins": ["*"]
				},
				"applications": []
			}
		}`

		var appConfig AppConfig
		err := json.Unmarshal([]byte(jsonData), &appConfig)

		assert.NoError(t, err, "JSON 언마샬링이 성공해야 합니다")
		assert.True(t, appConfig.Debug, "Debug가 true여야 합니다")
		assert.Equal(t, "telegram1", appConfig.Notifiers.DefaultNotifierID, "Notifier ID가 일치해야 합니다")
		assert.Equal(t, "telegram1", appConfig.Notifiers.Telegrams[0].ID, "Telegram ID가 일치해야 합니다")
		assert.Equal(t, "token123", appConfig.Notifiers.Telegrams[0].BotToken, "Bot Token이 일치해야 합니다")
		assert.Equal(t, int64(123456), appConfig.Notifiers.Telegrams[0].ChatID, "Chat ID가 일치해야 합니다")
	})
}
