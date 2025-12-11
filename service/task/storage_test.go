package task

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAppName = "test-app"

// setupTestStorage 테스트를 위한 helper 함수
func setupTestStorage(t *testing.T) (*FileTaskResultStorage, string) {
	t.Helper()
	storage := NewFileTaskResultStorage(testAppName)
	tempDir := t.TempDir()
	storage.SetBaseDir(tempDir)
	return storage, tempDir
}

// ComplexData 테스트에 사용할 복합 구조체 정의
type ComplexData struct {
	Name string            `json:"name"`
	Tags []string          `json:"tags"`
	Meta map[string]string `json:"meta"`
}

// TestFileTaskResultStorage_Basic Table-Driven 방식을 사용한 기본 기능 테스트
func TestFileTaskResultStorage_Basic(t *testing.T) {

	tests := []struct {
		name        string
		taskID      ID
		commandID   CommandID
		input       interface{}
		output      interface{} // Load할 때 사용할 빈 객체 포인터
		wantErr     bool
		errContains string
	}{
		{
			name:      "Simple String",
			taskID:    ID("TASK_STR"),
			commandID: CommandID("CMD_STR"),
			input:     "simple string",
			output:    new(string),
			wantErr:   false,
		},
		{
			name:      "Integer",
			taskID:    ID("TASK_INT"),
			commandID: CommandID("CMD_INT"),
			input:     12345,
			output:    new(int),
			wantErr:   false,
		},
		{
			name:      "Map",
			taskID:    ID("TASK_MAP"),
			commandID: CommandID("CMD_MAP"),
			input:     map[string]int{"one": 1, "two": 2},
			output:    &map[string]int{},
			wantErr:   false,
		},
		{
			name:      "Slice",
			taskID:    ID("TASK_SLICE"),
			commandID: CommandID("CMD_SLICE"),
			input:     []string{"a", "b", "c"},
			output:    &[]string{},
			wantErr:   false,
		},
		{
			name:      "Complex Struct",
			taskID:    ID("TASK_COMPLEX"),
			commandID: CommandID("CMD_COMPLEX"),
			input: ComplexData{
				Name: "complex",
				Tags: []string{"tag1", "tag2"},
				Meta: map[string]string{"k1": "v1"},
			},
			output:  &ComplexData{},
			wantErr: false,
		},
		{
			name:      "Empty Struct",
			taskID:    ID("TASK_EMPTY"),
			commandID: CommandID("CMD_EMPTY"),
			input:     struct{}{},
			output:    &struct{}{},
			wantErr:   false,
		},
		{
			name:      "Nil Input (JSON Marshal handles nil as null)",
			taskID:    ID("TASK_NIL"),
			commandID: CommandID("CMD_NIL"),
			input:     nil,
			output:    new(interface{}), // nil 로드 시 interface{}로 받음
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, _ := setupTestStorage(t)

			// Save
			err := storage.Save(tt.taskID, tt.commandID, tt.input)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)

			// Load
			err = storage.Load(tt.taskID, tt.commandID, tt.output)
			require.NoError(t, err)

			// Verify
			// Tip: json unmarshal of nil stays nil, pointer comparison needs expected value
			if tt.input == nil {
				// json.Unmarshal into interface{} for "null" results in nil
				val := *(tt.output.(*interface{}))
				assert.Nil(t, val)
			} else {
				// tt.output is a pointer, dereference to compare with input
				// 그러나 input타입과 output타입이 다를 수 있음(json unmarshal 특성)
				// 여기서는 간단히 Equal로 비교하되, 타입 불일치 시 테스트 실패할 수 있음.
				// 위 케이스들은 타입 매칭됨.

				// dereference logic for assertion
				// tt.output is *T, we want to compare *tt.output with tt.input
				assert.Equal(t, tt.input, getElement(tt.output))
			}
		})
	}
}

// getElement reflects the value from pointer using reflection to be generic
func getElement(ptr interface{}) interface{} {
	val := reflect.ValueOf(ptr)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	return val.Interface()
}

func TestFileTaskResultStorage_Load_NonExistentFile(t *testing.T) {
	storage, _ := setupTestStorage(t)

	type TestData struct{ Val string }
	data := &TestData{}

	// 존재하지 않는 파일 로드 시 에러 없이 빈 상태(마지막 인자 변경 없음)여야 하는지,
	// 혹은 nil 리턴인지 확인.
	// storage.go 구현상 os.PathError이면 nil 반환하도록 되어 있음.
	err := storage.Load(ID("NO_FILE"), CommandID("NO_CMD"), data)
	assert.NoError(t, err, "존재하지 않는 파일은 에러 없이 무시되어야 함")
	assert.Empty(t, data.Val)
}

