//go:build test

package log

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mocks & Helpers
// =============================================================================

// failWriter is a mock writer that always returns an error.
type failWriter struct {
	err error
}

func (w *failWriter) Write(p []byte) (n int, err error) {
	return 0, w.err
}

// errorFormatter is a mock formatter that always returns an error.
type errorFormatter struct{}

func (f *errorFormatter) Format(entry *Entry) ([]byte, error) {
	return nil, errors.New("formatting failed")
}

// safeBuffer is a thread-safe implementation of bytes.Buffer.
// Since hook.Fire holds a Read Lock (allowing concurrent Fire calls),
// the underlying writers must be thread-safe.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *safeBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}

func (b *safeBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

// newTestHook creates a hook with thread-safe buffers for testing.
func newTestHook() (*hook, *safeBuffer, *safeBuffer, *safeBuffer, *safeBuffer) {
	mainBuf := &safeBuffer{}
	critBuf := &safeBuffer{}
	verbBuf := &safeBuffer{}
	consBuf := &safeBuffer{}

	h := &hook{
		mainWriter:     mainBuf,
		criticalWriter: critBuf,
		verboseWriter:  verbBuf,
		consoleWriter:  consBuf,
		formatter:      &TextFormatter{DisableTimestamp: true},
	}
	return h, mainBuf, critBuf, verbBuf, consBuf
}

// =============================================================================
// Unit Tests
// =============================================================================

func TestHook_Levels(t *testing.T) {
	h := &hook{}
	assert.Equal(t, AllLevels, h.Levels(), "Hook should handle all log levels")
}

func TestHook_Fire_Routing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		level        Level
		expectMain   bool
		expectCrit   bool
		expectVerb   bool
		expectCons   bool
		setupHookOpt func(*hook)
	}{
		// 1. Critical Level Group
		{"Panic Level", PanicLevel, true, true, false, true, nil},
		{"Fatal Level", FatalLevel, true, true, false, true, nil},
		{"Error Level", ErrorLevel, true, true, false, true, nil},

		// 2. Main Level Group
		{"Warn Level", WarnLevel, true, false, false, true, nil},
		{"Info Level", InfoLevel, true, false, false, true, nil},

		// 3. Verbose Level Group
		{"Debug Level", DebugLevel, false, false, true, true, nil},
		{"Trace Level", TraceLevel, false, false, true, true, nil},

		// 4. Component Missing Scenarios
		{
			name:       "No Critical Writer (Error)",
			level:      ErrorLevel,
			expectMain: true, expectCrit: false, expectVerb: false, expectCons: true,
			setupHookOpt: func(h *hook) { h.criticalWriter = nil },
		},
		{
			name:       "No Verbose Writer (Debug)",
			level:      DebugLevel,
			expectMain: false, expectCrit: false, expectVerb: false, expectCons: true,
			setupHookOpt: func(h *hook) { h.verboseWriter = nil },
		},
	}

	for _, tc := range tests {
		tc := tc // capture for parallel
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, main, crit, verb, cons := newTestHook()
			if tc.setupHookOpt != nil {
				tc.setupHookOpt(h)
			}

			entry := &Entry{
				Level:   tc.level,
				Message: "test message",
			}

			err := h.Fire(entry)
			require.NoError(t, err)

			check := func(buf *safeBuffer, expected bool, name string) {
				if expected {
					assert.Contains(t, buf.String(), "test message", "%s should contain log", name)
				} else {
					assert.Empty(t, buf.String(), "%s should be empty", name)
				}
			}

			check(main, tc.expectMain, "MainWriter")
			check(crit, tc.expectCrit, "CriticalWriter")
			check(verb, tc.expectVerb, "VerboseWriter")
			check(cons, tc.expectCons, "ConsoleWriter")
		})
	}
}

