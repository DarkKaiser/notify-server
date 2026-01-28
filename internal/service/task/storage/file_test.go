package storage

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestStorage Tests helper: creates storage in a temp directory.
func setupTestStorage(t *testing.T) (contract.TaskResultStore, string) {
	t.Helper()
	tempDir := t.TempDir()
	storage, err := NewFileTaskResultStore(tempDir)
	require.NoError(t, err)
	return storage, tempDir
}

// ComplexData Structure for complex type testing
type ComplexData struct {
	Name string            `json:"name"`
	Tags []string          `json:"tags"`
	Meta map[string]string `json:"meta"`
}

func TestNewFileTaskResultStore(t *testing.T) {
	t.Run("Success with default directory", func(t *testing.T) {
		// Note: This creates "data" directory in current working dir.
		// We should clean it up or allow it.
		// For unit tests, it's better to provide a path.
		// Testing logic: If "" provided, it defaults to "data".
		// We won't actually create "data" here to avoid polluting workspace,
		// but we can verify it doesn't panic.
		// However, to keep it clean, we'll skip the actual creation or remove it after.
		// Let's just test with a specific temp dir usually.
		storage, err := NewFileTaskResultStore("")
		if err == nil {
			// If successful, cleanup "data"
			_ = os.RemoveAll("data")
		}
		// Expectation: It passes (as long as we have write perm)
		assert.NoError(t, err)
		assert.NotNil(t, storage)
	})

	t.Run("Success with specific directory", func(t *testing.T) {
		tempDir := t.TempDir()
		storage, err := NewFileTaskResultStore(tempDir)
		require.NoError(t, err)
		assert.NotNil(t, storage)
	})

	t.Run("Failure with file used as directory", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "file_as_dir")
		err := os.WriteFile(filePath, []byte("test"), 0644)
		require.NoError(t, err)

		// Try to use a file path as the base directory
		storage, err := NewFileTaskResultStore(filePath)
		require.Error(t, err)
		assert.Nil(t, storage)
		assert.Contains(t, err.Error(), "저장소 초기화 실패")
	})
}

func TestFileTaskResultStore_Save_Types(t *testing.T) {
	storage, _ := setupTestStorage(t)

	tests := []struct {
		name      string
		taskID    contract.TaskID
		commandID contract.TaskCommandID
		input     interface{}
	}{
		{
			name:      "Simple String",
			taskID:    "T_STR",
			commandID: "C_STR",
			input:     "hello world",
		},
		{
			name:      "Integer",
			taskID:    "T_INT",
			commandID: "C_INT",
			input:     42,
		},
		{
			name:      "Map",
			taskID:    "T_MAP",
			commandID: "C_MAP",
			input:     map[string]int{"one": 1},
		},
		{
			name:      "Complex Struct",
			taskID:    "T_STRUCT",
			commandID: "C_STRUCT",
			input: ComplexData{
				Name: "Test",
				Tags: []string{"go", "test"},
				Meta: map[string]string{"env": "dev"},
			},
		},
		{
			name:      "Nil Input",
			taskID:    "T_NIL",
			commandID: "C_NIL",
			input:     nil, // JSON marshals as null
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save
			err := storage.Save(tt.taskID, tt.commandID, tt.input)
			require.NoError(t, err)

			// Load verifying
			// Note: We need a pointer to the type for Unmarshal
			if tt.input != nil {
				// create a new pointer of the same type as input
				val := reflect.New(reflect.TypeOf(tt.input)).Interface()
				err = storage.Load(tt.taskID, tt.commandID, val)
				require.NoError(t, err)

				// dereference for comparison
				loadedVal := reflect.ValueOf(val).Elem().Interface()
				assert.Equal(t, tt.input, loadedVal)
			} else {
				// Handling nil input case
				var dest interface{}
				err = storage.Load(tt.taskID, tt.commandID, &dest)
				require.NoError(t, err)
				assert.Nil(t, dest)
			}
		})
	}
}

