package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"strings"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Helpers and Utilities
// =============================================================================

// ConfigBuilder는 테스트용 AppConfig 객체를 유창한 API로 생성하는 빌더 패턴입니다.
// 테스트 케이스별로 필요한 설정만 간결하게 구성할 수 있습니다.
type ConfigBuilder struct {
	config *AppConfig
}

// NewConfigBuilder는 기본 유효한 설정으로 초기화된 ConfigBuilder를 생성합니다.
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		config: &AppConfig{
			HTTPRetry: HTTPRetryConfig{
				MaxRetries: DefaultMaxRetries,
				RetryDelay: DefaultRetryDelay,
			},
			Notifiers: NotifierConfig{
				DefaultNotifierID: "telegram1",
				Telegrams: []TelegramConfig{
					{ID: "telegram1", BotToken: "token", ChatID: 123},
				},
			},
			Tasks: []TaskConfig{},
			NotifyAPI: NotifyAPIConfig{
				WS:           WSConfig{ListenPort: 8080},
				CORS:         CORSConfig{AllowOrigins: []string{"*"}},
				Applications: []ApplicationConfig{},
			},
		},
	}
}

// WithDebug는 디버그 모드를 설정합니다.
func (b *ConfigBuilder) WithDebug(debug bool) *ConfigBuilder {
	b.config.Debug = debug
	return b
}

// WithHTTPRetry는 HTTP 재시도 설정을 지정합니다.
func (b *ConfigBuilder) WithHTTPRetry(maxRetries int, retryDelay string) *ConfigBuilder {
	b.config.HTTPRetry = HTTPRetryConfig{
		MaxRetries: maxRetries,
		RetryDelay: retryDelay,
	}
	return b
}

// WithTelegram은 텔레그램 알림 설정을 추가합니다.
func (b *ConfigBuilder) WithTelegram(id, token string, chatID int64) *ConfigBuilder {
	b.config.Notifiers.Telegrams = append(b.config.Notifiers.Telegrams, TelegramConfig{
		ID:       id,
		BotToken: token,
		ChatID:   chatID,
	})
	return b
}

// WithDefaultNotifier는 기본 알림 채널 ID를 설정합니다.
func (b *ConfigBuilder) WithDefaultNotifier(notifierID string) *ConfigBuilder {
	b.config.Notifiers.DefaultNotifierID = notifierID
	return b
}

// WithApplication은 API 애플리케이션 설정을 추가합니다.
func (b *ConfigBuilder) WithApplication(id, appKey, defaultNotifierID string) *ConfigBuilder {
	b.config.NotifyAPI.Applications = append(b.config.NotifyAPI.Applications, ApplicationConfig{
		ID:                id,
		AppKey:            appKey,
		DefaultNotifierID: defaultNotifierID,
	})
	return b
}

// WithWSListenPort는 웹서버 수신 포트를 설정합니다.
func (b *ConfigBuilder) WithWSListenPort(port int) *ConfigBuilder {
	b.config.NotifyAPI.WS.ListenPort = port
	return b
}

// WithTLSServer는 TLS 서버 설정을 지정합니다.
func (b *ConfigBuilder) WithTLSServer(enabled bool, certFile, keyFile string) *ConfigBuilder {
	b.config.NotifyAPI.WS.TLSServer = enabled
	b.config.NotifyAPI.WS.TLSCertFile = certFile
	b.config.NotifyAPI.WS.TLSKeyFile = keyFile
	return b
}

// WithCORSOrigins는 CORS AllowOrigins 설정을 지정합니다.
func (b *ConfigBuilder) WithCORSOrigins(origins ...string) *ConfigBuilder {
	b.config.NotifyAPI.CORS.AllowOrigins = origins
	return b
}

// Build는 구성된 AppConfig 객체를 반환합니다.
func (b *ConfigBuilder) Build() *AppConfig {
	return b.config
}

// createTempConfigFile은 테스트용 임시 설정 파일을 생성합니다.
// content는 string, []byte, 또는 JSON 마샬링 가능한 객체를 받습니다.
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

// mustCreateValidConfig는 유효한 설정을 생성하며, 검증 실패 시 테스트를 중단합니다.
func mustCreateValidConfig(t *testing.T) *AppConfig {
	t.Helper()
	cfg := NewConfigBuilder().Build()
	require.NoError(t, cfg.Validate())
	return cfg
}

// =============================================================================
// Basic Tests
// =============================================================================

// TestAppConstants는 애플리케이션 상수값이 올바르게 정의되어 있는지 검증합니다.
func TestAppConstants(t *testing.T) {
	assert.Equal(t, "notify-server", AppName)
	assert.Equal(t, "notify-server.json", AppConfigFileName)
}

// =============================================================================
// SetDefaults Tests
// =============================================================================