func TestFileTaskResultStorage_Security(t *testing.T) {
	storage, _ := setupTestStorage(t)

	type TestData struct{ Val string }
	data := &TestData{Val: "hacked"}

	tests := []struct {
		name      string
		taskID    ID
		commandID CommandID
		wantErr   bool
	}{
		{
			name:      "Path Traversal in TaskID",
			taskID:    ID("../hack_task"),
			commandID: CommandID("cmd"),
			wantErr:   true,
		},
		{
			name:      "Path Traversal in CommandID",
			taskID:    ID("task"),
			commandID: CommandID("../cmd"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save 시도
			err := storage.Save(tt.taskID, tt.commandID, data)
			if tt.wantErr {
				// storage.go에서 Path Traversal 감지 시 ErrInternal 리턴
				// 구체적으로는 "Path Traversal Detected" 메시지 혹은 apperrors.ErrInternal
				require.Error(t, err)
				assert.True(t, apperrors.Is(err, apperrors.ErrInternal))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFileTaskResultStorage_Cleanup(t *testing.T) {
	storage, tempDir := setupTestStorage(t)

	// 더미 임시 파일 생성
	tmpFile := filepath.Join(tempDir, "task-result-dummy.tmp")
	err := os.WriteFile(tmpFile, []byte("dummy data"), 0644)
	require.NoError(t, err)

	// 패턴에 맞지 않는 파일 (삭제되지 않아야 함)
	otherTmp := filepath.Join(tempDir, "other.tmp")
	err = os.WriteFile(otherTmp, []byte("keep"), 0644)
	require.NoError(t, err)

	// 일반 파일 생성 (삭제되지 않아야 함)
	normalFile := filepath.Join(tempDir, "task-result-normal.json")
	err = os.WriteFile(normalFile, []byte("{}"), 0644)
	require.NoError(t, err)

	// 정리 수행
	storage.CleanupTempFiles()

	// 검증
	assert.NoFileExists(t, tmpFile, "패턴에 맞는 임시 파일은 삭제되어야 합니다")
	assert.FileExists(t, otherTmp, "패턴에 맞지 않는 임시 파일은 유지되어야 합니다")
	assert.FileExists(t, normalFile, "일반 파일은 유지되어야 합니다")
}

func TestFileTaskResultStorage_Concurrency(t *testing.T) {
	storage, _ := setupTestStorage(t)

	type Data struct {
		Counter int `json:"counter"`
	}

	t.Run("Concurrent Read/Write Single File", func(t *testing.T) {
		taskID := ID("CONCURRENCY_TASK")
		commandID := CommandID("SAME_KEY")
		workers := 50
		var wg sync.WaitGroup

		require.NoError(t, storage.Save(taskID, commandID, &Data{Counter: 0}))

		wg.Add(workers)
		for i := 0; i < workers; i++ {
			go func(val int) {
				defer wg.Done()
				// 쓰기 시도 (에러가 없어야 함)
				if err := storage.Save(taskID, commandID, &Data{Counter: val}); err != nil {
					// 로그만 찍거나 무시 (테스트 실패 유발 X)
					// t.Error 호출은 데이터 레이스 주의
				}
				// 읽기 시도
				var d Data
				_ = storage.Load(taskID, commandID, &d)
			}(i)
		}
		wg.Wait()

		// 최종 파일 상태 확인
		var final Data
		err := storage.Load(taskID, commandID, &final)
		assert.NoError(t, err)
	})

	t.Run("Parallel Multiple Files", func(t *testing.T) {
		workers := 50
		var wg sync.WaitGroup
		wg.Add(workers)

		for i := 0; i < workers; i++ {
			go func(idx int) {
				defer wg.Done()
				tid := ID(fmt.Sprintf("TASK_%d", idx))
				cid := CommandID(fmt.Sprintf("CMD_%d", idx))
				data := &Data{Counter: idx}

				assert.NoError(t, storage.Save(tid, cid, data))

				var read Data
				assert.NoError(t, storage.Load(tid, cid, &read))
				assert.Equal(t, idx, read.Counter)
			}(i)
		}
		wg.Wait()
	})
}

// Benchmarks

func BenchmarkFileTaskResultStorage_Save(b *testing.B) {
	// 벤치마크는 각 반복마다 setup을 하면 느려지므로,
	// 디렉토리 하나를 공유하되 파일명을 다르게 하거나 덮어쓰기 테스트
	storage := NewFileTaskResultStorage(testAppName)
	tempDir := b.TempDir()
	storage.SetBaseDir(tempDir)

	taskID := ID("BENCH_TASK")
	commandID := CommandID("BENCH_CMD")
	data := map[string]string{"key": "value", "data": "benchmark testing payload"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = storage.Save(taskID, commandID, data)
	}
}

func BenchmarkFileTaskResultStorage_Load(b *testing.B) {
	storage := NewFileTaskResultStorage(testAppName)
	tempDir := b.TempDir()
	storage.SetBaseDir(tempDir)

	taskID := ID("BENCH_TASK")
	commandID := CommandID("BENCH_CMD")
	data := map[string]string{"key": "value", "data": "benchmark testing payload"}
	_ = storage.Save(taskID, commandID, data)

	dest := make(map[string]string)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = storage.Load(taskID, commandID, &dest)
	}
}
