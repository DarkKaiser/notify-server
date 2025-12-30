package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFactoryDefaults는 환경별 기본 설정(Factory Functions)이
// 의도된 전략(안정성 vs 생산성)대로 올바르게 생성되는지 검증합니다.
func TestFactoryDefaults(t *testing.T) {
	appName := "notify-server-test"

	t.Run("Production Config Strategy", func(t *testing.T) {
		// When
		cfg := NewProductionConfig(appName)

		// Then
		// 1. Identity
		assert.Equal(t, appName, cfg.Name)

		// 2. Stability & Retention (안정성)
		assert.Equal(t, 30, cfg.MaxAge, "운영 환경은 로그를 30일간 보관해야 합니다")
		assert.True(t, cfg.EnableCriticalLog, "장애 격리를 위해 Critical 로그가 활성화되어야 합니다")
		assert.True(t, cfg.EnableVerboseLog, "상세 분석을 위해 Verbose 로그가 활성화되어야 합니다")

		// 3. Performance (성능)
		assert.False(t, cfg.EnableConsoleLog, "I/O 성능 최적화를 위해 운영 환경에서는 콘솔 출력을 꺼야 합니다")

		// 4. Observability (관측성)
		assert.True(t, cfg.ReportCaller, "스택 트레이스 추적을 위해 호출자 정보가 필요합니다")
	})

	t.Run("Development Config Strategy", func(t *testing.T) {
		// When
		cfg := NewDevelopmentConfig(appName)

		// Then
		// 1. Identity
		assert.Equal(t, appName, cfg.Name)

		// 2. Cleanup (정리)
		assert.Equal(t, 1, cfg.MaxAge, "개발 환경은 디스크 절약을 위해 1일만 보관해야 합니다")

		// 3. Simplicity (단순화)
		assert.False(t, cfg.EnableCriticalLog, "개발 중에는 파일 분리가 번거로우므로 끕니다")
		assert.False(t, cfg.EnableVerboseLog, "개발 중에는 파일 분리가 번거로우므로 끕니다")

		// 4. DX (개발자 경험)
		assert.True(t, cfg.EnableConsoleLog, "즉각적인 피드백을 위해 콘솔 출력이 켜져 있어야 합니다")
		assert.True(t, cfg.ReportCaller, "디버깅을 위해 호출자 정보가 필요합니다")
	})
}