// TestSetDefaults_TableDriven은 SetDefaults 메서드가 다양한 초기 상태에서
// 올바르게 기본값을 적용하는지 테이블 드리븐 방식으로 검증합니다.
func TestSetDefaults_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		inputConfig    HTTPRetryConfig
		expectedConfig HTTPRetryConfig
	}{
		{
			name:        "Both Zero - Apply Defaults",
			inputConfig: HTTPRetryConfig{},
			expectedConfig: HTTPRetryConfig{
				MaxRetries: DefaultMaxRetries,
				RetryDelay: DefaultRetryDelay,
			},
		},
		{
			name: "Only MaxRetries Set - Keep MaxRetries, Apply RetryDelay Default",
			inputConfig: HTTPRetryConfig{
				MaxRetries: 5,
			},
			expectedConfig: HTTPRetryConfig{
				MaxRetries: 5,
				RetryDelay: DefaultRetryDelay,
			},
		},
		{
			name: "Only RetryDelay Set - Apply MaxRetries Default, Keep RetryDelay",
			inputConfig: HTTPRetryConfig{
				RetryDelay: "10s",
			},
			expectedConfig: HTTPRetryConfig{
				MaxRetries: DefaultMaxRetries,
				RetryDelay: "10s",
			},
		},
		{
			name: "Both Set - Keep Both",
			inputConfig: HTTPRetryConfig{
				MaxRetries: 5,
				RetryDelay: "10s",
			},
			expectedConfig: HTTPRetryConfig{
				MaxRetries: 5,
				RetryDelay: "10s",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &AppConfig{HTTPRetry: tt.inputConfig}
			config.SetDefaults()
			assert.Equal(t, tt.expectedConfig, config.HTTPRetry)
		})
	}
}

// =============================================================================
// InitAppConfig Tests
// =============================================================================

// TestInitAppConfig_Success는 유효한 설정 파일로부터 AppConfig를 성공적으로 로드하는 시나리오를 검증합니다.
//
// 검증 항목:
//   - JSON 파일 파싱 성공
//   - 기본값 자동 적용
//   - 유효성 검사 통과
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

// TestInitAppConfig_Failure는 잘못된 설정 파일로 인한 실패 시나리오를 검증합니다.
//
// 검증 항목:
//   - 파일 없음 에러 처리
//   - 잘못된 JSON 형식 에러 처리
func TestInitAppConfig_Failure(t *testing.T) {
	tests := []struct {
		name          string
		setupFile     func(t *testing.T) string
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

// TestInitAppConfig_ErrorTypes는 InitAppConfig에서 발생하는 에러의 타입을 검증합니다.
// 커스텀 에러 타입(apperrors.AppError)이 올바르게 반환되는지 확인합니다.
func TestInitAppConfig_ErrorTypes(t *testing.T) {
	t.Run("File Not Found - System Error", func(t *testing.T) {
		cfg, err := InitAppConfigWithFile("non-existent.json")
		assert.Error(t, err)
		assert.Nil(t, cfg)

		// 에러 타입 검증
		var appErr *apperrors.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, apperrors.System, appErr.Type)
	})

	t.Run("Invalid JSON - InvalidInput Error", func(t *testing.T) {
		path := createTempConfigFile(t, "{ invalid")
		_, err := InitAppConfigWithFile(path)
		assert.Error(t, err)

		var appErr *apperrors.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, apperrors.InvalidInput, appErr.Type)
	})
}

// =============================================================================
// JSON Marshaling Tests
// =============================================================================

// TestAppConfig_JSONMarshaling은 AppConfig의 JSON 직렬화/역직렬화를 검증합니다.
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

// =============================================================================
// Edge Case Tests
// =============================================================================

// TestAppConfig_LargeScale은 대용량 데이터 처리를 검증합니다.
func TestAppConfig_LargeScale(t *testing.T) {
	t.Run("Many Notifiers (100개)", func(t *testing.T) {
		// 빈 Telegrams 배열로 시작
		cfg := &AppConfig{
			HTTPRetry: HTTPRetryConfig{
				MaxRetries: DefaultMaxRetries,
				RetryDelay: DefaultRetryDelay,
			},
			Notifiers: NotifierConfig{
				DefaultNotifierID: "telegram0",
				Telegrams:         []TelegramConfig{},
			},
			Tasks: []TaskConfig{},
			NotifyAPI: NotifyAPIConfig{
				WS:           WSConfig{ListenPort: 8080},
				CORS:         CORSConfig{AllowOrigins: []string{"*"}},
				Applications: []ApplicationConfig{},
			},
		}

		for i := 0; i < 100; i++ {
			cfg.Notifiers.Telegrams = append(cfg.Notifiers.Telegrams, TelegramConfig{
				ID:       fmt.Sprintf("telegram%d", i),
				BotToken: fmt.Sprintf("token%d", i),
				ChatID:   int64(i),
			})
		}

		assert.NoError(t, cfg.Validate())
		assert.Len(t, cfg.Notifiers.Telegrams, 100)
	})

	t.Run("Many Applications (50개)", func(t *testing.T) {
		builder := NewConfigBuilder()
		for i := 0; i < 50; i++ {
			builder.WithApplication(
				fmt.Sprintf("app%d", i),
				fmt.Sprintf("key%d", i),
				"telegram1",
			)
		}
		cfg := builder.Build()
		assert.NoError(t, cfg.Validate())
		assert.Len(t, cfg.NotifyAPI.Applications, 50)
	})
}

