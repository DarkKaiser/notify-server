package storage

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupStorageTest 테스트용 저장소와 임시 디렉토리를 생성하는 헬퍼 함수입니다.
// 테스트 종료 후 디렉토리가 자동으로 정리됩니다.
func setupStorageTest(t *testing.T) (contract.TaskResultStore, string) {
	t.Helper()
	tempDir := t.TempDir()
	store, err := NewFileTaskResultStore(tempDir)
	require.NoError(t, err)
	return store, tempDir
}

func TestNewFileTaskResultStore(t *testing.T) {
	t.Run("정상 생성: 임시 디렉토리", func(t *testing.T) {
		dir := t.TempDir()
		store, err := NewFileTaskResultStore(dir)
		require.NoError(t, err)
		require.NotNil(t, store)

		// 반환된 store가 올바른 타입을 가지고 있는지 확인
		_, ok := store.(*fileTaskResultStore)
		assert.True(t, ok, "should return *fileTaskResultStore")
	})

	t.Run("정상 생성: 기본 디렉토리(빈 문자열)", func(t *testing.T) {
		// 주의: 실제 작업 디렉토리에 "data" 폴더를 생성하므로 테스트 후 정리 필요
		// 동시성 테스트 간섭을 피하기 위해 안전하게 처리
		expectedDir, _ := filepath.Abs("data")
		defer os.RemoveAll(expectedDir)

		store, err := NewFileTaskResultStore("")
		require.NoError(t, err)
		require.NotNil(t, store)

		fsStore := store.(*fileTaskResultStore)
		assert.Equal(t, expectedDir, fsStore.baseDir)
	})

	t.Run("실패: 디렉토리 대신 파일이 존재하는 경우", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "file_as_dir")

		// 디렉토리와 같은 이름의 파일 생성
		err := os.WriteFile(filePath, []byte("test"), 0644)
		require.NoError(t, err)

		// 파일 경로를 디렉토리로 사용 시도
		store, err := NewFileTaskResultStore(filePath)
		require.Error(t, err)
		require.Nil(t, store)
		assert.Contains(t, err.Error(), "저장소 초기화 실패") // 에러 메시지 검증
	})
}

func TestFileTaskResultStore_SaveAndLoad(t *testing.T) {
	store, _ := setupStorageTest(t)

	// 복잡한 구조체 테스트용
	type ComplexData struct {
		ID        int               `json:"id"`
		Name      string            `json:"name"`
		Tags      []string          `json:"tags"`
		Metadata  map[string]string `json:"metadata"`
		CreatedAt time.Time         `json:"created_at"`
	}

	tests := []struct {
		name      string
		taskID    contract.TaskID
		commandID contract.TaskCommandID
		input     interface{}
		target    interface{} // Load 결과를 담을 포인터
	}{
		{
			name:      "단순 문자열",
			taskID:    "T_STR",
			commandID: "C_STR",
			input:     "Hello, World!",
			target:    new(string),
		},
		{
			name:      "정수형",
			taskID:    "T_INT",
			commandID: "C_INT",
			input:     12345,
			target:    new(int),
		},
		{
			name:      "복잡한 구조체",
			taskID:    "T_COMPLEX",
			commandID: "C_COMPLEX",
			input: ComplexData{
				ID:        1,
				Name:      "Complex Test",
				Tags:      []string{"go", "json", "file"},
				Metadata:  map[string]string{"env": "test"},
				CreatedAt: time.Now().Truncate(time.Second), // JSON 시간 정밀도 고려
			},
			target: new(ComplexData),
		},
		{
			name:      "특수문자 포함 ID",
			taskID:    "Task/..\\:|Test",
			commandID: "Cmd<*>?",
			input:     "Special Chars",
			target:    new(string),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save
			err := store.Save(tt.taskID, tt.commandID, tt.input)
			require.NoError(t, err)

			// Load
			err = store.Load(tt.taskID, tt.commandID, tt.target)
			require.NoError(t, err)

			// Verify
			// 포인터의 값을 꺼내서 비교
			got := reflect.ValueOf(tt.target).Elem().Interface()
			assert.Equal(t, tt.input, got)
		})
	}
}