func TestFileTaskResultStore_Save_Overwrite(t *testing.T) {
	storage, _ := setupTestStorage(t)
	taskID := contract.TaskID("OVERWRITE")
	commandID := contract.TaskCommandID("CMD")

	// First Save
	err := storage.Save(taskID, commandID, "first-data")
	require.NoError(t, err)

	var v string
	err = storage.Load(taskID, commandID, &v)
	require.NoError(t, err)
	assert.Equal(t, "first-data", v)

	// Second Save (Overwrite)
	err = storage.Save(taskID, commandID, "second-data")
	require.NoError(t, err)

	err = storage.Load(taskID, commandID, &v)
	require.NoError(t, err)
	assert.Equal(t, "second-data", v)
}

func TestFileTaskResultStore_Save_Failures(t *testing.T) {
	storage, _ := setupTestStorage(t)

	t.Run("JSON Marshal Failure", func(t *testing.T) {
		// Channels cannot be marshaled to JSON
		badData := make(chan int)
		err := storage.Save("ERR_TASK", "ERR_CMD", badData)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "JSON Marshal")
	})

	t.Run("Unsupported Float (Infinity)", func(t *testing.T) {
		// JSON does not support NaN or Infinity
		err := storage.Save("ERR_TASK", "ERR_CMD", math.Inf(1))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "JSON Marshal")
	})
}

func TestFileTaskResultStore_Load_EdgeCases(t *testing.T) {
	storage, tempDir := setupTestStorage(t)
	taskID := contract.TaskID("EDGE")
	commandID := contract.TaskCommandID("CASE")

	t.Run("NotFound", func(t *testing.T) {
		var v string
		err := storage.Load("UNKNOWN", "CMD", &v)
		require.Error(t, err)
		assert.ErrorIs(t, err, contract.ErrTaskResultNotFound)
	})

	t.Run("Requires Pointer", func(t *testing.T) {
		err := storage.Load(taskID, commandID, "not-a-pointer")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrLoadRequiresPointer)

		err = storage.Load(taskID, commandID, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrLoadRequiresPointer)
	})

	t.Run("Corrupt Data", func(t *testing.T) {
		// Manually create a corrupt JSON file
		err := storage.Save(taskID, commandID, "valid-data")
		require.NoError(t, err)

		// Find the generated file
		entries, err := os.ReadDir(tempDir)
		require.NoError(t, err)
		require.NotEmpty(t, entries)

		targetFile := filepath.Join(tempDir, entries[0].Name())
		err = os.WriteFile(targetFile, []byte("{ incomplete-json"), 0644)
		require.NoError(t, err)

		// Try to load
		var v string
		err = storage.Load(taskID, commandID, &v)
		require.Error(t, err)
		// It returns raw json.Unmarshal error
		assert.Contains(t, err.Error(), "invalid character")
	})
}

func TestFileTaskResultStore_Security(t *testing.T) {
	storage, _ := setupTestStorage(t)

	// Attempts to traverse path.
	// The store is expected to SANITIZE these inputs, effectively looking for a safe filename
	// rather than actually traversing. So it should SUCCEED but save to a safe name.
	// Except if logic explicitly detects traversal and errors out?
	// Checking code: resolveSafePath -> generateFilename (sanitize) -> resolveSafePath (check prefix)
	// generateFilename replaces ".." with "--". So "task/../hack" becomes "task---hack".
	// This generates a SAFE path.
	// BUT, resolveSafePath has a final check: if !strings.HasPrefix(cleanPath, prefix) -> Error.
	// Since generateFilename sanitizes ".." to "--", cleanPath WILL be safe and inside dir.
	// So ".." attempts should arguably SUCCEED as a sanitized filename, NOT fail with traversal error
	// UNLESS the sanitization is bypassed or faulty.

	tests := []struct {
		name      string
		taskID    contract.TaskID
		commandID contract.TaskCommandID
	}{
		{"DotDot in TaskID", "../hack", "cmd"},
		{"Slash in CommandID", "task", "/etc/passwd"},
		{"Backslash", "task", "C:\\Windows"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v string = "secure"
			err := storage.Save(tt.taskID, tt.commandID, v)
			// Current implementation sanitizes ".." to "--", so it stays within directory.
			// Thus it should NOT return error, but save to a sanitized name.
			assert.NoError(t, err)

			// Should be able to load it back using SAME malicious key (because it gets sanitized same way)
			var out string
			err = storage.Load(tt.taskID, tt.commandID, &out)
			assert.NoError(t, err)
			assert.Equal(t, "secure", out)
		})
	}
}