// TestAppConfig_JSONEdgeCases는 JSON 파싱의 엣지 케이스를 검증합니다.
func TestAppConfig_JSONEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		shouldError bool
		validate    func(t *testing.T, cfg *AppConfig)
	}{
		{
			name:        "Empty JSON Object",
			jsonData:    "{}",
			shouldError: true, // 필수 필드 누락으로 검증 실패
		},
		{
			name: "Extra Fields (Ignored)",
			jsonData: `{
				"debug": true,
				"unknown_field": "value",
				"notifiers": {
					"default_notifier_id": "telegram1",
					"telegrams": [{"id": "telegram1", "bot_token": "token", "chat_id": 123}]
				},
				"tasks": [],
				"notify_api": {
					"ws": {"listen_port": 8080},
					"cors": {"allow_origins": ["*"]},
					"applications": []
				}
			}`,
			shouldError: false,
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.True(t, cfg.Debug)
			},
		},
		{
			name: "Unicode Characters in ID",
			jsonData: `{
				"debug": false,
				"notifiers": {
					"default_notifier_id": "한글ID",
					"telegrams": [{"id": "한글ID", "bot_token": "token", "chat_id": 123}]
				},
				"tasks": [],
				"notify_api": {
					"ws": {"listen_port": 8080},
					"cors": {"allow_origins": ["*"]},
					"applications": []
				}
			}`,
			shouldError: false,
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "한글ID", cfg.Notifiers.DefaultNotifierID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := createTempConfigFile(t, tt.jsonData)
			cfg, err := InitAppConfigWithFile(path)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, cfg)
				}
			}
		})
	}
}

// =============================================================================
// VerifyRecommendations Tests
// =============================================================================

// TestVerifyRecommendations는 VerifyRecommendations 메서드가 권장 설정 미준수 시
// 적절한 경고 로그를 출력하는지 검증합니다.
func TestVerifyRecommendations(t *testing.T) {
	// Logrus 로그 캡처 훅 설정
	hook := test.NewGlobal()
	defer hook.Reset()

	tests := []struct {
		name             string
		configBuilder    *ConfigBuilder
		expectedWarnings []string
	}{
		{
			name: "Default Valid Config - No Warnings",
			configBuilder: NewConfigBuilder().
				WithDebug(true).
				WithHTTPRetry(3, "1s"),
			expectedWarnings: []string{},
		},
		{
			name: "System Reserved Port (< 1024)",
			configBuilder: NewConfigBuilder().
				WithWSListenPort(80),
			expectedWarnings: []string{
				"시스템 예약 포트(1-1023)가 설정되었습니다",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook.Reset()
			cfg := tt.configBuilder.Build()

			cfg.VerifyRecommendations()

			if len(tt.expectedWarnings) == 0 {
				// 경고 로그가 없어야 함
				// 주의: 다른 컴포넌트나 초기화 과정에서 발생한 로그가 있을 수 있으므로 WarnLevel 이상만 체크하거나
				// VerifyRecommendations가 생성하는 특정 로그가 없는지 확인해야 합니다.
				// 여기서는 간단히 WarnLevel 카운트로 체크합니다.
				warnCount := 0
				for _, entry := range hook.Entries {
					if entry.Level == logrus.WarnLevel {
						warnCount++
					}
				}
				assert.Equal(t, 0, warnCount, "예상치 못한 경고 로그가 발생했습니다")
			} else {
				// 예상되는 경고 로그가 존재하는지 확인
				for _, expected := range tt.expectedWarnings {
					found := false
					for _, entry := range hook.Entries {
						if entry.Level == logrus.WarnLevel && strings.Contains(entry.Message, expected) {
							found = true
							break
						}
					}
					assert.True(t, found, "예상 경고 로그를 찾을 수 없습니다: '%s'", expected)
				}
			}
		})
	}
}

// =============================================================================
// ConfigBuilder Tests
// =============================================================================

// TestConfigBuilder는 ConfigBuilder의 동작을 검증합니다.
func TestConfigBuilder(t *testing.T) {
	t.Run("Default Configuration", func(t *testing.T) {
		cfg := NewConfigBuilder().Build()
		assert.NoError(t, cfg.Validate())
		assert.Equal(t, DefaultMaxRetries, cfg.HTTPRetry.MaxRetries)
		assert.Equal(t, DefaultRetryDelay, cfg.HTTPRetry.RetryDelay)
	})

	t.Run("Fluent API Chaining", func(t *testing.T) {
		cfg := NewConfigBuilder().
			WithDebug(true).
			WithHTTPRetry(5, "3s").
			WithTelegram("telegram2", "token2", 456).
			WithApplication("app1", "key1", "telegram1").
			Build()

		assert.True(t, cfg.Debug)
		assert.Equal(t, 5, cfg.HTTPRetry.MaxRetries)
		assert.Equal(t, "3s", cfg.HTTPRetry.RetryDelay)
		assert.Len(t, cfg.Notifiers.Telegrams, 2) // 기본 1개 + 추가 1개
		assert.Len(t, cfg.NotifyAPI.Applications, 1)
	})
}
