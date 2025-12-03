package config

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppConstants(t *testing.T) {
	t.Run("앱 상수 확인", func(t *testing.T) {
		assert.Equal(t, "notify-server", AppName, "AppName이 일치해야 합니다")

		assert.Equal(t, "notify-server.json", AppConfigFileName, "AppConfigFileName이 일치해야 합니다")
	})
}

func TestInitAppConfig_ValidConfig(t *testing.T) {
	t.Run("유효한 설정 파일 로드", func(t *testing.T) {
		// 테스트용 설정 파일 생성
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

		// JSON 파일로 저장
		data, err := json.MarshalIndent(testConfig, "", "  ")
		assert.NoError(t, err)

		// 원본 파일 백업
		originalExists := false
		var originalData []byte
		if _, err := os.Stat(AppConfigFileName); err == nil {
			originalExists = true
			originalData, _ = os.ReadFile(AppConfigFileName)
		}

		// 테스트 설정 파일 작성
		err = os.WriteFile(AppConfigFileName, data, 0644)
		assert.NoError(t, err)

		// 테스트 후 원본 복구를 위한 defer
		defer func() {
			if originalExists {
				os.WriteFile(AppConfigFileName, originalData, 0644)
			} else {
				os.Remove(AppConfigFileName)
			}
		}()

		// 설정 로드 테스트
		appConfig, err := InitAppConfig()
		assert.NoError(t, err)

		assert.NotNil(t, appConfig, "설정이 로드되어야 합니다")
		assert.True(t, appConfig.Debug, "Debug 모드가 활성화되어야 합니다")
		assert.Equal(t, "test-telegram", appConfig.Notifiers.DefaultNotifierID, "기본 Notifier ID가 일치해야 합니다")
		assert.Equal(t, 1, len(appConfig.Notifiers.Telegrams), "Telegram 설정이 1개여야 합니다")
		assert.Equal(t, 1, len(appConfig.NotifyAPI.Applications), "Application이 1개여야 합니다")
	})
}

func TestInitAppConfig_DuplicateNotifierID(t *testing.T) {
	t.Run("중복된 Notifier ID", func(t *testing.T) {
		testConfig := map[string]interface{}{
			"debug": false,
			"notifiers": map[string]interface{}{
				"default_notifier_id": "telegram1",
				"telegrams": []map[string]interface{}{
					{
						"id":        "telegram1",
						"bot_token": "token1",
						"chat_id":   float64(123),
					},
					{
						"id":        "telegram1", // Duplicate!
						"bot_token": "token2",
						"chat_id":   float64(456),
					},
				},
			},
			"tasks": []map[string]interface{}{},
			"notify_api": map[string]interface{}{
				"ws":           map[string]interface{}{"tls_server": false, "listen_port": float64(2443)},
				"applications": []map[string]interface{}{},
			},
		}

		data, _ := json.MarshalIndent(testConfig, "", "  ")

		originalExists, originalData := backupConfigFile()
		defer restoreConfigFile(originalExists, originalData)

		os.WriteFile(AppConfigFileName, data, 0644)

		// Should return error due to duplicate notifier ID
		appConfig, err := InitAppConfig()
		assert.Error(t, err, "중복된 Notifier ID로 인해 에러가 발생해야 합니다")
		assert.Nil(t, appConfig)
	})
}

func TestInitAppConfig_InvalidDefaultNotifierID(t *testing.T) {
	t.Run("존재하지 않는 기본 Notifier ID", func(t *testing.T) {
		testConfig := map[string]interface{}{
			"debug": false,
			"notifiers": map[string]interface{}{
				"default_notifier_id": "non-existent", // Invalid!
				"telegrams": []map[string]interface{}{
					{
						"id":        "telegram1",
						"bot_token": "token1",
						"chat_id":   float64(123),
					},
				},
			},
			"tasks": []map[string]interface{}{},
			"notify_api": map[string]interface{}{
				"ws":           map[string]interface{}{"tls_server": false, "listen_port": float64(2443)},
				"applications": []map[string]interface{}{},
			},
		}

		data, _ := json.MarshalIndent(testConfig, "", "  ")

		originalExists, originalData := backupConfigFile()
		defer restoreConfigFile(originalExists, originalData)

		os.WriteFile(AppConfigFileName, data, 0644)

		appConfig, err := InitAppConfig()
		assert.Error(t, err, "존재하지 않는 기본 Notifier ID로 인해 에러가 발생해야 합니다")
		assert.Nil(t, appConfig)
	})
}

