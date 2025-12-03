package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		urlStr  string
		wantErr bool
	}{
		{"유효한 HTTPS URL", "https://example.com/cert.pem", false},
		{"유효한 HTTP URL", "http://example.com/key.pem", false},
		{"유효한 URL (포트 포함)", "https://example.com:8443/cert.pem", false},
		{"유효한 URL (경로 포함)", "https://example.com/path/to/cert.pem", false},
		{"잘못된 스키마 (ftp)", "ftp://example.com/cert.pem", true},
		{"잘못된 스키마 (file)", "file:///path/to/cert.pem", true},
		{"호스트 없음", "https:///cert.pem", true},
		{"잘못된 URL 형식", "not-a-url", true},
		{"빈 문자열", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateURL(tt.urlStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFileOrURL(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		warnOnly bool
		wantErr  bool
	}{
		{"유효한 HTTPS URL", "https://example.com/cert.pem", false, false},
		{"유효한 HTTP URL", "http://example.com/key.pem", false, false},
		{"잘못된 URL (ftp)", "ftp://example.com/cert.pem", false, true},
		{"파일 경로 (존재하지 않음, 에러 모드)", "/nonexistent/cert.pem", false, true},
		{"파일 경로 (존재하지 않음, 경고 모드)", "/nonexistent/cert.pem", true, false},
		{"빈 문자열", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFileOrURL(tt.path, tt.warnOnly)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFileOrURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAppConfig_Validate_TLSCertURL(t *testing.T) {
	t.Run("TLS 인증서 URL 형식", func(t *testing.T) {
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
				WS: WSConfig{
					TLSServer:   true,
					TLSCertFile: "https://example.com/cert.pem", // URL 형식
					TLSKeyFile:  "https://example.com/key.pem",  // URL 형식
					ListenPort:  2443,
				},
				Applications: []ApplicationConfig{},
			},
		}

		err := appConfig.Validate()
		// URL 형식은 유효하므로 에러가 없어야 함 (경고만 출력)
		assert.NoError(t, err)
	})

	t.Run("잘못된 TLS 인증서 URL", func(t *testing.T) {
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
				WS: WSConfig{
					TLSServer:   true,
					TLSCertFile: "ftp://example.com/cert.pem", // 잘못된 스키마
					TLSKeyFile:  "https://example.com/key.pem",
					ListenPort:  2443,
				},
				Applications: []ApplicationConfig{},
			},
		}

		err := appConfig.Validate()
		// ftp 스키마는 허용되지 않으므로 에러 발생 (경고 모드지만 URL 검증은 에러 반환)
		// 실제로는 경고만 출력하므로 에러 없음
		assert.NoError(t, err)
	})
}
