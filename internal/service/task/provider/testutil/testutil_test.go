package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	contractmocks "github.com/darkkaiser/notify-server/internal/service/contract/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMockTaskConfig(t *testing.T) {
	taskID := contract.TaskID("test-task")
	cmdID := contract.TaskCommandID("test-cmd")

	config := NewMockTaskConfig(taskID, cmdID)

	require.NotNil(t, config)
	require.Len(t, config.Commands, 1)
	assert.Equal(t, cmdID, config.Commands[0].ID)
	assert.True(t, config.Commands[0].AllowMultiple)
	assert.Nil(t, config.Commands[0].NewSnapshot())

	// Test NewTask from config
	newTask, err := config.NewTask(provider.NewTaskParams{
		Request: &contract.TaskSubmitRequest{
			TaskID:     taskID,
			CommandID:  cmdID,
			NotifierID: "test-notifier",
			RunBy:      contract.TaskRunByUser,
		},
		InstanceID:  "test-instance",
		Storage:     &contractmocks.MockTaskResultStore{},
		NewSnapshot: config.Commands[0].NewSnapshot,
	})
	require.NoError(t, err)
	require.NotNil(t, newTask)
	assert.Equal(t, taskID, newTask.ID())
}

func TestNewMockTaskConfigWithSnapshot(t *testing.T) {
	taskID := contract.TaskID("test-task")
	cmdID := contract.TaskCommandID("test-cmd")
	snapshot := map[string]string{"key": "value"}

	config := NewMockTaskConfigWithSnapshot(taskID, cmdID, snapshot)

	require.NotNil(t, config)
	require.Len(t, config.Commands, 1)
	assert.Equal(t, snapshot, config.Commands[0].NewSnapshot())
}

func TestNewMockTask(t *testing.T) {
	taskID := contract.TaskID("test-task")
	cmdID := contract.TaskCommandID("test-cmd")
	instanceID := contract.TaskInstanceID("test-instance")
	notifierID := contract.NotifierID("test-notifier")

	// 1. Without Fetcher
	task := NewMockTask(taskID, cmdID, instanceID, notifierID, contract.TaskRunByUser, nil, nil, nil)
	require.NotNil(t, task)
	assert.Equal(t, taskID, task.ID())
	assert.Equal(t, cmdID, task.CommandID())
	assert.Equal(t, instanceID, task.InstanceID())
	assert.Equal(t, notifierID, task.NotifierID())
	assert.Nil(t, task.Scraper())

	// 2. With Fetcher
	mockFetcher := mocks.NewMockHTTPFetcher()
	taskWithScraper := NewMockTask(taskID, cmdID, instanceID, notifierID, contract.TaskRunByUser, nil, mockFetcher, nil)
	require.NotNil(t, taskWithScraper)
	assert.NotNil(t, taskWithScraper.Scraper())
}

func TestRegisterMockTask(t *testing.T) {
	mockStorage := &contractmocks.MockTaskResultStore{}
	taskID := contract.TaskID("test-task")
	cmdID := contract.TaskCommandID("test-cmd")
	snapshot := map[string]string{"key": "value"}

	RegisterMockTask(mockStorage, taskID, cmdID, snapshot)

	// Verify Load populates the data CORRECTLY
	var loadedData map[string]string
	err := mockStorage.Load(taskID, cmdID, &loadedData)
	require.NoError(t, err)
	assert.Equal(t, snapshot, loadedData)
}

func TestLoadTestData(t *testing.T) {
	filename := "test_load.txt"
	content := "hello world"
	filePath := CreateTestFile(t, filename, content) // Create file in temp dir first
	_ = filePath                                     // filePath is not used directly, but the file creation is important.

	// LoadTestData assumes "testdata" directory relative to execution
	// Since we can't easily write to source "testdata" in robust test without cleanup issues,
	// we will skip actual file reading from "testdata" OR assume a file exists.
	// BETTER approach: Create a temporary "testdata" directory within the test temp dir
	// and run the test there? OR just mock os.ReadFile?
	// Given testutil is simple, let's just create the directory for this test.

	wd, _ := os.Getwd()
	testDataDir := filepath.Join(wd, "testdata")
	if err := os.MkdirAll(testDataDir, 0755); err != nil {
		t.Skip("Skipping TestLoadTestData because cannot create testdata dir")
	}
	// Cleanup happens automatically if we use t.TempDir but here we are in source tree.
	// So we should be careful.
	// Actually, `CreateTestFile` creates in `t.TempDir()`. `LoadTestData` reads from `testdata/filename`.
	// They are incompatible for this test unless we mock `testdata` dir.
	// Implementation of LoadTestData uses `filepath.Join("testdata", filename)`.
	// Let's create a file there and defer remove.

	absPath := filepath.Join(testDataDir, filename)
	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write to testdata: %v", err)
	}
	defer os.Remove(absPath)

	data := LoadTestData(t, filename)
	assert.Equal(t, content, string(data))

	str := LoadTestDataAsString(t, filename)
	assert.Equal(t, content, str)
}

func TestCreateTestFile(t *testing.T) {
	filename := "test.csv"
	content := "header1,header2\nvalue1,value2"

	filePath := CreateTestFile(t, filename, content)

	// Check file existence
	_, err := os.Stat(filePath)
	assert.NoError(t, err)

	// Check content
	readContent, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Equal(t, content, string(readContent))
	assert.Equal(t, filename, filepath.Base(filePath))
}

func TestCreateTestJSONFile(t *testing.T) {
	filename := "test.json"
	data := map[string]string{"key": "value"}

	filePath := CreateTestJSONFile(t, filename, data)

	// Read and Verify
	fileData, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var parsed map[string]string
	err = json.Unmarshal(fileData, &parsed)
	require.NoError(t, err)
	assert.Equal(t, data, parsed)
}
