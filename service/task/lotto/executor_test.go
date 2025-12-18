package lotto

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"os"
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
		inputs        [][]byte // 여러 번 Write 할 수 있음
		expectedN     []int    // 각 Write 호출의 반환값
		expectedTotal int      // 최종 written 카운트
		expectedBuf   string   // 버퍼에 담긴 내용
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
			expectedN:     []int{10}, // 반환값은 입력 길이 (에러 방지)
			expectedTotal: 5,         // 실제 기록은 제한값
			expectedBuf:   "12345\n... (truncated)",
		},
		{
			name:          "Exceed Limit Multiple Writes",
			limit:         10,
			inputs:        [][]byte{[]byte("12345"), []byte("67890ABC")},
			expectedN:     []int{5, 8}, // 5바이트 씀, 8바이트 시도(3바이트 초과)
			expectedTotal: 10,
			expectedBuf:   "1234567890\n... (truncated)",
		},
		{
			name:          "Already Full",
			limit:         5,
			inputs:        [][]byte{[]byte("12345"), []byte("ABC")},
			expectedN:     []int{5, 3},
			expectedTotal: 5,
			expectedBuf:   "12345\n... (truncated)", // 이제 정확히 꽉 찼어도 이후 쓰기 시도가 있으면 Truncated Msg가 써짐
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

			// 결과 검증
			assert.Equal(t, tt.expectedTotal, lw.written, "internal written count mismatch")
			assert.Equal(t, tt.expectedBuf, buf.String())
		})
	}
}

func TestLimitWriter_LargeData(t *testing.T) {
	// 1MB 데이터 생성
	largeData := make([]byte, 1024*1024)
	n, _ := rand.Read(largeData)
	require.Equal(t, len(largeData), n)

	var buf bytes.Buffer
	limit := 1024 // 1kb 제한
	lw := &limitWriter{w: &buf, limit: limit}

	written, err := lw.Write(largeData)
	assert.NoError(t, err)
	assert.Equal(t, len(largeData), written)

	truncatedMsg := []byte("\n... (truncated)")
	assert.Equal(t, limit+len(truncatedMsg), buf.Len())

	expected := append(largeData[:limit], truncatedMsg...)
	assert.Equal(t, expected, buf.Bytes())
}

// --- DefaultCommandExecutor Integration Tests ---

// TestHelperProcess는 테스트 목적의 자식 프로세스로 실행됩니다.
// -test.run=TestHelperProcess 플래그와 함께 실행되어야 합니다.
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
	case "sleep":
		time.Sleep(10 * time.Second) // 충분히 길게 대기
	case "stderr":
		fmt.Fprint(os.Stderr, os.Getenv("HELPER_PAYLOAD"))
	case "large-output":
		fmt.Print(strings.Repeat("A", 1024*1024)) // 1MB 출력
	}
}

// makeHelperCommand는 자기 자신을 자식 프로세스로 실행하는 명령어를 생성합니다.
func makeHelperCommand(ctx context.Context, executor commandExecutor, mode string, env map[string]string) (commandProcess, error) {
	// 현재 실행 중인 테스트 바이너리 경로
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}

	// 기본 환경변수에 헬퍼 모드 추가
	defaultEnv := []string{"GO_TEST_HELPER_PROCESS=1", "HELPER_MODE=" + mode}
	for k, v := range env {
		defaultEnv = append(defaultEnv, fmt.Sprintf("%s=%s", k, v))
	}

	// executor가 defaultCommandExecutor인 경우 환경변수 설정
	if dexec, ok := executor.(*defaultCommandExecutor); ok {
		dexec.env = defaultEnv
	} else {
		return nil, fmt.Errorf("executor type mismatch")
	}

	args := []string{"-test.run=TestHelperProcess", "-test.v=false"} // -test.v=false로 노이즈 최소화
	return executor.StartCommand(ctx, exe, args...)
}

func TestDefaultCommandExecutor_Env(t *testing.T) {
	executor := &defaultCommandExecutor{}
	ctx := context.Background()

	// 헬퍼가 TARGET_ENV_KEY 값을 출력하도록 요청
	proc, err := makeHelperCommand(ctx, executor, "env", map[string]string{
		"TARGET_ENV_KEY": "SUPER_SECRET_VALUE",
	})
	require.NoError(t, err)

	err = proc.Wait()
	assert.NoError(t, err)

	output := proc.Stdout()
	// go test 출력에 섞일 수 있으므로 Contains로 확인
	assert.Contains(t, output, "SUPER_SECRET_VALUE")
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
	// 100ms 타임아웃
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// 10초 동안 자는 프로세스 실행
	proc, err := makeHelperCommand(ctx, executor, "sleep", nil)
	require.NoError(t, err)

	start := time.Now()
	err = proc.Wait()
	duration := time.Since(start)

	// Context 취소로 인해 에러가 반환되어야 함 (주로 signal: killed 또는 exit status 등)
	assert.Error(t, err)

	// 10초가 아니라 타임아웃(100ms) + 알파 내에 종료되어야 함
	assert.Less(t, duration, 2*time.Second, "프로세스가 타임아웃에 의해 강제 종료되지 않았습니다")
}

func TestDefaultCommandExecutor_LimitOutput(t *testing.T) {
	limit := 1024 // 1KB
	executor := &defaultCommandExecutor{
		limit: limit,
	}
	ctx := context.Background()

	// 1MB 출력 요청
	proc, err := makeHelperCommand(ctx, executor, "large-output", nil)
	require.NoError(t, err)

	err = proc.Wait()
	assert.NoError(t, err)

	output := proc.Stdout()
	truncatedMark := "\n... (truncated)"
	assert.Equal(t, limit+len(truncatedMark), len(output))
	assert.Contains(t, output, truncatedMark)
}
