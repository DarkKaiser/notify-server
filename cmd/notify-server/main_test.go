package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// 메타데이터 및 상수 검증 (Metadata & Constants Validation)
// =============================================================================

// TestAppMetadata는 애플리케이션의 기본 메타데이터 설정이 올바른지 검증합니다.
func TestAppMetadata(t *testing.T) {
	t.Run("AppVersion 검증", func(t *testing.T) {
		assert.NotEmpty(t, Version, "애플리케이션 버전(Version)은 비어있을 수 없습니다")

		// 기본값("dev") 또는 Semantic Versioning 형식(vX.Y.Z)을 준수해야 함
		if Version != "dev" {
			assert.Regexp(t, `^v?\d+\.\d+\.\d+(?:-.*)?$`, Version, "버전은 Semantic Versioning 표준 형식을 따라야 합니다")
		}
	})

	t.Run("AppName 검증", func(t *testing.T) {
		assert.Equal(t, "notify-server", config.AppName, "애플리케이션 이름은 'notify-server'여야 합니다")
		assert.NotContains(t, config.AppName, " ", "애플리케이션 이름에는 공백이 포함될 수 없습니다")
	})

	t.Run("ConfigFileName 검증", func(t *testing.T) {
		expected := "notify-server.json"
		assert.Equal(t, expected, config.AppConfigFileName, "설정 파일명은 '%s'여야 합니다", expected)
	})
}

// TestBuildInfo는 빌드 타임에 주입되는 정보들의 기본 상태를 검증합니다.
func TestBuildInfo(t *testing.T) {
	tests := []struct {
		name  string
		value string
		field string
	}{
		{"Version", Version, "Version"},
		{"BuildDate", BuildDate, "BuildDate"},
		{"BuildNumber", BuildNumber, "BuildNumber"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.value, "%s 필드는 초기화되어야 합니다", tt.field)
		})
	}
}

// =============================================================================
// 배너 검증 (Banner Validation)
// =============================================================================

// TestBanner는 서버 시작 시 출력되는 배너의 형식과 내용이 올바른지 검증합니다.
func TestBanner(t *testing.T) {
	t.Run("템플릿 형식 검증", func(t *testing.T) {
		assert.Contains(t, banner, "%s", "배너 템플릿에는 버전 포맷팅을 위한 '%s'가 포함되어야 합니다")
		assert.Contains(t, banner, "DarkKaiser", "배너에는 개발자/조직명(DarkKaiser)이 포함되어야 합니다")
	})

	t.Run("출력 포맷팅 검증", func(t *testing.T) {
		output := fmt.Sprintf(banner, Version)
		assert.Contains(t, output, Version, "최종 출력된 배너에는 실제 버전 정보가 포함되어야 합니다")
		assert.NotContains(t, output, "%s", "최종 출력된 배너에는 포맷 지정자가 남아있지 않아야 합니다")
	})
}

// =============================================================================
// 설정 로드 통합 테스트 (Configuration Loading Integration Test)
// =============================================================================

// TestInitAppConfig는 설정 파일 로드 로직을 다양한 시나리오(정상, 오류, 경계값)에서 검증합니다.
// 실제 파일 시스템 I/O를 수반하므로 임시 파일을 생성하여 테스트합니다.
func TestInitAppConfig(t *testing.T) {
	// 1. 정상 케이스
	t.Run("Normal_ValidConfig", func(t *testing.T) {
		f := createTempConfigFile(t, validConfigJSON())
		defer os.Remove(f)

		cfg, err := config.InitAppConfigWithFile(f)
		require.NoError(t, err, "유효한 설정 파일 로드 시 에러가 발생하면 안 됩니다")
		require.NotNil(t, cfg, "설정 객체는 nil이 아니어야 합니다")

		// 주요 설정값 검증
		assert.True(t, cfg.Debug, "Debug 모드가 설정 파일대로 로드되어야 합니다")
		assert.Equal(t, "test-notifier", cfg.Notifiers.DefaultNotifierID)
	})

	// 2. 오류 케이스: 파일 부재
	t.Run("Error_FileNotFound", func(t *testing.T) {
		nonExistentFile := "ghost_config_file_12345.json"
		cfg, err := config.InitAppConfigWithFile(nonExistentFile)

		assert.Error(t, err, "존재하지 않는 파일 로드 시 에러를 반환해야 합니다")
		assert.Nil(t, cfg)
		// 에러 메시지 검증 (OS별 메시지 차이 고려하여 핵심 키워드 체크)
		errMsg := err.Error()
		isPathError := strings.Contains(errMsg, nonExistentFile) || strings.Contains(errMsg, "no such file") || strings.Contains(errMsg, "지정된 파일을 찾을 수 없습니다")
		assert.True(t, isPathError, "에러 메시지에 파일명이나 '찾을 수 없음' 내용이 포함되어야 합니다")
	})

	// 3. 오류 케이스: 잘못된 JSON
	t.Run("Error_InvalidJSON", func(t *testing.T) {
		f := createTempConfigFile(t, `{"debug": true, "broken_json...`)
		defer os.Remove(f)

		cfg, err := config.InitAppConfigWithFile(f)
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "JSON", "JSON 파싱 에러임을 명시해야 합니다")
	})

	// 4. 오류 케이스: 빈 파일 및 빈 객체
	t.Run("Error_EmptyContent", func(t *testing.T) {
		// Case A: 완전 빈 파일
		f1 := createTempConfigFile(t, "")
		defer os.Remove(f1)
		_, err1 := config.InitAppConfigWithFile(f1)
		assert.Error(t, err1, "빈 파일은 JSON 파싱 에러를 유발해야 합니다")

		// Case B: 빈 JSON 객체 ({}) -> 유효성 검사 실패 예상
		f2 := createTempConfigFile(t, "{}")
		defer os.Remove(f2)
		_, err2 := config.InitAppConfigWithFile(f2)
		assert.Error(t, err2, "필수 설정이 누락된 빈 객체는 유효성 검사를 통과하지 못해야 합니다")
	})
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

func createTempConfigFile(t *testing.T, content string) string {
	t.Helper()
	tmp, err := os.CreateTemp("", "test_cfg_*.json")
	require.NoError(t, err, "테스트용 임시 파일 생성 실패")

	_, err = tmp.WriteString(content)
	require.NoError(t, err, "임시 파일 내용 기록 실패")

	err = tmp.Close()
	require.NoError(t, err, "임시 파일 핸들 닫기 실패")

	return tmp.Name()
}

func validConfigJSON() string {
	return `{
		"debug": true,
		"notifiers": {
			"default_notifier_id": "test-notifier",
			"telegrams": [
				{ "id": "test-notifier", "bot_token": "token", "chat_id": 12345 }
			]
		},
		"tasks": [],
		"notify_api": {
			"ws": { "tls_server": false, "listen_port": 18080 },
			"cors": { "allow_origins": ["*"] },
			"applications": []
		}
	}`
}
