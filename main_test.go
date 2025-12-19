package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Helpers
// =============================================================================

// createTempConfigFile은 테스트용 임시 설정 파일을 생성합니다.
// content는 JSON 문자열이어야 합니다.
func createTempConfigFile(t *testing.T, content string) string {
	t.Helper()
	tempFile, err := os.CreateTemp("", "test_config_*.json")
	require.NoError(t, err, "임시 파일 생성 실패")

	_, err = tempFile.WriteString(content)
	require.NoError(t, err, "임시 파일 쓰기 실패")

	err = tempFile.Close()
	require.NoError(t, err, "임시 파일 닫기 실패")

	return tempFile.Name()
}

// validConfigJSON은 유효한 설정 JSON 문자열을 반환합니다.
func validConfigJSON() string {
	return `{
		"debug": true,
		"notifiers": {
			"default_notifier_id": "test-notifier",
			"telegrams": [
				{
					"id": "test-notifier",
					"bot_token": "test-token",
					"chat_id": 12345
				}
			]
		},
		"tasks": [],
		"notify_api": {
			"ws": {
				"tls_server": false,
				"listen_port": 18080
			},
			"cors": {
				"allow_origins": ["*"]
			},
			"applications": []
		}
	}`
}

// =============================================================================
// Application Metadata Tests
// =============================================================================

// TestAppVersion은 애플리케이션 버전이 올바르게 설정되어 있는지 검증합니다.
//
// 검증 항목:
//   - 버전이 비어있지 않음
//   - "dev" 또는 Semantic Versioning 형식 (vX.Y.Z)
func TestAppVersion(t *testing.T) {
	assert.NotEmpty(t, Version, "애플리케이션 버전이 설정되어야 합니다")

	// Version은 기본값이 "dev"이므로 정규식 검사는 "dev" 또는 Git Tag 형식(vX.Y.Z...)이어야 함
	if Version != "dev" {
		assert.Regexp(t, `^v?\d+\.\d+\.\d+(?:-.*)?$`, Version, "버전은 Semantic Versioning 형식이어야 합니다")
	}
}

// TestAppName은 애플리케이션 이름이 올바르게 설정되어 있는지 검증합니다.
func TestAppName(t *testing.T) {
	assert.NotEmpty(t, config.AppName, "애플리케이션 이름이 설정되어야 합니다")
	assert.Equal(t, "notify-server", config.AppName, "애플리케이션 이름이 일치해야 합니다")
	assert.NotContains(t, config.AppName, " ", "애플리케이션 이름에 공백이 없어야 합니다")
}

// TestConfigFileName은 설정 파일 이름이 올바른지 검증합니다.
func TestConfigFileName(t *testing.T) {
	expectedFileName := config.AppName + ".json"
	assert.Equal(t, expectedFileName, config.AppConfigFileName, "설정 파일 이름이 올바라야 합니다")
	assert.Equal(t, "notify-server.json", config.AppConfigFileName, "설정 파일 이름이 notify-server.json이어야 합니다")
}

// =============================================================================
// Banner Tests
// =============================================================================

// TestBanner는 배너 형식과 출력이 올바른지 검증합니다.
//
// 검증 항목:
//   - 배너 형식 (플레이스홀더, 개발자 이름)
//   - 배너 출력 (버전 포함, 플레이스홀더 제거)
func TestBanner(t *testing.T) {
	t.Run("배너 형식", func(t *testing.T) {
		assert.Contains(t, banner, "%s", "배너에 버전 플레이스홀더가 있어야 합니다")
		assert.Contains(t, banner, "DarkKaiser", "배너에 개발자 이름이 있어야 합니다")
		assert.NotEmpty(t, banner, "배너가 비어있지 않아야 합니다")
	})

	t.Run("배너 출력", func(t *testing.T) {
		formattedBanner := fmt.Sprintf(banner, Version)

		assert.Contains(t, formattedBanner, Version, "포맷된 배너에 버전이 포함되어야 합니다")
		assert.Contains(t, formattedBanner, "DarkKaiser", "포맷된 배너에 개발자 이름이 포함되어야 합니다")
		assert.NotContains(t, formattedBanner, "%s", "포맷된 배너에 플레이스홀더가 남아있지 않아야 합니다")
	})
}

// =============================================================================
// Configuration Loading Tests
// =============================================================================

