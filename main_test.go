package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/g"
	"github.com/stretchr/testify/assert"
)

// TestAppVersion은 애플리케이션 버전이 설정되어 있는지 확인합니다.
func TestAppVersion(t *testing.T) {
	assert.NotEmpty(t, g.AppVersion, "애플리케이션 버전이 설정되어야 합니다")
	assert.Regexp(t, `^\d+\.\d+\.\d+$`, g.AppVersion, "버전은 x.y.z 형식이어야 합니다")
}

// TestAppName은 애플리케이션 이름이 설정되어 있는지 확인합니다.
func TestAppName(t *testing.T) {
	assert.NotEmpty(t, g.AppName, "애플리케이션 이름이 설정되어야 합니다")
	assert.Equal(t, "notify-server", g.AppName, "애플리케이션 이름이 일치해야 합니다")
}

// TestBannerFormat은 배너 형식이 올바른지 확인합니다.
func TestBannerFormat(t *testing.T) {
	assert.Contains(t, banner, "v%s", "배너에 버전 플레이스홀더가 있어야 합니다")
	assert.Contains(t, banner, "DarkKaiser", "배너에 개발자 이름이 있어야 합니다")
	assert.NotEmpty(t, banner, "배너가 비어있지 않아야 합니다")
}

// TestBannerOutput은 배너 출력이 정상적으로 작동하는지 확인합니다.
func TestBannerOutput(t *testing.T) {
	formattedBanner := fmt.Sprintf(banner, g.AppVersion)

	assert.Contains(t, formattedBanner, g.AppVersion, "포맷된 배너에 버전이 포함되어야 합니다")
	assert.Contains(t, formattedBanner, "DarkKaiser", "포맷된 배너에 개발자 이름이 포함되어야 합니다")
	assert.NotContains(t, formattedBanner, "%s", "포맷된 배너에 플레이스홀더가 남아있지 않아야 합니다")
}

// TestConfigFileName은 설정 파일 이름이 올바른지 확인합니다.
func TestConfigFileName(t *testing.T) {
	expectedFileName := g.AppName + ".json"
	assert.Equal(t, expectedFileName, g.AppConfigFileName, "설정 파일 이름이 올바라야 합니다")
	assert.Equal(t, "notify-server.json", g.AppConfigFileName, "설정 파일 이름이 notify-server.json이어야 합니다")
}

// TestEnvironmentSetup은 환경 설정이 가능한지 확인합니다.
func TestEnvironmentSetup(t *testing.T) {
	t.Run("설정 파일 존재 여부", func(t *testing.T) {
		// 설정 파일이 존재하는지 확인 (선택적 테스트)
		_, err := os.Stat(g.AppConfigFileName)
		if err == nil {
			t.Logf("설정 파일 '%s'이 존재합니다", g.AppConfigFileName)
		} else if os.IsNotExist(err) {
			t.Logf("설정 파일 '%s'이 존재하지 않습니다 (테스트 환경에서는 정상)", g.AppConfigFileName)
		} else {
			t.Logf("설정 파일 확인 중 에러: %v", err)
		}
		// 이 테스트는 실패하지 않음 - 정보 제공용
	})
}

// TestApplicationMetadata은 애플리케이션 메타데이터를 검증합니다.
func TestApplicationMetadata(t *testing.T) {
	t.Run("버전 형식", func(t *testing.T) {
		// 버전이 비어있지 않고 올바른 형식인지 확인
		assert.NotEmpty(t, g.AppVersion, "버전이 설정되어야 합니다")

		// 간단한 버전 형식 검증 (예: "0.0.3")
		versionParts := len(g.AppVersion)
		assert.Greater(t, versionParts, 4, "버전 문자열이 최소 길이를 만족해야 합니다")
	})

	t.Run("애플리케이션 이름", func(t *testing.T) {
		assert.Equal(t, "notify-server", g.AppName, "애플리케이션 이름이 정확해야 합니다")
		assert.NotContains(t, g.AppName, " ", "애플리케이션 이름에 공백이 없어야 합니다")
	})
}

// TestInitAppConfig은 설정 파일 로딩을 테스트합니다.
func TestInitAppConfig(t *testing.T) {
	t.Run("유효한 설정 파일 로딩", func(t *testing.T) {
		// 임시 설정 파일 생성
		tempFile := createTempConfigFile(t, validConfigJSON())
		defer os.Remove(tempFile)

		// 설정 로딩
		appConfig, err := g.InitAppConfigWithFile(tempFile)
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
		appConfig, err := g.InitAppConfigWithFile("nonexistent_file_12345.json")

		// 검증
		assert.Error(t, err, "에러가 반환되어야 합니다")
		assert.Nil(t, appConfig, "설정 객체는 nil이어야 합니다")

		// 파일이 존재하지 않는 에러 메시지 확인
		errMsg := err.Error()
		assert.True(t,
			strings.Contains(errMsg, "nonexistent_file_12345.json") ||
				strings.Contains(errMsg, "no such file") ||
				strings.Contains(errMsg, "cannot find"),
			"파일을 찾을 수 없다는 에러 메시지가 있어야 합니다")
	})

	t.Run("잘못된 JSON 형식", func(t *testing.T) {
		// 잘못된 JSON 파일 생성
		invalidJSON := `{"debug": true, "invalid json`
		tempFile := createTempConfigFile(t, invalidJSON)
		defer os.Remove(tempFile)

		// 잘못된 JSON 파일 로딩 시도
		appConfig, err := g.InitAppConfigWithFile(tempFile)

		// 검증
		assert.Error(t, err, "JSON 파싱 에러가 반환되어야 합니다")
		assert.Nil(t, appConfig, "설정 객체는 nil이어야 합니다")
	})
}

// 헬퍼 함수: 유효한 설정 JSON 반환
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
			"applications": []
		}
	}`
}

// 헬퍼 함수: 임시 설정 파일 생성
func createTempConfigFile(t *testing.T, content string) string {
	tempFile, err := os.CreateTemp("", "test_config_*.json")
	assert.NoError(t, err, "임시 파일 생성 실패")

	_, err = tempFile.WriteString(content)
	assert.NoError(t, err, "임시 파일 쓰기 실패")

	err = tempFile.Close()
	assert.NoError(t, err, "임시 파일 닫기 실패")

	return tempFile.Name()
}
