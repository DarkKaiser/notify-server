//go:build test

package log

import (
	"bytes"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mocks & Helpers
// =============================================================================

// failWriter는 의도적으로 에러를 발생시키는 Writer 모의 객체입니다.
type failWriter struct {
	err error
}

func (w *failWriter) Write(p []byte) (n int, err error) {
	return 0, w.err
}

// errorFormatter는 Format 호출 시 항상 에러를 반환하는 Formatter입니다.
type errorFormatter struct{}

func (f *errorFormatter) Format(entry *Entry) ([]byte, error) {
	return nil, errors.New("formatting failed")
}

// safeBuffer는 동시성 테스트를 위해 Mutex로 보호되는 Buffer입니다.
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

func (b *safeBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

// newTestHook은 테스트를 위한 격리된 hook 인스턴스를 생성합니다.
func newTestHook() (*hook, *bytes.Buffer, *bytes.Buffer, *bytes.Buffer, *bytes.Buffer) {
	mainBuf := &bytes.Buffer{}
	critBuf := &bytes.Buffer{}
	verbBuf := &bytes.Buffer{}
	consBuf := &bytes.Buffer{}

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
	assert.Equal(t, AllLevels, h.Levels(), "Hook은 모든 로그 레벨을 처리해야 합니다.")
}

func TestHook_Fire_Routing(t *testing.T) {
	// Table-Driven Tests 설계를 통해 모든 로그 레벨에 대한 라우팅 정책을 검증합니다.
	tests := []struct {
		name         string
		level        Level
		expectMain   bool
		expectCrit   bool
		expectVerb   bool
		expectCons   bool // Console은 항상 출력되어야 함
		setupHookOpt func(*hook)
	}{
		// 1. Critical Level Group
		{"Panic Level", PanicLevel, true, true, false, true, nil},
		{"Fatal Level", FatalLevel, true, true, false, true, nil},
		{"Error Level", ErrorLevel, true, true, false, true, nil},

		// 2. Main Level Group
		{"Warn Level", WarnLevel, true, false, false, true, nil},
		{"Info Level", InfoLevel, true, false, false, true, nil},

		// 3. Verbose Level Group (Noise Isolation Check)
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
		t.Run(tc.name, func(t *testing.T) {
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

			// Helper to check buffer content
			check := func(buf *bytes.Buffer, expected bool, name string) {
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
	t.Run("Critical Writer 실패 시에도 Main Writer는 기록되어야 함 (Soft Failure)", func(t *testing.T) {
		expectedErr := errors.New("disk full")
		h, main, _, _, _ := newTestHook()
		h.criticalWriter = &failWriter{err: expectedErr}

		entry := &Entry{Level: ErrorLevel, Message: "critical failure"}

		err := h.Fire(entry)

		// 에러는 반환되어야 하지만...
		assert.ErrorIs(t, err, expectedErr)
		// Main 로깅은 성공해야 함
		assert.Contains(t, main.String(), "critical failure", "Critical 실패가 Main 기록을 방해해선 안 됨")
	})

	t.Run("Verbose Writer 실패 시 에러 반환 (Main 오염 방지)", func(t *testing.T) {
		expectedErr := errors.New("disk full")
		h, main, _, _, _ := newTestHook()
		h.verboseWriter = &failWriter{err: expectedErr}

		entry := &Entry{Level: DebugLevel, Message: "verbose failure"}

		err := h.Fire(entry)

		assert.ErrorIs(t, err, expectedErr)
		assert.Empty(t, main.String(), "Verbose 로깅 실패 시 Main으로 넘어가지 않고 종료되어야 함")
	})

	t.Run("Console Writer 실패는 완전히 무시됨 (Fail-Safe)", func(t *testing.T) {
		h, main, _, _, _ := newTestHook()
		h.consoleWriter = &failWriter{err: errors.New("stdout closed")}

		entry := &Entry{Level: InfoLevel, Message: "console failure"}

		err := h.Fire(entry)

		assert.NoError(t, err, "Console Writer 에러는 전파되지 않아야 함")
		assert.Contains(t, main.String(), "console failure", "Console 실패가 Main 기록에 영향을 주면 안 됨")
	})

	t.Run("Formatter 실패 시 즉시 에러 반환", func(t *testing.T) {
		h, _, _, _, _ := newTestHook()
		h.formatter = &errorFormatter{}

		err := h.Fire(&Entry{Level: InfoLevel, Message: "format fail"})

		assert.ErrorContains(t, err, "formatting failed")
	})
}

func TestHook_Close_Lifecycle(t *testing.T) {
	h, main, _, _, _ := newTestHook()

	// 1. 정상 동작 확인
	require.NoError(t, h.Fire(&Entry{Level: InfoLevel, Message: "alive"}))
	require.Contains(t, main.String(), "alive")
	main.Reset()

	// 2. Close 호출
	require.NoError(t, h.Close())

	// 3. Close 후 동작 확인 (No-op)
	require.NoError(t, h.Fire(&Entry{Level: InfoLevel, Message: "dead"}))
	assert.Empty(t, main.String(), "Close된 Hook은 로그를 기록하지 않아야 함")
}

// =============================================================================
// Concurrency Tests (Go Race Detector 필수)
// =============================================================================

func TestHook_Concurrency_Stress(t *testing.T) {
	// 동시성 환경에서 Data Race 및 데드락 발생 여부를 검증합니다.

	mainBuf := &safeBuffer{}
	h := &hook{
		mainWriter:     mainBuf,
		criticalWriter: &safeBuffer{},
		verboseWriter:  &safeBuffer{},
		consoleWriter:  &safeBuffer{}, // Safe mock
		formatter:      &TextFormatter{DisableTimestamp: true},
	}

	const (
		goroutines = 50
		logsPerG   = 100
	)

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			<-start // 동시 시작 대기

			for j := 0; j < logsPerG; j++ {
				// Random Level Logging
				level := Level(j % 6) // All levels mixed
				_ = h.Fire(&Entry{
					Level:   level,
					Message: "stress test",
				})
			}
		}(i)
	}

	close(start) // 출발 신호
	wg.Wait()

	// 검증: 최소한의 로그가 기록되었는지 (개별 Count는 버퍼 특성상 어려우나 Race 여부가 핵심)
	assert.Greater(t, mainBuf.Len(), 0, "Stress 테스트 후 Main 로그가 비어있으면 안 됨")
}

func TestHook_Concurrency_Close_Race(t *testing.T) {
	// Fire()와 Close()가 동시에 호출될 때 Panic이나 Race가 발생하지 않아야 합니다.
	h, _, _, _, _ := newTestHook()
	// Thread-safe writers needed equivalent to real file writers (Lumberjack is mutex protected)
	// But our bytes.Buffer is NOT thread safe. We need safe mocks for this specific test too?
	// Actually newTestHook uses plain bytes.Buffer which will Race if written concurrently.
	// For this test, we accept we need SafeWriters.

	safeW := &safeBuffer{}
	h.mainWriter = safeW
	h.criticalWriter = safeW
	h.verboseWriter = safeW
	h.consoleWriter = safeW

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	// 1. Writer Goroutine
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 1000; i++ {
			_ = h.Fire(&Entry{Level: InfoLevel, Message: "race"})
			time.Sleep(10 * time.Microsecond)
		}
	}()

	// 2. Closer Goroutine
	go func() {
		defer wg.Done()
		<-start
		time.Sleep(2 * time.Millisecond) // Let some logs pass
		_ = h.Close()
	}()

	close(start)
	wg.Wait()

	// Pass if no panic/race detected
}
