package task

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/darkkaiser/notify-server/config"
	"github.com/stretchr/testify/mock"
)

// TestHelpers - 테스트 헬퍼 함수들

// MockTaskResultStorage 테스트용 Mock Storage
type MockTaskResultStorage struct {
	mock.Mock
}

func (m *MockTaskResultStorage) Get(taskID ID, commandID CommandID) (string, error) {
	args := m.Called(taskID, commandID)
	return args.String(0), args.Error(1)
}

func (m *MockTaskResultStorage) Save(taskID ID, commandID CommandID, data interface{}) error {
	args := m.Called(taskID, commandID, data)
	return args.Error(0)
}

func (m *MockTaskResultStorage) SetStorage(storage TaskResultStorage) {
	// Mock에서는 아무것도 하지 않음 or Mock 동작 정의
}

func (m *MockTaskResultStorage) Load(taskID ID, commandID CommandID, data interface{}) error {
	args := m.Called(taskID, commandID, data)
	return args.Error(0)
}

// MockHTTPFetcher 테스트용 Mock Fetcher
type MockHTTPFetcher struct {
	mu sync.Mutex

	// URL별 응답 설정
	Responses map[string][]byte // URL -> 응답 바이트
	Errors    map[string]error  // URL -> 에러

	// 호출 기록
	RequestedURLs []string
}

// NewMockHTTPFetcher 새로운 MockHTTPFetcher를 생성합니다.
func NewMockHTTPFetcher() *MockHTTPFetcher {
	return &MockHTTPFetcher{
		Responses:     make(map[string][]byte),
		Errors:        make(map[string]error),
		RequestedURLs: make([]string, 0),
	}
}

// SetResponse 특정 URL에 대한 응답을 설정합니다.
func (m *MockHTTPFetcher) SetResponse(url string, response []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Responses[url] = response
}

// SetError 특정 URL에 대한 에러를 설정합니다.
func (m *MockHTTPFetcher) SetError(url string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Errors[url] = err
}

// Get Mock HTTP Get 요청을 수행합니다.
func (m *MockHTTPFetcher) Get(url string) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 호출 기록 저장
	m.RequestedURLs = append(m.RequestedURLs, url)

	// 에러가 설정되어 있으면 에러 반환
	if err, ok := m.Errors[url]; ok {
		return nil, err
	}

	// 응답이 설정되어 있으면 응답 반환
	if responseBody, ok := m.Responses[url]; ok {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(responseBody)),
		}, nil
	}

	// 설정되지 않은 URL은 404 반환 (또는 빈 응답)
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(bytes.NewReader([]byte{})),
	}, nil
}

// Do Mock HTTP 요청을 수행합니다.
func (m *MockHTTPFetcher) Do(req *http.Request) (*http.Response, error) {
	return m.Get(req.URL.String())
}

// GetRequestedURLs 요청된 URL 목록을 반환합니다.
func (m *MockHTTPFetcher) GetRequestedURLs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	urls := make([]string, len(m.RequestedURLs))
	copy(urls, m.RequestedURLs)
	return urls
}

// Reset 모든 설정과 기록을 초기화합니다.
func (m *MockHTTPFetcher) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Responses = make(map[string][]byte)
	m.Errors = make(map[string]error)
	m.RequestedURLs = make([]string, 0)
}

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