func TestHook_Fire_FailSafe(t *testing.T) {
	t.Parallel()

	t.Run("Critical Writer 실패 시에도 Main Writer는 기록되어야 함", func(t *testing.T) {
		expectedErr := errors.New("disk full")
		h, main, _, _, _ := newTestHook()
		h.criticalWriter = &failWriter{err: expectedErr}

		entry := &Entry{Level: ErrorLevel, Message: "critical failure"}

		err := h.Fire(entry)

		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, main.String(), "critical failure", "Main logging should succeed despite critical failure")
	})

	t.Run("Verbose Writer 실패 시 에러 반환 및 Main 오염 방지", func(t *testing.T) {
		expectedErr := errors.New("disk full")
		h, main, _, _, _ := newTestHook()
		h.verboseWriter = &failWriter{err: expectedErr}

		entry := &Entry{Level: DebugLevel, Message: "verbose failure"}

		err := h.Fire(entry)

		assert.ErrorIs(t, err, expectedErr)
		assert.Empty(t, main.String(), "Main should not receive verbose logs even on failure")
	})

	t.Run("Console Writer 실패는 완전히 무시됨", func(t *testing.T) {
		h, main, _, _, _ := newTestHook()
		h.consoleWriter = &failWriter{err: errors.New("stdout closed")}

		entry := &Entry{Level: InfoLevel, Message: "console failure"}

		err := h.Fire(entry)

		assert.NoError(t, err, "Console error should be ignored")
		assert.Contains(t, main.String(), "console failure", "Main logging should still succeed")
	})

	t.Run("Formatter 실패 시 즉시 에러 반환", func(t *testing.T) {
		h, _, _, _, _ := newTestHook()
		h.formatter = &errorFormatter{}

		err := h.Fire(&Entry{Level: InfoLevel, Message: "format fail"})

		assert.ErrorContains(t, err, "formatting failed")
	})
}

func TestHook_Close_Lifecycle(t *testing.T) {
	t.Parallel()

	h, main, _, _, _ := newTestHook()

	// 1. Valid log
	require.NoError(t, h.Fire(&Entry{Level: InfoLevel, Message: "alive"}))
	require.Contains(t, main.String(), "alive")
	main.Reset()

	// 2. Close
	require.NoError(t, h.Close())

	// 3. Log after close (should be ignored)
	require.NoError(t, h.Fire(&Entry{Level: InfoLevel, Message: "dead"}))
	assert.Empty(t, main.String(), "Logs must be ignored after Close")
}

// =============================================================================
// Concurrency Tests
// =============================================================================

func TestHook_Concurrency_Stress(t *testing.T) {
	t.Parallel()

	h, mainBuf, _, _, _ := newTestHook()

	const (
		goroutines = 50
		logsPerG   = 100
	)

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < logsPerG; j++ {
				_ = h.Fire(&Entry{
					Level:   InfoLevel,
					Message: "stress test",
				})
			}
		}()
	}

	close(start)
	wg.Wait()

	// Use Lens check approximately. Exact number depends on format length.
	assert.Greater(t, mainBuf.Len(), 0)
}

func TestHook_Concurrency_Close_Race(t *testing.T) {
	t.Parallel()
	h, _, _, _, _ := newTestHook()

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	// Writer
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 1000; i++ {
			_ = h.Fire(&Entry{Level: InfoLevel, Message: "race"})
			time.Sleep(10 * time.Microsecond)
		}
	}()

	// Closer
	go func() {
		defer wg.Done()
		<-start
		time.Sleep(5 * time.Millisecond)
		_ = h.Close()
	}()

	close(start)
	wg.Wait()
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkHook_Fire(b *testing.B) {
	// Setup a hook with discard writers and silent formatter for minimal overhead measurement.
	h := &hook{
		mainWriter:     io.Discard,
		criticalWriter: io.Discard,
		verboseWriter:  io.Discard,
		consoleWriter:  io.Discard,
		formatter:      &silentFormatter{},
	}

	infoEntry := &Entry{Level: InfoLevel, Message: "benchmark"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = h.Fire(infoEntry)
		}
	})
}
