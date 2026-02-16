package lotto

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- LimitWriter Unit Tests ---

func TestLimitWriter_Write(t *testing.T) {
	tests := []struct {
		name          string
		limit         int
		inputs        [][]byte
		expectedN     []int
		expectedTotal int
		expectedBuf   string
	}{
		{
			name:          "Within Limit",
			limit:         100,
			inputs:        [][]byte{[]byte("hello world")},
			expectedN:     []int{11},
			expectedTotal: 11,
			expectedBuf:   "hello world",
		},
		{
			name:          "Exact Limit",
			limit:         5,
			inputs:        [][]byte{[]byte("12345")},
			expectedN:     []int{5},
			expectedTotal: 5,
			expectedBuf:   "12345",
		},
		{
			name:          "Exceed Limit Single Write",
			limit:         5,
			inputs:        [][]byte{[]byte("1234567890")},
			expectedN:     []int{10},
			expectedTotal: 5,
			expectedBuf:   "12345\n...(생략됨)",
		},
		{
			name:          "Exceed Limit Multiple Writes",
			limit:         10,
			inputs:        [][]byte{[]byte("12345"), []byte("67890ABC")},
			expectedN:     []int{5, 8},
			expectedTotal: 10,
			expectedBuf:   "1234567890\n...(생략됨)",
		},
		{
			name:          "Already Full",
			limit:         5,
			inputs:        [][]byte{[]byte("12345"), []byte("ABC")},
			expectedN:     []int{5, 3},
			expectedTotal: 5,
			expectedBuf:   "12345\n...(생략됨)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			lw := &limitWriter{w: &buf, limit: tt.limit}

			for i, input := range tt.inputs {
				n, err := lw.Write(input)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedN[i], n)
			}

			assert.Equal(t, tt.expectedTotal, lw.written, "internal written count mismatch")
			assert.Equal(t, tt.expectedBuf, buf.String())
		})
	}

	t.Run("Error Propagation", func(t *testing.T) {
		errWriter := &errorWriter{err: fmt.Errorf("write error")}
		lw := &limitWriter{w: errWriter, limit: 10}

		// 1. Normal write error propagation
		n, err := lw.Write([]byte("123"))
		assert.Error(t, err)
		assert.Equal(t, 2, n)

		// 2. Truncation message write error
		lw.written = 10
		n, err = lw.Write([]byte("exceed"))
		assert.Error(t, err)
		assert.Equal(t, 0, n)
	})
}

type errorWriter struct {
	err error
}

func (e *errorWriter) Write(p []byte) (n int, err error) {
	if len(p) > 2 && string(p[:2]) == "12" {
		return 2, e.err
	}
	return 0, e.err
}

func TestLimitWriter_LargeData(t *testing.T) {
	largeData := make([]byte, 1024*1024)
	n, _ := rand.Read(largeData)
	require.Equal(t, len(largeData), n)

	var buf bytes.Buffer
	limit := 1024
	lw := &limitWriter{w: &buf, limit: limit}

	written, err := lw.Write(largeData)
	assert.NoError(t, err)
	assert.Equal(t, len(largeData), written)

	truncatedMsg := []byte("\n...(생략됨)")
	assert.Equal(t, limit+len(truncatedMsg), buf.Len())

	expected := append(largeData[:limit], truncatedMsg...)
	assert.Equal(t, expected, buf.Bytes())
}

// --- DefaultCommandExecutor Integration Tests ---

// TestHelperProcess acts as a child process for tests.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_TEST_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	mode := os.Getenv("HELPER_MODE")
	switch mode {
	case "echo":
		fmt.Print(os.Getenv("HELPER_PAYLOAD"))
	case "env":
		fmt.Print(os.Getenv("TARGET_ENV_KEY"))
	case "pwd":
		wd, _ := os.Getwd()
		fmt.Print(wd)
	case "sleep":
		time.Sleep(10 * time.Second)
	case "stderr":
		fmt.Fprint(os.Stderr, os.Getenv("HELPER_PAYLOAD"))
	case "large-output":
		fmt.Print(strings.Repeat("A", 1024*1024))
	case "crash":
		os.Exit(1)
	}
}

