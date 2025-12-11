package task

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/darkkaiser/notify-server/config"
)

// TestHelpers - 테스트 헬퍼 함수들

// CreateTestTask 테스트용 Task 인스턴스를 생성합니다.
func CreateTestTask(id ID, commandID CommandID, instanceID InstanceID) *Task {
	return &Task{
		ID:         id,
		CommandID:  commandID,
		InstanceID: instanceID,
		NotifierID: "test_notifier",
		Canceled:   false,
		RunBy:      RunByUser,
		Storage:    &MockTaskResultStorage{},
	}
}

// CreateTestConfig 테스트용 AppConfig를 생성합니다.
func CreateTestConfig() *config.AppConfig {
	return &config.AppConfig{
		Debug: true,
		Notifiers: config.NotifierConfig{
			DefaultNotifierID: "test-notifier",
			Telegrams: []config.TelegramConfig{
				{
					ID:       "test-notifier",
					BotToken: "test-token",
					ChatID:   12345,
				},
			},
		},
		Tasks: []config.TaskConfig{},
		NotifyAPI: config.NotifyAPIConfig{
			WS: config.WSConfig{
				TLSServer:  false,
				ListenPort: 18080,
			},
		},
	}
}

// CreateTestCSVFile 테스트용 CSV 파일을 생성합니다.
func CreateTestCSVFile(t *testing.T, filename string, content string) string {
	tempDir := CreateTestTempDir(t)
	filePath := filepath.Join(tempDir, filename)

	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("테스트 CSV 파일 생성 실패: %v", err)
	}

	return filePath
}

// CreateTestTempDir 테스트용 임시 디렉토리를 생성합니다.
// 테스트 종료 시 자동으로 정리됩니다.
func CreateTestTempDir(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "notify-server-test-*")
	if err != nil {
		t.Fatalf("임시 디렉토리 생성 실패: %v", err)
	}

	// 테스트 종료 시 자동 정리
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	return tempDir
}

// LoadTestDataAsString testdata 디렉토리에서 파일을 문자열로 로드합니다.
func LoadTestDataAsString(t *testing.T, filename string) string {
	data := LoadTestData(t, filename)
	return string(data)
}

// LoadTestData testdata 디렉토리에서 파일을 로드합니다.
func LoadTestData(t *testing.T, filename string) []byte {
	// testdata 디렉토리 경로 구성
	testdataPath := filepath.Join("testdata", filename)

	// 파일 읽기
	data, err := os.ReadFile(testdataPath)
	if err != nil {
		t.Fatalf("테스트 데이터 로드 실패 (%s): %v", testdataPath, err)
	}

	return data
}

// CreateTestConfigWithTasks Task가 포함된 테스트용 AppConfig를 생성합니다.
func CreateTestConfigWithTasks(tasks []struct {
	ID       string
	Title    string
	Commands []struct {
		ID                string
		Title             string
		Runnable          bool
		TimeSpec          string
		DefaultNotifierID string
	}
}) *config.AppConfig {
	appConfig := CreateTestConfig()

	// Tasks 추가
	for _, task := range tasks {
		configTask := config.TaskConfig{
			ID:    task.ID,
			Title: task.Title,
		}

		// Commands 추가
		for _, cmd := range task.Commands {
			configCmd := config.TaskCommandConfig{
				ID:    cmd.ID,
				Title: cmd.Title,
				Scheduler: struct {
					Runnable bool   `json:"runnable"`
					TimeSpec string `json:"time_spec"`
				}{
					Runnable: cmd.Runnable,
					TimeSpec: cmd.TimeSpec,
				},
				DefaultNotifierID: cmd.DefaultNotifierID,
			}

			configTask.Commands = append(configTask.Commands, configCmd)
		}

		appConfig.Tasks = append(appConfig.Tasks, configTask)
	}

	return appConfig
}