func TestInitAppConfig_DuplicateTaskID(t *testing.T) {
	t.Run("중복된 Task ID", func(t *testing.T) {
		testConfig := map[string]interface{}{
			"debug": false,
			"notifiers": map[string]interface{}{
				"default_notifier_id": "telegram1",
				"telegrams": []map[string]interface{}{
					{"id": "telegram1", "bot_token": "token1", "chat_id": float64(123)},
				},
			},
			"tasks": []map[string]interface{}{
				{
					"id":       "TASK1",
					"title":    "Task 1",
					"commands": []map[string]interface{}{},
				},
				{
					"id":       "TASK1", // Duplicate!
					"title":    "Task 1 Duplicate",
					"commands": []map[string]interface{}{},
				},
			},
			"notify_api": map[string]interface{}{
				"ws":           map[string]interface{}{"tls_server": false, "listen_port": float64(2443)},
				"applications": []map[string]interface{}{},
			},
		}

		data, _ := json.MarshalIndent(testConfig, "", "  ")

		originalExists, originalData := backupConfigFile()
		defer restoreConfigFile(originalExists, originalData)

		os.WriteFile(AppConfigFileName, data, 0644)

		appConfig, err := InitAppConfig()
		assert.Error(t, err, "중복된 Task ID로 인해 에러가 발생해야 합니다")
		assert.Nil(t, appConfig)
	})
}

func TestInitAppConfig_DuplicateCommandID(t *testing.T) {
	t.Run("중복된 Command ID", func(t *testing.T) {
		testConfig := map[string]interface{}{
			"debug": false,
			"notifiers": map[string]interface{}{
				"default_notifier_id": "telegram1",
				"telegrams": []map[string]interface{}{
					{"id": "telegram1", "bot_token": "token1", "chat_id": float64(123)},
				},
			},
			"tasks": []map[string]interface{}{
				{
					"id":    "TASK1",
					"title": "Task 1",
					"commands": []map[string]interface{}{
						{
							"id":                  "CMD1",
							"title":               "Command 1",
							"default_notifier_id": "telegram1",
						},
						{
							"id":                  "CMD1", // Duplicate!
							"title":               "Command 1 Duplicate",
							"default_notifier_id": "telegram1",
						},
					},
				},
			},
			"notify_api": map[string]interface{}{
				"ws":           map[string]interface{}{"tls_server": false, "listen_port": float64(2443)},
				"applications": []map[string]interface{}{},
			},
		}

		data, _ := json.MarshalIndent(testConfig, "", "  ")

		originalExists, originalData := backupConfigFile()
		defer restoreConfigFile(originalExists, originalData)

		os.WriteFile(AppConfigFileName, data, 0644)

		appConfig, err := InitAppConfig()
		assert.Error(t, err, "중복된 Command ID로 인해 에러가 발생해야 합니다")
		assert.Nil(t, appConfig)
	})
}

