package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateCronExpression(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		wantErr bool
	}{
		{"유효한 Cron (초 포함)", "0 */5 * * * *", false},
		{"유효한 Cron (매분)", "0 * * * * *", false},
		{"유효한 Cron (매일 9시)", "0 0 9 * * *", false},
		{"잘못된 Cron (필드 부족)", "* * *", true},
		{"잘못된 Cron (범위 초과)", "70 * * * * *", true},
		{"빈 문자열", "", true},
		{"잘못된 형식", "invalid cron", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCronExpression(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCronExpression() error = %v, wantErr %v", err, tt.wantErr)
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
		{"유효한 포트", 8080, false},
		{"최소 포트", 1, false},
		{"최대 포트", 65535, false},
		{"일반적인 포트", 2443, false},
		{"0 포트", 0, true},
		{"음수 포트", -1, true},
		{"범위 초과", 65536, true},
		{"범위 초과 (큰 값)", 100000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePort(tt.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePort() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration string
		wantErr  bool
	}{
		{"초 단위", "2s", false},
		{"밀리초 단위", "100ms", false},
		{"분 단위", "1m", false},
		{"시간 단위", "1h", false},
		{"복합 단위", "1m30s", false},
		{"잘못된 형식", "2 seconds", true},
		{"빈 문자열", "", true},
		{"숫자만", "123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDuration(tt.duration)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDuration() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFileExists(t *testing.T) {
	// 테스트용 임시 파일 생성
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tests := []struct {
		name     string
		path     string
		warnOnly bool
		wantErr  bool
	}{
		{"존재하는 파일 (에러 모드)", tmpFile.Name(), false, false},
		{"존재하는 파일 (경고 모드)", tmpFile.Name(), true, false},
		{"존재하지 않는 파일 (에러 모드)", "/nonexistent/file.txt", false, true},
		{"존재하지 않는 파일 (경고 모드)", "/nonexistent/file.txt", true, false},
		{"빈 경로 (에러 모드)", "", false, false},
		{"빈 경로 (경고 모드)", "", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFileExists(tt.path, tt.warnOnly)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFileExists() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAppConfig_Validate_InvalidDuration(t *testing.T) {
	t.Run("잘못된 HTTP Retry Duration", func(t *testing.T) {
		appConfig := &AppConfig{
			HTTPRetry: HTTPRetryConfig{
				MaxRetries: 3,
				RetryDelay: "2 seconds", // Invalid!
			},
			Notifiers: NotifierConfig{
				DefaultNotifierID: "telegram1",
				Telegrams: []TelegramConfig{
					{ID: "telegram1", BotToken: "token1", ChatID: 123},
				},
			},
			Tasks: []TaskConfig{},
			NotifyAPI: NotifyAPIConfig{
				WS:           WSConfig{TLSServer: false, ListenPort: 2443},
				Applications: []ApplicationConfig{},
			},
		}

		err := appConfig.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "HTTP Retry")
		assert.Contains(t, err.Error(), "duration")
	})
}

func TestAppConfig_Validate_InvalidCronExpression(t *testing.T) {
	t.Run("잘못된 Cron 표현식", func(t *testing.T) {
		appConfig := &AppConfig{
			HTTPRetry: HTTPRetryConfig{
				MaxRetries: 3,
				RetryDelay: "2s",
			},
			Notifiers: NotifierConfig{
				DefaultNotifierID: "telegram1",
				Telegrams: []TelegramConfig{
					{ID: "telegram1", BotToken: "token1", ChatID: 123},
				},
			},
			Tasks: []TaskConfig{
				{
					ID:    "test-task",
					Title: "Test Task",
					Commands: []TaskCommandConfig{
						{
							ID:    "test-command",
							Title: "Test Command",
							Scheduler: struct {
								Runnable bool   `json:"runnable"`
								TimeSpec string `json:"time_spec"`
							}{
								Runnable: true,
								TimeSpec: "invalid cron", // Invalid!
							},
							DefaultNotifierID: "telegram1",
						},
					},
				},
			},
			NotifyAPI: NotifyAPIConfig{
				WS:           WSConfig{TLSServer: false, ListenPort: 2443},
				Applications: []ApplicationConfig{},
			},
		}

		err := appConfig.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Cron")
		assert.Contains(t, err.Error(), "test-task")
		assert.Contains(t, err.Error(), "test-command")
	})
}

func TestAppConfig_Validate_InvalidPort(t *testing.T) {
	t.Run("잘못된 포트 번호", func(t *testing.T) {
		appConfig := &AppConfig{
			HTTPRetry: HTTPRetryConfig{
				MaxRetries: 3,
				RetryDelay: "2s",
			},
			Notifiers: NotifierConfig{
				DefaultNotifierID: "telegram1",
				Telegrams: []TelegramConfig{
					{ID: "telegram1", BotToken: "token1", ChatID: 123},
				},
			},
			Tasks: []TaskConfig{},
			NotifyAPI: NotifyAPIConfig{
				WS:           WSConfig{TLSServer: false, ListenPort: 70000}, // Invalid!
				Applications: []ApplicationConfig{},
			},
		}

		err := appConfig.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "포트")
	})
}

func TestAppConfig_Validate_ValidCronExpression(t *testing.T) {
	t.Run("유효한 Cron 표현식", func(t *testing.T) {
		appConfig := &AppConfig{
			HTTPRetry: HTTPRetryConfig{
				MaxRetries: 3,
				RetryDelay: "2s",
			},
			Notifiers: NotifierConfig{
				DefaultNotifierID: "telegram1",
				Telegrams: []TelegramConfig{
					{ID: "telegram1", BotToken: "token1", ChatID: 123},
				},
			},
			Tasks: []TaskConfig{
				{
					ID:    "test-task",
					Title: "Test Task",
					Commands: []TaskCommandConfig{
						{
							ID:    "test-command",
							Title: "Test Command",
							Scheduler: struct {
								Runnable bool   `json:"runnable"`
								TimeSpec string `json:"time_spec"`
							}{
								Runnable: true,
								TimeSpec: "0 */5 * * * *", // Valid!
							},
							DefaultNotifierID: "telegram1",
						},
					},
				},
			},
			NotifyAPI: NotifyAPIConfig{
				WS: WSConfig{TLSServer: false, ListenPort: 2443},
				Applications: []ApplicationConfig{
					{
						ID:                "test-app",
						Title:             "Test App",
						DefaultNotifierID: "telegram1",
						AppKey:            "test-key",
					},
				},
			},
		}

		err := appConfig.Validate()
		assert.NoError(t, err)
	})
}
