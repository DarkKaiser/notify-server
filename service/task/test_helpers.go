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
)

// MockNotificationSender 테스트용 NotificationSender 구현체입니다.
type MockNotificationSender struct {
	mu sync.Mutex

	// 호출 기록
	NotifyToDefaultCalls           []string
	NotifyWithTaskContextCalls     []NotifyWithTaskContextCall
	SupportsHTMLMessageCalls       []string
	SupportsHTMLMessageReturnValue bool
}

// NotifyWithTaskContextCall NotifyWithTaskContext 호출 정보를 저장합니다.
type NotifyWithTaskContextCall struct {
	NotifierID string
	Message    string
	TaskCtx    TaskContext
}

// NewMockNotificationSender 새로운 Mock 객체를 생성합니다.
func NewMockNotificationSender() *MockNotificationSender {
	return &MockNotificationSender{
		NotifyToDefaultCalls:           make([]string, 0),
		NotifyWithTaskContextCalls:     make([]NotifyWithTaskContextCall, 0),
		SupportsHTMLMessageCalls:       make([]string, 0),
		SupportsHTMLMessageReturnValue: true, // 기본값: HTML 지원
	}
}

// NotifyToDefault 기본 알림을 전송합니다 (Mock).
func (m *MockNotificationSender) NotifyToDefault(message string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NotifyToDefaultCalls = append(m.NotifyToDefaultCalls, message)
	return true
}

// NotifyWithTaskContext Task 컨텍스트와 함께 알림을 전송합니다 (Mock).
func (m *MockNotificationSender) NotifyWithTaskContext(notifierID string, message string, taskCtx TaskContext) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NotifyWithTaskContextCalls = append(m.NotifyWithTaskContextCalls, NotifyWithTaskContextCall{
		NotifierID: notifierID,
		Message:    message,
		TaskCtx:    taskCtx,
	})
	return true
}

// SupportsHTMLMessage HTML 메시지 지원 여부를 반환합니다 (Mock).
func (m *MockNotificationSender) SupportsHTMLMessage(notifierID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SupportsHTMLMessageCalls = append(m.SupportsHTMLMessageCalls, notifierID)
	return m.SupportsHTMLMessageReturnValue
}

// Reset 모든 호출 기록을 초기화합니다.
func (m *MockNotificationSender) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NotifyToDefaultCalls = make([]string, 0)
	m.NotifyWithTaskContextCalls = make([]NotifyWithTaskContextCall, 0)
	m.SupportsHTMLMessageCalls = make([]string, 0)
}

// GetNotifyToDefaultCallCount NotifyToDefault 호출 횟수를 반환합니다.
func (m *MockNotificationSender) GetNotifyToDefaultCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.NotifyToDefaultCalls)
}

// GetNotifyWithTaskContextCallCount NotifyWithTaskContext 호출 횟수를 반환합니다.
func (m *MockNotificationSender) GetNotifyWithTaskContextCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.NotifyWithTaskContextCalls)
}

// GetSupportsHTMLMessageCallCount SupportsHTMLMessage 호출 횟수를 반환합니다.
func (m *MockNotificationSender) GetSupportsHTMLMessageCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.SupportsHTMLMessageCalls)
}

// MockHTTPFetcher HTTP 요청을 Mock하는 구조체입니다.
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

// CleanupTestFile 테스트 파일을 정리합니다.
func CleanupTestFile(t *Task) error {
	// 테스트 데이터 파일 삭제
	filename := t.dataFileName()
	return removeFileIfExists(filename)
}

// removeFileIfExists는 파일이 존재하면 삭제합니다.
func removeFileIfExists(filename string) error {
	// 파일 존재 여부 확인
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil // 파일이 없으면 성공
	}

	// 파일 삭제
	return os.Remove(filename)
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