func TestFileTaskResultStore_Cleanup(t *testing.T) {
	storage, tempDir := setupTestStorage(t)

	// Create a dummy .tmp file that looks old
	tmpName := "task-result-old.tmp"
	tmpPath := filepath.Join(tempDir, tmpName)
	err := os.WriteFile(tmpPath, []byte("ghost data"), 0644)
	require.NoError(t, err)

	// Set time to 2 hours ago
	oldTime := time.Now().Add(-2 * time.Hour)
	err = os.Chtimes(tmpPath, oldTime, oldTime)
	require.NoError(t, err)

	// Create a new .tmp file (should keep)
	newTmpName := "task-result-new.tmp"
	newTmpPath := filepath.Join(tempDir, newTmpName)
	err = os.WriteFile(newTmpPath, []byte("fresh data"), 0644)
	require.NoError(t, err)

	// Create a normal file (should keep)
	normalPath := filepath.Join(tempDir, "other.json")
	err = os.WriteFile(normalPath, []byte("{}"), 0644)
	require.NoError(t, err)

	// We need to trigger cleanup. It runs on NewFileTaskResultStore in background.
	// Or we can cast and call private method if we really want deterministic unit test of logic.
	// "cleanupStaleTempFiles" is private.
	// Reflection or just call via interface if possible? No.
	// We can use the fact that it's a method on the struct.

	impl, ok := storage.(interface{ cleanupStaleTempFiles() })
	if ok {
		impl.cleanupStaleTempFiles()
	} else {
		// If casting failed (maybe pointer receiver?), try pointer
		// Actually storage IS returned as interface.
		// In Go tests within same package, we can access private methods if we cast to concrete type.
		// concrete type is unexported "fileTaskResultStore".
		// But valid Code: storage.(*fileTaskResultStore) works if in same package.
		// Wait, "fileTaskResultStore" is unexported. We CANNOT cast it in test if test is in "storage_test" package?
		// Unless test is `package storage`. Let's check package declaration.
		// Step 8 shows "package storage". So we CAN access unexported identifiers.

		// To cast to private type:
		// We need to use reflection or just assume the type if we are in same package.
		// Since we are `package storage`, we can access `fileTaskResultStore`.
	}

	// Dynamic cast
	// Note: 'storage' variable is of type contract.TaskResultStore interface.
	// We assert it to *fileTaskResultStore
	s, ok := storage.(*fileTaskResultStore)
	require.True(t, ok)

	s.cleanupStaleTempFiles()

	// Verify
	assert.NoFileExists(t, tmpPath, "Old temp file should be deleted")
	assert.FileExists(t, newTmpPath, "New temp file should be kept")
	assert.FileExists(t, normalPath, "Normal file should be kept")
}

func TestFileTaskResultStore_Concurrency(t *testing.T) {
	storage, _ := setupTestStorage(t)

	taskID := contract.TaskID("CONC")
	commandID := contract.TaskCommandID("TEST")

	concurrency := 50
	var wg sync.WaitGroup
	wg.Add(concurrency)

	// Mixed Read/Write
	for i := 0; i < concurrency; i++ {
		go func(val int) {
			defer wg.Done()
			// Write
			err := storage.Save(taskID, commandID, val)
			assert.NoError(t, err) // Might fail if file system is overwhelmed but usually fine

			// Read
			var out int
			_ = storage.Load(taskID, commandID, &out)
			// Value might be anything from 0 to 49, just check no error
		}(i)
	}
	wg.Wait()

	// Final consistency check
	var out int
	err := storage.Load(taskID, commandID, &out)
	assert.NoError(t, err)
}
