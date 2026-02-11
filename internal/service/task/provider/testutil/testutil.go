package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	contractmocks "github.com/darkkaiser/notify-server/internal/service/contract/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"

	"github.com/stretchr/testify/mock"
)

// NewMockTaskConfig 테스트를 위한 기본 Task Config 인스턴스를 생성합니다.
func NewMockTaskConfig(taskID contract.TaskID, commandID contract.TaskCommandID) *provider.Config {
	return NewMockTaskConfigWithSnapshot(taskID, commandID, nil)
}

// NewMockTaskConfigWithSnapshot 테스트를 위한 Task Config 인스턴스를 스냅샷과 함께 생성합니다.
func NewMockTaskConfigWithSnapshot(taskID contract.TaskID, commandID contract.TaskCommandID, snapshot interface{}) *provider.Config {
	return &provider.Config{
		Commands: []*provider.CommandConfig{
			{
				ID:            commandID,
				AllowMultiple: true,
				NewSnapshot:   func() interface{} { return snapshot },
			},
		},
		NewTask: func(instanceID contract.TaskInstanceID, req *contract.TaskSubmitRequest, appConfig *config.AppConfig, storage contract.TaskResultStore) (provider.Task, error) {
			t := NewMockTask(taskID, commandID, instanceID, "test_notifier", contract.TaskRunByUser, storage)
			return t, nil
		},
	}
}

// NewMockTask 테스트를 위한 Task 인스턴스를 생성하고 Mock Storage를 연결하여 반환합니다.
// NewMockTask 테스트를 위한 Task 인스턴스를 생성하고 Mock Storage를 연결하여 반환합니다.
func NewMockTask(taskID contract.TaskID, commandID contract.TaskCommandID, instanceID contract.TaskInstanceID, notifierID contract.NotifierID, runBy contract.TaskRunBy, storage contract.TaskResultStore) *provider.Base {
	if storage == nil {
		storage = &contractmocks.MockTaskResultStore{}
	}
	// Explicitly define the variable type to ensure compatibility with provider.NewBase return type
	var t *provider.Base = provider.NewBase(taskID, commandID, instanceID, notifierID, runBy, storage)
	return t
}

// RegisterMockTask Mock TaskResultStore에 특정 작업 결과를 미리 등록합니다.
func RegisterMockTask(storage *contractmocks.MockTaskResultStore, taskID contract.TaskID, commandID contract.TaskCommandID, snapshot interface{}) {
	storage.On("Load", taskID, commandID, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2)
		if arg != nil && snapshot != nil {
			// Reflect the snapshot into the provided data interface
			dataBytes, _ := json.Marshal(snapshot)
			json.Unmarshal(dataBytes, arg)
		}
	})
}

// LoadTestData testdata 디렉토리에서 파일을 읽어옵니다.
// 실패 시 t.Fatalf로 테스트를 중단합니다.
func LoadTestData(t *testing.T, filename string) []byte {
	t.Helper() // 테스트 실패 시 호출자를 가리키도록 설정

	testdataPath := filepath.Join("testdata", filename)
	data, err := os.ReadFile(testdataPath)
	if err != nil {
		t.Fatalf("테스트 데이터 로드 실패 (%s): %v", testdataPath, err)
	}
	return data
}

// LoadTestDataAsString testdata 디렉토리에서 파일을 읽어 문자열로 반환합니다.
func LoadTestDataAsString(t *testing.T, filename string) string {
	t.Helper()
	return string(LoadTestData(t, filename))
}

// CreateTestCSVFile 임시 디렉토리에 CSV 파일을 생성하고 경로를 반환합니다.
func CreateTestCSVFile(t *testing.T, filename string, content string) string {
	t.Helper()

	tempDir := CreateTestTempDir(t)
	filePath := filepath.Join(tempDir, filename)

	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("테스트 CSV 파일 생성 실패: %v", err)
	}

	return filePath
}

// CreateTestTempDir 테스트가 끝나면 자동으로 삭제되는 임시 디렉토리를 생성합니다.
func CreateTestTempDir(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "notify-server-test-*")
	if err != nil {
		t.Fatalf("임시 디렉토리 생성 실패: %v", err)
	}

	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	return dir
}

// CreateTestJSONFile 임의의 데이터를 JSON으로 변환하여 임시 파일로 저장하고 경로를 반환합니다.
func CreateTestJSONFile(t *testing.T, filename string, data interface{}) string {
	t.Helper()

	content, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("JSON 마샬링 실패: %v", err)
	}

	return CreateTestCSVFile(t, filename, string(content))
}
