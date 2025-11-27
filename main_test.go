package main

import (
	"testing"

	"github.com/darkkaiser/notify-server/g"
	"github.com/stretchr/testify/assert"
)

// TestAppVersion은 애플리케이션 버전이 설정되어 있는지 확인합니다.
func TestAppVersion(t *testing.T) {
	assert.NotEmpty(t, g.AppVersion, "애플리케이션 버전이 설정되어야 합니다")
}

// TestAppName은 애플리케이션 이름이 설정되어 있는지 확인합니다.
func TestAppName(t *testing.T) {
	assert.NotEmpty(t, g.AppName, "애플리케이션 이름이 설정되어야 합니다")
}

// TestBannerFormat은 배너 형식이 올바른지 확인합니다.
func TestBannerFormat(t *testing.T) {
	assert.Contains(t, banner, "v%s", "배너에 버전 플레이스홀더가 있어야 합니다")
	assert.Contains(t, banner, "DarkKaiser", "배너에 개발자 이름이 있어야 합니다")
	assert.NotEmpty(t, banner, "배너가 비어있지 않아야 합니다")
}
