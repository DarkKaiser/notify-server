package task

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAppName = "test-app"

// TestFileTaskResultStorage_Basic Table-Driven 방식을 사용한 기본 기능 테스트
func TestFileTaskResultStorage_Basic(t *testing.T) {
	type TestData struct {
		Value string `json:"value"`
		Count int    `json:"count"`
	}

	tests := []struct {
		name      string
		taskID    ID
		commandID CommandID
		input     *TestData
		wantErr   bool
	}{
		{
			name:      "정상적인 저장 및 로드",
			taskID:    ID("TASK_001"),
			commandID: CommandID("CMD_001"),
			input:     &TestData{Value: "hello", Count: 1},
			wantErr:   false,
		},
		{
			name:      "특수 문자가 포함된 ID (에러 예상)",
			taskID:    ID("TASK-@#$"),
			commandID: CommandID("CMD_!&*"),
			input:     &TestData{Value: "special", Count: 99},
			wantErr:   true, // 특수문자 등으로 인해 경로 생성 실패 또는 Sanitizing 정책에 의해 거부될 수 있음
		},
		{
			name:      "빈 데이터 저장",
			taskID:    ID("TASK_EMPTY"),
			commandID: CommandID("CMD_EMPTY"),
			input:     &TestData{Value: "", Count: 0},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewFileTaskResultStorage(testAppName)
			tempDir := t.TempDir()
			storage.SetBaseDir(tempDir)

			// Save
			err := storage.Save(tt.taskID, tt.commandID, tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Load
			output := &TestData{}
			err = storage.Load(tt.taskID, tt.commandID, output)
			require.NoError(t, err)

			// Verify
			assert.Equal(t, tt.input, output)
		})
	}
}

// TestFileTaskResultStorage_Load_NonExistentFile 존재하지 않는 파일 읽기 테스트
func TestFileTaskResultStorage_Load_NonExistentFile(t *testing.T) {
	storage := NewFileTaskResultStorage(testAppName)
	storage.SetBaseDir(t.TempDir())

	type TestData struct{ Val string }
	data := &TestData{}

	err := storage.Load(ID("NO_FILE"), CommandID("NO_CMD"), data)
	assert.NoError(t, err, "존재하지 않는 파일은 에러 없이 무시되어야 함")
	assert.Empty(t, data.Val)
}

// TestFileTaskResultStorage_Security 보안 관련 테스트 (Path Traversal)
func TestFileTaskResultStorage_Security(t *testing.T) {
	storage := NewFileTaskResultStorage(testAppName)
	storage.SetBaseDir(t.TempDir())

	type TestData struct{ Val string }
	data := &TestData{Val: "hacked"}

	// 공격 시도: 상위 디렉토리 접근 문자 포함
	// Stringer인 ID를 통해 resolvePath 내부에서 체크됨
	taskID := ID("../hack_task")
	commandID := CommandID("cmd")

	// Save 시도
	// 주의: 내부적으로 strutil.ToSnakeCase를 사용하므로 "../"가 안전한 문자(예: "--")로 변환될 수 있음.
	// 하지만 파일 생성 과정에서 시스템 에러가 발생하거나 Path Traversal 에러가 발생하여 차단되어야 함.
	err := storage.Save(taskID, commandID, data)
	assert.Error(t, err, "경로 조작 시도는 에러로 차단되어야 합니다")

	// Load 시도 (Panic 확인용)
	readData := &TestData{}
	_ = storage.Load(taskID, commandID, readData)
}

// TestFileTaskResultStorage_Cleanup 임시 파일 정리 테스트
func TestFileTaskResultStorage_Cleanup(t *testing.T) {
	storage := NewFileTaskResultStorage(testAppName)
	tempDir := t.TempDir()
	storage.SetBaseDir(tempDir)

	// 더미 임시 파일 생성
	tmpFile := filepath.Join(tempDir, "task-result-dummy.tmp")
	err := os.WriteFile(tmpFile, []byte("dummy data"), 0644)
	require.NoError(t, err)

	// 일반 파일 생성 (삭제되면 안 됨)
	normalFile := filepath.Join(tempDir, "task-result-normal.json")
	err = os.WriteFile(normalFile, []byte("{}"), 0644)
	require.NoError(t, err)

	// 정리 수행
	storage.CleanupTempFiles()

	// 검증
	assert.NoFileExists(t, tmpFile, "임시 파일(.tmp)은 삭제되어야 합니다")
	assert.FileExists(t, normalFile, "일반 파일(.json)은 삭제되면 안 됩니다")
}

// TestFileTaskResultStorage_Concurrency 동시성 테스트
func TestFileTaskResultStorage_Concurrency(t *testing.T) {
	storage := NewFileTaskResultStorage(testAppName)
	tempDir := t.TempDir()
	storage.SetBaseDir(tempDir)

	type Data struct {
		Counter int `json:"counter"`
	}

	t.Run("동일한 파일에 대한 동시 쓰기/읽기", func(t *testing.T) {
		taskID := ID("CONCURRENCY_TASK")
		commandID := CommandID("SAME_KEY")
		workers := 50
		var wg sync.WaitGroup

		// 초기 파일 생성
		require.NoError(t, storage.Save(taskID, commandID, &Data{Counter: 0}))

		wg.Add(workers)
		for i := 0; i < workers; i++ {
			go func(val int) {
				defer wg.Done()
				// 쓰기
				_ = storage.Save(taskID, commandID, &Data{Counter: val})
				// 읽기
				var d Data
				_ = storage.Load(taskID, commandID, &d)
			}(i)
		}
		wg.Wait()

		// 최종적으로 에러 없이 파일이 존재하고 읽혀야 함
		var final Data
		err := storage.Load(taskID, commandID, &final)
		assert.NoError(t, err)
		// 값은 마지막에 쓴 고루틴에 따라 달라지므로 특정 값 검증은 어려우나, 파일이 깨지지는 않아야 함
	})

	t.Run("서로 다른 파일에 대한 병렬 처리", func(t *testing.T) {
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