func TestInitAppConfig_InvalidCommandNotifierID(t *testing.T) {
	t.Run("Command의 존재하지 않는 Notifier ID", func(t *testing.T) {
		testConfig := map[string]interface{}{
			"debug": false,
			"notifiers": map[string]interface{}{
				"default_notifier_id": "telegram1",
				"telegrams": []map[string]interface{}{
					{"id": "telegram1", "bot_token": "token1", "chat_id": float64(123)},
				},
			},
			"tasks": []map[string]interface{}{
				{
					"id":    "TASK1",
					"title": "Task 1",
					"commands": []map[string]interface{}{
						{
							"id":                  "CMD1",
							"title":               "Command 1",
							"default_notifier_id": "non-existent", // Invalid!
						},
					},
				},
			},
			"notify_api": map[string]interface{}{
				"ws":           map[string]interface{}{"tls_server": false, "listen_port": float64(2443)},
				"applications": []map[string]interface{}{},
			},
		}

		data, _ := json.MarshalIndent(testConfig, "", "  ")

		originalExists, originalData := backupConfigFile()
		defer restoreConfigFile(originalExists, originalData)

		os.WriteFile(AppConfigFileName, data, 0644)

		appConfig, err := InitAppConfig()
		assert.Error(t, err, "존재하지 않는 Command Notifier ID로 인해 에러가 발생해야 합니다")
		assert.Nil(t, appConfig)
	})
}

func TestInitAppConfig_TLSServerMissingCertFile(t *testing.T) {
	t.Run("TLS 서버 활성화 시 Cert 파일 누락", func(t *testing.T) {
		testConfig := map[string]interface{}{
			"debug": false,
			"notifiers": map[string]interface{}{
				"default_notifier_id": "telegram1",
				"telegrams": []map[string]interface{}{
					{"id": "telegram1", "bot_token": "token1", "chat_id": float64(123)},
				},
			},
			"tasks": []map[string]interface{}{},
			"notify_api": map[string]interface{}{
				"ws": map[string]interface{}{
					"tls_server":    true,
					"tls_cert_file": "", // Missing!
					"tls_key_file":  "/path/to/key.pem",
					"listen_port":   float64(2443),
				},
				"applications": []map[string]interface{}{},
			},
		}

		data, _ := json.MarshalIndent(testConfig, "", "  ")

		originalExists, originalData := backupConfigFile()
		defer restoreConfigFile(originalExists, originalData)

		os.WriteFile(AppConfigFileName, data, 0644)

		appConfig, err := InitAppConfig()
		assert.Error(t, err, "TLS Cert 파일 누락으로 인해 에러가 발생해야 합니다")
		assert.Nil(t, appConfig)
	})
}

func TestInitAppConfig_TLSServerMissingKeyFile(t *testing.T) {
	t.Run("TLS 서버 활성화 시 Key 파일 누락", func(t *testing.T) {
		testConfig := map[string]interface{}{
			"debug": false,
			"notifiers": map[string]interface{}{
				"default_notifier_id": "telegram1",
				"telegrams": []map[string]interface{}{
					{"id": "telegram1", "bot_token": "token1", "chat_id": float64(123)},
				},
			},
			"tasks": []map[string]interface{}{},
			"notify_api": map[string]interface{}{
				"ws": map[string]interface{}{
					"tls_server":    true,
					"tls_cert_file": "/path/to/cert.pem",
					"tls_key_file":  "", // Missing!
					"listen_port":   float64(2443),
				},
				"applications": []map[string]interface{}{},
			},
		}

		data, _ := json.MarshalIndent(testConfig, "", "  ")

		originalExists, originalData := backupConfigFile()
		defer restoreConfigFile(originalExists, originalData)

		os.WriteFile(AppConfigFileName, data, 0644)

		appConfig, err := InitAppConfig()
		assert.Error(t, err, "TLS Key 파일 누락으로 인해 에러가 발생해야 합니다")
		assert.Nil(t, appConfig)
	})
}

