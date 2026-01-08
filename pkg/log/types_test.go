package log

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// TestTypeAliases verifies that our local types correctly alias logrus types.
// This is a form of contract testing to ensure our abstraction doesn't drift.
func TestTypeAliases(t *testing.T) {
	t.Parallel()

	// 1. Verify Level Constants Mappings
	assert.Equal(t, logrus.PanicLevel, PanicLevel)
	assert.Equal(t, logrus.FatalLevel, FatalLevel)
	assert.Equal(t, logrus.ErrorLevel, ErrorLevel)
	assert.Equal(t, logrus.WarnLevel, WarnLevel)
	assert.Equal(t, logrus.InfoLevel, InfoLevel)
	assert.Equal(t, logrus.DebugLevel, DebugLevel)
	assert.Equal(t, logrus.TraceLevel, TraceLevel)

	// 2. Verify Implements (Interface Compliance)
	// Hook Interface
	var _ Hook = &testHook{}        // Verify our Hook alias matches implementation expectation
	var _ logrus.Hook = &testHook{} // Verify it's indeed compatible with logrus

	// Formatter Interface
	var _ Formatter = &logrus.TextFormatter{} // Verify logrus formatter implements our alias
	var _ Formatter = &logrus.JSONFormatter{}

	// 3. Verify AllLevels
	assert.Equal(t, logrus.AllLevels, AllLevels)
}

// TestInternalConstants verifies package-internal constants.
func TestInternalConstants(t *testing.T) {
	t.Parallel()
	// setup.go에 정의된 fileExt가 "log"인지 확인
	// fileExt는 unexported 상수이므로 같은 패키지 내 테스트 파일에서만 접근 가능
	assert.Equal(t, "log", fileExt)
}

// testHook is a simple struct to verify House interface compliance
type testHook struct{}

func (h *testHook) Levels() []Level     { return AllLevels }
func (h *testHook) Fire(e *Entry) error { return nil }