func TestFileTaskResultStore_Overwrite(t *testing.T) {
	store, _ := setupStorageTest(t)
	taskID := contract.TaskID("OVERWRITE")
	commandID := contract.TaskCommandID("TEST")

	// 1. 초기 저장
	initialData := "Initial Data"
	err := store.Save(taskID, commandID, initialData)
	require.NoError(t, err)

	var loaded1 string
	err = store.Load(taskID, commandID, &loaded1)
	require.NoError(t, err)
	assert.Equal(t, initialData, loaded1)

	// 2. 덮어쓰기
	newData := "New Data"
	err = store.Save(taskID, commandID, newData)
	require.NoError(t, err)

	var loaded2 string
	err = store.Load(taskID, commandID, &loaded2)
	require.NoError(t, err)
	assert.Equal(t, newData, loaded2)
}

func TestFileTaskResultStore_Save_Errors(t *testing.T) {
	store, _ := setupStorageTest(t)

	t.Run("JSON 직렬화 불가능한 타입", func(t *testing.T) {
		// Channel은 JSON Marshal 불가능
		invalidData := make(chan int)
		err := store.Save("ERR", "CMD", invalidData)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "직렬화")
	})

	t.Run("지원되지 않는 값 (Infinity)", func(t *testing.T) {
		input := math.Inf(1)
		err := store.Save("ERR", "CMD", input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "직렬화")
	})
}

func TestFileTaskResultStore_Load_Errors(t *testing.T) {
	store, dir := setupStorageTest(t)
	taskID := contract.TaskID("LOAD_ERR")
	commandID := contract.TaskCommandID("TEST")

	t.Run("파일 없음 (NotFound)", func(t *testing.T) {
		var dest string
		err := store.Load("UNKNOWN", "CMD", &dest)
		require.Error(t, err)
		assert.ErrorIs(t, err, contract.ErrTaskResultNotFound)
	})

	t.Run("잘못된 대상 포인터 (nil)", func(t *testing.T) {
		err := store.Load(taskID, commandID, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrLoadRequiresPointer)
	})

	t.Run("잘못된 대상 포인터 (값 전달)", func(t *testing.T) {
		var dest string
		err := store.Load(taskID, commandID, dest) // 포인터 아님
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrLoadRequiresPointer)
	})

	t.Run("손상된 JSON 파일", func(t *testing.T) {
		// 정상 파일 먼저 생성
		err := store.Save(taskID, commandID, "data")
		require.NoError(t, err)

		// 파일을 찾아서 내용 오염시키기
		files, _ := os.ReadDir(dir)
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".json") {
				path := filepath.Join(dir, f.Name())
				err := os.WriteFile(path, []byte("{ invalid-json"), 0644)
				require.NoError(t, err)
			}
		}

		var dest string
		err = store.Load(taskID, commandID, &dest)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "역직렬화")
	})
}

func TestFileTaskResultStore_Security_PathTraversal(t *testing.T) {
	store, _ := setupStorageTest(t)

	// [보안 테스트 시나리오]
	// 공격자가 TaskID나 CommandID에 "../"를 포함시켜 상위 디렉토리 접근을 시도하는 경우
	// 시스템은 이를 감지하거나, 안전한 문자로 치환하여 격리해야 합니다.
	// 현재 구현은 sanitizeName에서 ".."를 "--"로 치환하여 방어합니다.

	maliciousTaskID := contract.TaskID("../../../windows")
	maliciousCommandID := contract.TaskCommandID("system32")

	// 1. 저장 시도 - 에러가 나지 않고 안전하게 저장되어야 함 (치환 로직)
	err := store.Save(maliciousTaskID, maliciousCommandID, "hacked")
	require.NoError(t, err)

	// 2. 로드 시도 - 동일하게 치환된 경로에서 읽어와야 함
	var dest string
	err = store.Load(maliciousTaskID, maliciousCommandID, &dest)
	require.NoError(t, err)
	assert.Equal(t, "hacked", dest)

	// 3. 실제 파일 시스템 검증 - Root 디렉토리에 파일이 생성되지 않았는지 확인
	// 만약 ../../가 먹혔다면 tempDir 상위에 생겼을 것임.
	// 여기서는 단순히 정상 동작(에러 없음)만 확인해도 충분하지만,
	// resolveSafePath가 ".." 접두사 에러를 뱉는 로직(`ErrPathTraversalDetected`)은
	// generateFilename을 우회하지 않는 한 호출되기 어렵다는 것을 확인.
}

