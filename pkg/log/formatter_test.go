//go:build test

package log

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// TestSilentFormatter_Interface verifies that silentFormatter implements logrus.Formatter.
func TestSilentFormatter_Interface(t *testing.T) {
	t.Parallel()

	var _ logrus.Formatter = (*silentFormatter)(nil)
	var _ logrus.Formatter = &silentFormatter{}

	assert.Implements(t, (*logrus.Formatter)(nil), &silentFormatter{})
}

// TestSilentFormatter_Format verifies that Format always returns nil, nil.
func TestSilentFormatter_Format(t *testing.T) {
	t.Parallel()

	f := &silentFormatter{}

	// Case 1: Nil Entry
	data, err := f.Format(nil)
	assert.Nil(t, data)
	assert.Nil(t, err)

	// Case 2: Populated Entry
	entry := &logrus.Entry{
		Message: "test message",
		Level:   logrus.InfoLevel,
		Data:    logrus.Fields{"key": "value"},
	}
	data, err = f.Format(entry)
	assert.Nil(t, data)
	assert.Nil(t, err)
}

// BenchmarkSilentFormatter proves that silentFormatter has near-zero overhead.
// Run with: go test -bench=. -benchmem
func BenchmarkSilentFormatter(b *testing.B) {
	f := &silentFormatter{}
	entry := &logrus.Entry{
		Message: "benchmark",
		Data:    logrus.Fields{"key": "value"},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = f.Format(entry)
	}
}