func makeHelperCommand(ctx context.Context, executor commandExecutor, mode string, env map[string]string) (commandProcess, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}

	defaultEnv := []string{"GO_TEST_HELPER_PROCESS=1", "HELPER_MODE=" + mode}
	for k, v := range env {
		defaultEnv = append(defaultEnv, fmt.Sprintf("%s=%s", k, v))
	}

	if dexec, ok := executor.(*defaultCommandExecutor); ok {
		newEnv := append([]string{}, defaultEnv...)
		if len(dexec.env) > 0 {
			newEnv = append(newEnv, dexec.env...)
		}
		dexec.env = newEnv
	}

	args := []string{"-test.run=TestHelperProcess", "-test.v=false"}
	return executor.Start(ctx, exe, args...)
}

func TestDefaultCommandExecutor_Start_Error(t *testing.T) {
	executor := &defaultCommandExecutor{}
	ctx := context.Background()

	_, err := executor.Start(ctx, "non_existent_command_12345")
	assert.Error(t, err)
}

func TestDefaultCommandExecutor_Dir(t *testing.T) {
	tempDir := t.TempDir()

	// On Windows, tempDir might be something like "C:\Users\ADMINI~1\AppData\Local\Temp\...",
	// but Getwd() might return the long path version.
	// So we evaluate symlinks to get the "real" path for comparison.
	realTempDir, err := filepath.EvalSymlinks(tempDir)
	require.NoError(t, err)

	executor := &defaultCommandExecutor{
		dir: tempDir,
	}
	ctx := context.Background()

	proc, err := makeHelperCommand(ctx, executor, "pwd", nil)
	require.NoError(t, err)

	err = proc.Wait()
	assert.NoError(t, err)

	output := strings.TrimSpace(proc.Stdout())

	// Resolve output path as well just in case
	realOutput, err := filepath.EvalSymlinks(output)
	if err == nil {
		output = realOutput
	}

	// Case-insensitive comparison for Windows robustness
	assert.Equal(t, strings.ToLower(realTempDir), strings.ToLower(output))
}

func TestDefaultCommandExecutor_Env(t *testing.T) {
	executor := &defaultCommandExecutor{}
	ctx := context.Background()

	proc, err := makeHelperCommand(ctx, executor, "env", map[string]string{
		"TARGET_ENV_KEY": "SUPER_SECRET_VALUE",
	})
	require.NoError(t, err)

	err = proc.Wait()
	assert.NoError(t, err)
	assert.Contains(t, proc.Stdout(), "SUPER_SECRET_VALUE")
}

func TestDefaultCommandExecutor_CaptureStderr(t *testing.T) {
	executor := &defaultCommandExecutor{}
	ctx := context.Background()

	payload := "This is error message"
	proc, err := makeHelperCommand(ctx, executor, "stderr", map[string]string{
		"HELPER_PAYLOAD": payload,
	})
	require.NoError(t, err)

	err = proc.Wait()
	assert.NoError(t, err)
	assert.Contains(t, proc.Stderr(), payload)
	assert.Empty(t, proc.Stdout())
}

func TestDefaultCommandExecutor_ContextCancel(t *testing.T) {
	executor := &defaultCommandExecutor{}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	proc, err := makeHelperCommand(ctx, executor, "sleep", nil)
	require.NoError(t, err)

	start := time.Now()
	err = proc.Wait()
	duration := time.Since(start)

	assert.Error(t, err)
	assert.Less(t, duration, 2*time.Second)
}

func TestDefaultCommandExecutor_LimitOutput(t *testing.T) {
	limit := 1024
	executor := &defaultCommandExecutor{
		limit: limit,
	}
	ctx := context.Background()

	proc, err := makeHelperCommand(ctx, executor, "large-output", nil)
	require.NoError(t, err)

	err = proc.Wait()
	assert.NoError(t, err)

	output := proc.Stdout()
	truncatedMark := "\n...(생략됨)"
	assert.Equal(t, limit+len(truncatedMark), len(output))
	assert.Contains(t, output, truncatedMark)
}

func TestDefaultCommandExecutor_ExitError(t *testing.T) {
	executor := &defaultCommandExecutor{}
	ctx := context.Background()

	proc, err := makeHelperCommand(ctx, executor, "crash", nil)
	require.NoError(t, err)

	err = proc.Wait()
	assert.Error(t, err) // Should error on non-zero exit code
}

func TestDefaultCommandProcess_Kill(t *testing.T) {
	executor := &defaultCommandExecutor{}
	ctx := context.Background()

	proc, err := makeHelperCommand(ctx, executor, "sleep", nil)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
	err = proc.Kill()
	assert.NoError(t, err)

	err = proc.Wait()
	assert.Error(t, err)
}