// TestInitAppConfig는 설정 파일 로딩의 다양한 시나리오를 검증합니다.
//
// 검증 항목:
//   - 유효한 설정 파일 로딩 성공
//   - 존재하지 않는 파일 에러 처리
//   - 잘못된 JSON 형식 에러 처리
//   - 빈 파일 에러 처리
func TestInitAppConfig(t *testing.T) {
	t.Run("유효한 설정 파일 로딩", func(t *testing.T) {
		// 임시 설정 파일 생성
		tempFile := createTempConfigFile(t, validConfigJSON())
		defer os.Remove(tempFile)

		// 설정 로딩
		appConfig, err := config.InitAppConfigWithFile(tempFile)
		assert.NoError(t, err)

		// 검증
		assert.NotNil(t, appConfig, "설정이 로드되어야 합니다")
		assert.True(t, appConfig.Debug, "Debug 모드가 활성화되어야 합니다")
		assert.Equal(t, "test-notifier", appConfig.Notifiers.DefaultNotifierID, "기본 NotifierID가 일치해야 합니다")
		assert.Equal(t, 1, len(appConfig.Notifiers.Telegrams), "Telegram notifier가 1개 있어야 합니다")
		assert.Equal(t, "test-notifier", appConfig.Notifiers.Telegrams[0].ID, "Telegram ID가 일치해야 합니다")
	})

	t.Run("존재하지 않는 설정 파일", func(t *testing.T) {
		// 존재하지 않는 파일로 설정 로딩 시도
		appConfig, err := config.InitAppConfigWithFile("nonexistent_file_12345.json")

		// 검증
		assert.Error(t, err, "에러가 반환되어야 합니다")
		assert.Nil(t, appConfig, "설정 객체는 nil이어야 합니다")

		// 파일이 존재하지 않는 에러 메시지 확인
		errMsg := err.Error()
		assert.True(t,
			strings.Contains(errMsg, "nonexistent_file_12345.json") ||
				strings.Contains(errMsg, "no such file") ||
				strings.Contains(errMsg, "cannot find") ||
				strings.Contains(errMsg, "파일을 열 수 없습니다"),
			"파일을 찾을 수 없다는 에러 메시지가 있어야 합니다")
	})

	t.Run("잘못된 JSON 형식", func(t *testing.T) {
		// 잘못된 JSON 파일 생성
		invalidJSON := `{"debug": true, "invalid json`
		tempFile := createTempConfigFile(t, invalidJSON)
		defer os.Remove(tempFile)

		// 잘못된 JSON 파일 로딩 시도
		appConfig, err := config.InitAppConfigWithFile(tempFile)

		// 검증
		assert.Error(t, err, "JSON 파싱 에러가 반환되어야 합니다")
		assert.Nil(t, appConfig, "설정 객체는 nil이어야 합니다")
		assert.Contains(t, err.Error(), "JSON", "JSON 파싱 에러 메시지가 있어야 합니다")
	})

	t.Run("빈 파일", func(t *testing.T) {
		// 빈 파일 생성
		tempFile := createTempConfigFile(t, "")
		defer os.Remove(tempFile)

		// 빈 파일 로딩 시도
		appConfig, err := config.InitAppConfigWithFile(tempFile)

		// 검증
		assert.Error(t, err, "에러가 반환되어야 합니다")
		assert.Nil(t, appConfig, "설정 객체는 nil이어야 합니다")
	})

	t.Run("빈 JSON 객체", func(t *testing.T) {
		// 빈 JSON 객체 파일 생성
		emptyJSON := `{}`
		tempFile := createTempConfigFile(t, emptyJSON)
		defer os.Remove(tempFile)

		// 빈 JSON 객체 파일 로딩 시도
		appConfig, err := config.InitAppConfigWithFile(tempFile)

		// 검증 (빈 객체는 유효성 검사에서 실패해야 함)
		assert.Error(t, err, "유효성 검사 에러가 반환되어야 합니다")
		assert.Nil(t, appConfig, "설정 객체는 nil이어야 합니다")
	})
}

// =============================================================================
// Environment Tests
// =============================================================================

// TestEnvironmentSetup은 환경 설정 상태를 확인합니다.
// 이 테스트는 정보 제공용이며 실패하지 않습니다.
func TestEnvironmentSetup(t *testing.T) {
	t.Run("설정 파일 존재 여부", func(t *testing.T) {
		// 설정 파일이 존재하는지 확인 (선택적 테스트)
		_, err := os.Stat(config.AppConfigFileName)
		if err == nil {
			t.Logf("설정 파일 '%s'이 존재합니다", config.AppConfigFileName)
		} else if os.IsNotExist(err) {
			t.Logf("설정 파일 '%s'이 존재하지 않습니다 (테스트 환경에서는 정상)", config.AppConfigFileName)
		} else {
			t.Logf("설정 파일 확인 중 에러: %v", err)
		}
		// 이 테스트는 실패하지 않음 - 정보 제공용
	})
}

// =============================================================================
// Build Info Tests
// =============================================================================

// TestBuildInfo는 빌드 정보 변수들이 설정되어 있는지 검증합니다.
//
// 검증 항목:
//   - Version, BuildDate, BuildNumber 변수 존재
//   - 기본값 확인
func TestBuildInfo(t *testing.T) {
	t.Run("Version", func(t *testing.T) {
		assert.NotEmpty(t, Version, "Version이 설정되어야 합니다")
	})

	t.Run("BuildDate", func(t *testing.T) {
		assert.NotEmpty(t, BuildDate, "BuildDate가 설정되어야 합니다")
		// 기본값은 "unknown"
	})

	t.Run("BuildNumber", func(t *testing.T) {
		assert.NotEmpty(t, BuildNumber, "BuildNumber가 설정되어야 합니다")
		// 기본값은 "0"
	})
}
