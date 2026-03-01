package log

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// TestTypeAliases verifies that our local types correctly alias logrus types.
// This ensures that the abstraction layer provided by pkg/log is transparent
// and fully compatible with the underlying logrus library.
func TestTypeAliases(t *testing.T) {
	t.Parallel()

	// 1. Verify Level Constants Mappings
	// Check if the values of the constants are exactly the same.
	assert.Equal(t, logrus.PanicLevel, PanicLevel)
	assert.Equal(t, logrus.FatalLevel, FatalLevel)
	assert.Equal(t, logrus.ErrorLevel, ErrorLevel)
	assert.Equal(t, logrus.WarnLevel, WarnLevel)
	assert.Equal(t, logrus.InfoLevel, InfoLevel)
	assert.Equal(t, logrus.DebugLevel, DebugLevel)
	assert.Equal(t, logrus.TraceLevel, TraceLevel)

	// 2. Verify AllLevels
	assert.Equal(t, logrus.AllLevels, AllLevels)
}

// TestTypeCompatibility verifies that types defined in pkg/log are truly aliases
// and can be interchangeably used with logrus types.
func TestTypeCompatibility(t *testing.T) {
	t.Parallel()

	// 1. Fields Interchangeability
	// log.Fields should be assignable to logrus.Fields and vice versa.
	localFields := Fields{"key": "value"}
	var logrusFields logrus.Fields = localFields
	var backToLocal Fields = logrusFields

	assert.Equal(t, localFields, Fields(logrusFields))
	assert.Equal(t, logrusFields, logrus.Fields(backToLocal))

	// 2. Level Interchangeability
	localLevel := InfoLevel
	var logrusLevel logrus.Level = localLevel
	var backToLocalLevel Level = logrusLevel

	assert.Equal(t, localLevel, Level(logrusLevel))
	assert.Equal(t, logrusLevel, logrus.Level(backToLocalLevel))

	// 3. Logger Interchangeability
	// Pointers should be exchangeable.
	localLogger := &Logger{}
	var logrusLogger *logrus.Logger = localLogger
	var backToLocalLogger *Logger = logrusLogger

	assert.NotNil(t, logrusLogger)
	assert.NotNil(t, backToLocalLogger)

	// 4. Entry Interchangeability
	localEntry := &Entry{}
	var logrusEntry *logrus.Entry = localEntry
	var backToLocalEntry *Entry = logrusEntry

	assert.NotNil(t, logrusEntry)
	assert.NotNil(t, backToLocalEntry)
}

// TestInterfaceCompliance ensures that our aliases satisfy imperative interfaces.
func TestInterfaceCompliance(t *testing.T) {
	t.Parallel()

	// 1. Hook Interface
	// Our 'Hook' alias should be compatible with logrus.Hook interface requirements.
	var _ Hook = &minimalHook{}        // Verify our implementation satisfies our alias
	var _ logrus.Hook = &minimalHook{} // Verify it satisfies the original interface

	// 2. Formatter Interface
	// Standard logrus formatters should satisfy our 'Formatter' alias.
	var _ Formatter = &logrus.TextFormatter{}
	var _ Formatter = &logrus.JSONFormatter{}

	// Our aliases for formatters should also satisfy the interface.
	var _ Formatter = &TextFormatter{}
	var _ Formatter = &JSONFormatter{}
}

// minimalHook is a minimal Hook implementation for interface compliance testing.
// NOTE: log_test.go (//go:build test) has a more complete `testHook` for integration tests.
// This type uses a different name to avoid redeclaration errors when -tags test is used.
type minimalHook struct{}

func (h *minimalHook) Levels() []Level     { return AllLevels }
func (h *minimalHook) Fire(e *Entry) error { return nil }
