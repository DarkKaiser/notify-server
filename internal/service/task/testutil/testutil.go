package testutil

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	tasksvc "github.com/darkkaiser/notify-server/internal/service/task"
	"github.com/darkkaiser/notify-server/internal/service/task/storage"
	"github.com/stretchr/testify/mock"
)

// MockTaskResultStorage 테스트용 Mock Storage 구현체입니다.
// TaskResultStorage 인터페이스를 만족하며, testify/mock을 사용하여 동작을 모의합니다.
type MockTaskResultStorage struct {
	mock.Mock
}

// Get 저장된 작업 결과를 조회합니다.
func (m *MockTaskResultStorage) Get(taskID contract.TaskID, commandID contract.TaskCommandID) (string, error) {
	args := m.Called(taskID, commandID)
	return args.String(0), args.Error(1)
}

// Save 작업 결과를 저장합니다.
func (m *MockTaskResultStorage) Save(taskID contract.TaskID, commandID contract.TaskCommandID, data interface{}) error {
	args := m.Called(taskID, commandID, data)
	return args.Error(0)
}

// SetStorage 내부 스토리지를 설정합니다. (Mock에서는 동작하지 않음)
func (m *MockTaskResultStorage) SetStorage(storage storage.TaskResultStorage) {
	// Mock에서는 아무것도 하지 않음
}

// Load 저장된 데이터를 불러옵니다.
func (m *MockTaskResultStorage) Load(taskID contract.TaskID, commandID contract.TaskCommandID, data interface{}) error {
	args := m.Called(taskID, commandID, data)
	return args.Error(0)
}

// MockHTTPFetcher 테스트용 Mock HTTP Fetcher 구현체입니다.
// URL별 응답을 미리 설정할 수 있으며, 동시성 테스트를 위해 스레드 안전(Thread-safe)하게 설계되었습니다.
type MockHTTPFetcher struct {
	mu            sync.Mutex
	Responses     map[string][]byte
	Errors        map[string]error
	Delays        map[string]time.Duration
	RequestedURLs []string
}

// NewMockHTTPFetcher 새로운 MockHTTPFetcher 인스턴스를 생성합니다.
func NewMockHTTPFetcher() *MockHTTPFetcher {
	return &MockHTTPFetcher{
		Responses:     make(map[string][]byte),
		Errors:        make(map[string]error),
		Delays:        make(map[string]time.Duration),
		RequestedURLs: make([]string, 0),
	}
}

// SetDelay 특정 URL 요청 시 응답 지연 시간을 설정합니다.
func (m *MockHTTPFetcher) SetDelay(url string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Delays[url] = d
}

// SetResponse 특정 URL에 대한 응답 바이트를 설정합니다.
func (m *MockHTTPFetcher) SetResponse(url string, response []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Responses[url] = response
}

// SetError 특정 URL 요청 시 반환할 에러를 설정합니다.
func (m *MockHTTPFetcher) SetError(url string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Errors[url] = err
}

// Get 설정된 Mock 응답을 반환합니다. 요청된 URL은 기록됩니다.
func (m *MockHTTPFetcher) Get(url string) (*http.Response, error) {
	m.mu.Lock()

	// 호출 기록 저장
	m.RequestedURLs = append(m.RequestedURLs, url)

	// 에러 설정 확인
	err := m.Errors[url]

	// 응답 설정 확인
	responseBody, hasResponse := m.Responses[url]

	// 지연 설정 확인
	delay, hasDelay := m.Delays[url]

	m.mu.Unlock()

	if hasDelay {
		time.Sleep(delay)
	}

	if err != nil {
		return nil, err
	}

	if hasResponse {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(responseBody)),
		}, nil
	}

	// 설정되지 않은 URL은 404 Not Found 반환
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(bytes.NewReader([]byte{})),
	}, nil
}

// Do http.Request를 받아 Get과 동일하게 처리합니다.
func (m *MockHTTPFetcher) Do(req *http.Request) (*http.Response, error) {
	return m.Get(req.URL.String())
}

// GetRequestedURLs 지금까지 요청된 모든 URL 목록을 반환합니다.
func (m *MockHTTPFetcher) GetRequestedURLs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	urls := make([]string, len(m.RequestedURLs))
	copy(urls, m.RequestedURLs)
	return urls
}

// Reset 모든 응답 설정과 요청 기록을 초기화합니다.
func (m *MockHTTPFetcher) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Responses = make(map[string][]byte)
	m.Errors = make(map[string]error)
	m.Delays = make(map[string]time.Duration)
	m.RequestedURLs = make([]string, 0)
}

// NewMockTaskConfig 테스트를 위한 기본 Task Config 인스턴스를 생성합니다.
func NewMockTaskConfig(taskID contract.TaskID, commandID contract.TaskCommandID) *tasksvc.Config {
	return NewMockTaskConfigWithSnapshot(taskID, commandID, nil)
}

// NewMockTaskConfigWithSnapshot 테스트를 위한 Task Config 인스턴스를 스냅샷과 함께 생성합니다.
func NewMockTaskConfigWithSnapshot(taskID contract.TaskID, commandID contract.TaskCommandID, snapshot interface{}) *tasksvc.Config {
	return &tasksvc.Config{
		Commands: []*tasksvc.CommandConfig{
			{
				ID:            commandID,
				AllowMultiple: true,
				NewSnapshot:   func() interface{} { return snapshot },
			},
		},
		NewTask: func(instanceID contract.TaskInstanceID, req *contract.TaskSubmitRequest, appConfig *config.AppConfig) (tasksvc.Handler, error) {
			t := NewMockTask(taskID, commandID, instanceID, "test_notifier", contract.TaskRunByUser)
			return &t, nil
		},
	}
}

// NewMockTask 테스트를 위한 Task 인스턴스를 생성하고 Mock Storage를 연결하여 반환합니다.
func NewMockTask(taskID contract.TaskID, commandID contract.TaskCommandID, instanceID contract.TaskInstanceID, notifierID contract.NotifierID, runBy contract.TaskRunBy) tasksvc.Base {
	t := tasksvc.NewBaseTask(taskID, commandID, instanceID, notifierID, runBy)
	t.SetStorage(&MockTaskResultStorage{})
	return t
}

// RegisterMockTask Mock TaskResultStorage에 특정 작업 결과를 미리 등록합니다.
func RegisterMockTask(storage *MockTaskResultStorage, taskID contract.TaskID, commandID contract.TaskCommandID, snapshot interface{}) {
	storage.On("Load", taskID, commandID, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2)
		if arg != nil {
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