func TestInitAppConfig_DuplicateApplicationID(t *testing.T) {
	t.Run("중복된 Application ID", func(t *testing.T) {
		testConfig := map[string]interface{}{
			"debug": false,
			"notifiers": map[string]interface{}{
				"default_notifier_id": "telegram1",
				"telegrams": []map[string]interface{}{
					{"id": "telegram1", "bot_token": "token1", "chat_id": float64(123)},
				},
			},
			"tasks": []map[string]interface{}{},
			"notify_api": map[string]interface{}{
				"ws": map[string]interface{}{"tls_server": false, "listen_port": float64(2443)},
				"applications": []map[string]interface{}{
					{
						"id":                  "app1",
						"title":               "App 1",
						"default_notifier_id": "telegram1",
						"app_key":             "key1",
					},
					{
						"id":                  "app1", // Duplicate!
						"title":               "App 1 Duplicate",
						"default_notifier_id": "telegram1",
						"app_key":             "key2",
					},
				},
			},
		}

		data, _ := json.MarshalIndent(testConfig, "", "  ")

		originalExists, originalData := backupConfigFile()
		defer restoreConfigFile(originalExists, originalData)

		os.WriteFile(AppConfigFileName, data, 0644)

		appConfig, err := InitAppConfig()
		assert.Error(t, err, "중복된 Application ID로 인해 에러가 발생해야 합니다")
		assert.Nil(t, appConfig)
	})
}

func TestInitAppConfig_InvalidApplicationNotifierID(t *testing.T) {
	t.Run("Application의 존재하지 않는 Notifier ID", func(t *testing.T) {
		testConfig := map[string]interface{}{
			"debug": false,
			"notifiers": map[string]interface{}{
				"default_notifier_id": "telegram1",
				"telegrams": []map[string]interface{}{
					{"id": "telegram1", "bot_token": "token1", "chat_id": float64(123)},
				},
			},
			"tasks": []map[string]interface{}{},
			"notify_api": map[string]interface{}{
				"ws": map[string]interface{}{"tls_server": false, "listen_port": float64(2443)},
				"applications": []map[string]interface{}{
					{
						"id":                  "app1",
						"title":               "App 1",
						"default_notifier_id": "non-existent", // Invalid!
						"app_key":             "key1",
					},
				},
			},
		}

		data, _ := json.MarshalIndent(testConfig, "", "  ")

		originalExists, originalData := backupConfigFile()
		defer restoreConfigFile(originalExists, originalData)

		os.WriteFile(AppConfigFileName, data, 0644)

		appConfig, err := InitAppConfig()
		assert.Error(t, err, "존재하지 않는 Application Notifier ID로 인해 에러가 발생해야 합니다")
		assert.Nil(t, appConfig)
	})
}

func TestInitAppConfig_MissingAppKey(t *testing.T) {
	t.Run("Application의 AppKey 누락", func(t *testing.T) {
		testConfig := map[string]interface{}{
			"debug": false,
			"notifiers": map[string]interface{}{
				"default_notifier_id": "telegram1",
				"telegrams": []map[string]interface{}{
					{"id": "telegram1", "bot_token": "token1", "chat_id": float64(123)},
				},
			},
			"tasks": []map[string]interface{}{},
			"notify_api": map[string]interface{}{
				"ws": map[string]interface{}{"tls_server": false, "listen_port": float64(2443)},
				"applications": []map[string]interface{}{
					{
						"id":                  "app1",
						"title":               "App 1",
						"default_notifier_id": "telegram1",
						"app_key":             "", // Missing!
					},
				},
			},
		}

		data, _ := json.MarshalIndent(testConfig, "", "  ")

		originalExists, originalData := backupConfigFile()
		defer restoreConfigFile(originalExists, originalData)

		os.WriteFile(AppConfigFileName, data, 0644)

		appConfig, err := InitAppConfig()
		assert.Error(t, err, "AppKey 누락으로 인해 에러가 발생해야 합니다")
		assert.Nil(t, appConfig)
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

// Helper functions
func backupConfigFile() (bool, []byte) {
	originalExists := false
	var originalData []byte
	if _, err := os.Stat(AppConfigFileName); err == nil {
		originalExists = true
		originalData, _ = os.ReadFile(AppConfigFileName)
	}
	return originalExists, originalData
}

func restoreConfigFile(originalExists bool, originalData []byte) {
	if originalExists {
		os.WriteFile(AppConfigFileName, originalData, 0644)
	} else {
		os.Remove(AppConfigFileName)
	}
}
