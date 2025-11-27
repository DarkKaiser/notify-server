package main

import (
	"fmt"
	"os"
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
