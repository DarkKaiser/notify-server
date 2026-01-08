//go:build test

package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestProfileOptions_Defaults는 각 환경별 프로필 함수(New...Options)가
// 기획된 정책대로 정확한 기본값을 반환하는지 모든 필드를 검증합니다.
func TestProfileOptions_Defaults(t *testing.T) {
	t.Parallel()

	const appName = "notify-server-test"

	t.Run("NewProductionOptions (운영 환경)", func(t *testing.T) {
		// When
		opts := NewProductionOptions(appName)

		// Then - 1. Identity & Core Control
		assert.Equal(t, appName, opts.Name)
		assert.Equal(t, InfoLevel, opts.Level, "운영 환경은 Info 레벨이어야 함")
		assert.Empty(t, opts.Dir, "기본값은 빈 문자열이어야 함 (Setup에서 'logs'로 처리됨)")

		// Then - 2. Rotation Policy
		assert.Equal(t, 30, opts.MaxAge, "보관 기간은 30일")
		assert.Equal(t, 100, opts.MaxSizeMB, "파일 크기는 100MB")
		assert.Equal(t, 20, opts.MaxBackups, "백업 개수는 20개")

		// Then - 3. Feature Flags
		assert.True(t, opts.EnableCriticalLog, "장애 대응을 위한 중요 로그 격리 활성화")
		assert.True(t, opts.EnableVerboseLog, "문제 추적을 위한 상세 로그 분리 활성화")
		assert.False(t, opts.EnableConsoleLog, "성능을 위해 콘솔 출력 비활성화")

		// Then - 4. Metadata Detail
		assert.True(t, opts.ReportCaller, "정확한 문제 원인 파악을 위한 호출 위치 기록 활성화")
		assert.Equal(t, "", opts.CallerPathPrefix, "기본값: 전체 경로 출력 (Prefix 없음)")

		// Validation Check
		err := opts.Validate()
		assert.NoError(t, err, "생성된 옵션은 유효해야 함")
	})

	t.Run("NewDevelopmentOptions (개발 환경)", func(t *testing.T) {
		// When
		opts := NewDevelopmentOptions(appName)

		// Then - 1. Identity & Core Control
		assert.Equal(t, appName, opts.Name)
		assert.Equal(t, TraceLevel, opts.Level, "개발 환경은 Trace 레벨이어야 함")
		assert.Empty(t, opts.Dir)

		// Then - 2. Rotation Policy
		assert.Equal(t, 1, opts.MaxAge, "보관 기간은 1일 (디스크 절약)")
		assert.Equal(t, 50, opts.MaxSizeMB, "파일 크기는 50MB")
		assert.Equal(t, 5, opts.MaxBackups, "백업 개수는 5개")

		// Then - 3. Feature Flags
		assert.False(t, opts.EnableCriticalLog, "개발 편의를 위해 로그 통합")
		assert.False(t, opts.EnableVerboseLog, "개발 편의를 위해 로그 통합")
		assert.True(t, opts.EnableConsoleLog, "즉각적 피드백을 위해 콘솔 출력 활성화")

		// Then - 4. Metadata Detail
		assert.True(t, opts.ReportCaller, "디버깅을 위해 호출 위치 추적 활성화")
		assert.Equal(t, "", opts.CallerPathPrefix, "기본값: 전체 경로 출력 (Prefix 없음)")

		// Validation Check
		err := opts.Validate()
		assert.NoError(t, err, "생성된 옵션은 유효해야 함")
	})
}
