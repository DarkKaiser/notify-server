package lotto

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLimitWriter_Write_WithinLimit(t *testing.T) {
	var buf bytes.Buffer
	limit := 100
	lw := &limitWriter{w: &buf, limit: limit}

	data := []byte("hello world")
	n, err := lw.Write(data)

	assert.NoError(t, err)
	assert.Equal(t, len(data), n)
	assert.Equal(t, "hello world", buf.String())
	assert.Equal(t, len(data), lw.written)
}

func TestLimitWriter_Write_ExactLimit(t *testing.T) {
	var buf bytes.Buffer
	limit := 5
	lw := &limitWriter{w: &buf, limit: limit}

	data := []byte("12345")
	n, err := lw.Write(data)

	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "12345", buf.String())
	assert.Equal(t, 5, lw.written)
}

func TestLimitWriter_Write_ExceedsLimit_SingleWrite(t *testing.T) {
	var buf bytes.Buffer
	limit := 5
	lw := &limitWriter{w: &buf, limit: limit}

	// 5바이트 제한인데 10바이트 씀
	data := []byte("1234567890")
	n, err := lw.Write(data)

	assert.NoError(t, err)
	assert.Equal(t, 10, n, "실제 쓰인 양과 무관하게 입력받은 길이를 반환해야 함 (에러 방지)")
	assert.Equal(t, "12345\n... (truncated)", buf.String(), "버퍼에는 제한된 양만 있어야 함")
	assert.Equal(t, 5, lw.written, "실제 기록된 양은 제한값이어야 함")
}

func TestLimitWriter_Write_ExceedsLimit_MultipleWrites(t *testing.T) {
	var buf bytes.Buffer
	limit := 10
	lw := &limitWriter{w: &buf, limit: limit}

	// 1. 5바이트 씀 (여유 5)
	n, err := lw.Write([]byte("12345"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, 5, lw.written)

	// 2. 8바이트 씀 (여유 5인데 8 들어옴 -> 5만 쓰고 3 버림)
	n, err = lw.Write([]byte("67890ABC"))
	assert.NoError(t, err)
	assert.Equal(t, 8, n)

	assert.Equal(t, "1234567890\n... (truncated)", buf.String()) // 정확히 10바이트만 저장됨 + Truncated Msg
	assert.Equal(t, 10, lw.written)

	// 3. 이미 꽉 참 (여유 0 -> 전부 버림)
	n, err = lw.Write([]byte("DEF"))
	assert.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, "1234567890\n... (truncated)", buf.String()) // 변화 없음
}

func TestLimitWriter_LargeData(t *testing.T) {
	// 1MB 데이터 생성
	largeData := make([]byte, 1024*1024)
	rand.Read(largeData)

	var buf bytes.Buffer
	limit := 1024 // 1kb 제한
	lw := &limitWriter{w: &buf, limit: limit}

	n, err := lw.Write(largeData)
	assert.NoError(t, err)
	assert.Equal(t, len(largeData), n)

	truncatedMsg := []byte("\n... (truncated)")
	assert.Equal(t, limit+len(truncatedMsg), buf.Len())

	expected := append(largeData[:limit], truncatedMsg...)
	assert.Equal(t, expected, buf.Bytes())
}

func TestDefaultCommandExecutor_StartCommand_LimitOutput(t *testing.T) {
	// Helper Process 패턴을 사용하여 실제 프로세스 실행 시 제한이 동작하는지 검증
	limit := 10 // 10바이트 제한

	// 명시적으로 환경변수를 주입하여 자식 프로세스가 Helper로 동작하도록 함
	executor := &defaultCommandExecutor{
		limit: limit,
		env:   []string{"TEST_HELPER_PROCESS=1"},
	}
	ctx := context.Background()

	// 자기 자신을 다시 실행하되, "-test.run=TestHelperProcess"를 통해 TestHelperProcess 함수만 실행되도록 함
	proc, err := executor.StartCommand(ctx, os.Args[0], "-test.run=TestHelperProcess", "-test.v")
	assert.NoError(t, err)

	err = proc.Wait()
	assert.NoError(t, err)

	output := proc.Stdout()

	// 출력 길이 제한 확인
	// 주의: go test 출력(PASS 등)이 섞일 수 있으므로 정확한 매칭보다는
	// 1. 길이가 제한값(10)인지 확인 (limitWriter가 정상 작동했는지)
	// 2. 내용에 우리가 출력한 데이터가 일부 포함되어 있는지 확인
	truncatedMsg := "\n... (truncated)"
	assert.Equal(t, limit+len(truncatedMsg), len(output), "출력은 제한 크기 + Truncated Msg여야 함")

	// "1234567890ABCDE" 중 앞부분이 포함되어야 함.
	// 하지만 limitWriter는 앞에서부터 자르므로 "1234567890"이 되어야 함.
	// 만약 "PASS" 같은게 먼저 나오면 그게 담길 수도 있음.
	// 테스트 신뢰성을 위해, TestHelperProcess가 최대한 먼저 출력하도록 유도했지만 보장할 순 없음.
	// 적어도 비어있지는 않아야 함.
	assert.NotEmpty(t, output)
}

// TestHelperProcess는 테스트 목적의 자식 프로세스로 실행됩니다.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("TEST_HELPER_PROCESS") != "1" {
		return
	}
	// "go test"의 다른 출력이 나오기 전에 최대한 빨리 출력하고 종료
	fmt.Print("1234567890ABCDE")
	os.Exit(0)
}