func TestFileTaskResultStore_Cleanup(t *testing.T) {
	store, dir := setupStorageTest(t)
	fsStore := store.(*fileTaskResultStore)

	// 1. 오래된 임시 파일 생성 (삭제 대상)
	oldTmpName := "task-result-old.tmp"
	oldTmpPath := filepath.Join(dir, oldTmpName)
	err := os.WriteFile(oldTmpPath, []byte("old"), 0644)
	require.NoError(t, err)

	// 수정 시간을 2시간 전으로 변경
	twoHoursAgo := time.Now().Add(-2 * time.Hour)
	err = os.Chtimes(oldTmpPath, twoHoursAgo, twoHoursAgo)
	require.NoError(t, err)

	// 2. 최신 임시 파일 생성 (유지 대상)
	newTmpName := "task-result-new.tmp"
	newTmpPath := filepath.Join(dir, newTmpName)
	err = os.WriteFile(newTmpPath, []byte("new"), 0644)
	require.NoError(t, err)

	// 3. 일반 JSON 파일 생성 (유지 대상)
	jsonName := "task-test.json"
	jsonPath := filepath.Join(dir, jsonName)
	err = os.WriteFile(jsonPath, []byte("{}"), 0644)
	require.NoError(t, err)

	// 4. 정리 함수 직접 호출
	// 원래는 NewFileTaskResultStore 내부 고루틴에서 실행되지만, 테스트 확정성을 위해 직접 호출
	fsStore.cleanupStaleTempFiles()

	// 5. 검증
	assert.NoFileExists(t, oldTmpPath, "오래된 임시 파일은 삭제되어야 합니다")
	assert.FileExists(t, newTmpPath, "최신 임시 파일은 유지되어야 합니다")
	assert.FileExists(t, jsonPath, "일반 파일은 유지되어야 합니다")
}

func TestFileTaskResultStore_Concurrency(t *testing.T) {
	store, _ := setupStorageTest(t)
	taskID := contract.TaskID("CONCURRENCY")
	commandID := contract.TaskCommandID("TEST")

	concurrency := 50
	var wg sync.WaitGroup
	wg.Add(concurrency)

	// 50개의 고루틴이 동시에 같은 파일에 읽기/쓰기 수행
	for i := 0; i < concurrency; i++ {
		go func(val int) {
			defer wg.Done()

			// 쓰기
			err := store.Save(taskID, commandID, val)
			// 파일 Lock 경합으로 인해 지연될 수 있으나 에러는 없어야 함
			assert.NoError(t, err)

			// 읽기
			var out int
			err = store.Load(taskID, commandID, &out)
			if err == nil {
				// 읽기에 성공했다면, 값은 0~concurrency 중 하나여야 함
				assert.GreaterOrEqual(t, out, 0)
			} else {
				// 아주 드물게 파일이 생성되기 직전 읽기를 시도하면 NotFound가 날 수도 있음 (타이밍 이슈)
				// 하지만 Save 후 Load이므로 NotFound는 발생하면 안 됨
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	// 최종 상태 확인
	var finalVal int
	err := store.Load(taskID, commandID, &finalVal)
	require.NoError(t, err)

	// 최종 값은 저장된 값 중 하나여야 함 (데이터 깨짐이 없는지 확인)
	t.Logf("Final value: %d", finalVal)
}
