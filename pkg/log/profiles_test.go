package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewProductionOptions(t *testing.T) {
	opts := NewProductionOptions("prod-app")
	assert.Equal(t, "prod-app", opts.Name)
	assert.Equal(t, InfoLevel, opts.Level)
	assert.Equal(t, 30, opts.MaxAge)
	assert.Equal(t, 10, opts.MaxSizeMB)
	assert.Equal(t, 20, opts.MaxBackups)
	assert.True(t, opts.EnableCriticalLog)
	assert.True(t, opts.EnableVerboseLog)
	assert.False(t, opts.EnableConsoleLog)
	assert.True(t, opts.ReportCaller)
}

func TestNewDevelopmentOptions(t *testing.T) {
	opts := NewDevelopmentOptions("dev-app")
	assert.Equal(t, "dev-app", opts.Name)
	assert.Equal(t, TraceLevel, opts.Level)
	assert.Equal(t, 1, opts.MaxAge)
	assert.Equal(t, 50, opts.MaxSizeMB)
	assert.Equal(t, 5, opts.MaxBackups)
	assert.False(t, opts.EnableCriticalLog)
	assert.False(t, opts.EnableVerboseLog)
	assert.True(t, opts.EnableConsoleLog)
	assert.True(t, opts.ReportCaller)
}
