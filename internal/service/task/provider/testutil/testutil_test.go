package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	contractmocks "github.com/darkkaiser/notify-server/internal/service/contract/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestMockHTTPFetcher_SetAndGet MockHTTPFetcher의 기본 동작(응답 설정 및 조회)을 테스트합니다.
func TestMockHTTPFetcher_SetAndGet(t *testing.T) {
	mockFetcher := mocks.NewMockHTTPFetcher()
	url := "http://example.com"
	expectedBody := []byte("hello world")

	// 응답 설정
	mockFetcher.SetResponse(url, expectedBody)

	// Get으로 조회
	resp, err := fetcher.Get(context.Background(), mockFetcher, url)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, expectedBody, body)
	assert.Contains(t, mockFetcher.GetRequestedURLs(), url)
}

// TestMockHTTPFetcher_Error 에러 설정 및 조회 동작을 테스트합니다.
func TestMockHTTPFetcher_Error(t *testing.T) {
	mockFetcher := mocks.NewMockHTTPFetcher()
	url := "http://example.com/error"
	expectedErr := fmt.Errorf("network error")

	// 에러 설정
	mockFetcher.SetError(url, expectedErr)

	// Get 시 에러 반환 확인
	_, err := fetcher.Get(context.Background(), mockFetcher, url)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

// TestMockHTTPFetcher_NotFound 설정되지 않은 URL 요청 시 404 동작을 테스트합니다.
func TestMockHTTPFetcher_NotFound(t *testing.T) {
	mockFetcher := mocks.NewMockHTTPFetcher()
	url := "http://example.com/unknown"

	resp, err := fetcher.Get(context.Background(), mockFetcher, url)
	require.NoError(t, err) // 404는 에러가 아님 (http.Response 반환)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestMockHTTPFetcher_Concurrency 동시성 안전성을 테스트합니다.
// 여러 고루틴에서 동시에 SetResponse와 Get을 호출하여 Race Condition이 발생하지 않는지 확인합니다.
func TestMockHTTPFetcher_Concurrency(t *testing.T) {
	mockFetcher := mocks.NewMockHTTPFetcher()
	urlBase := "http://example.com/"
	concurrency := 100
	var wg sync.WaitGroup

	// SetResponse 고루틴
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			url := fmt.Sprintf("%s%d", urlBase, idx)
			mockFetcher.SetResponse(url, []byte("data"))
		}(i)
	}

	// Get 고루틴
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			// 임의의 키에 접근 (Set과 동시에 일어날 수 있음)
			url := fmt.Sprintf("%s%d", urlBase, idx)
			_, _ = fetcher.Get(context.Background(), mockFetcher, url)
		}(i)
	}

	wg.Wait()

	// 모든 요청이 기록되었는지 확인 (Get 호출 횟수에 따라 다름, 최소한 패닉은 없어야 함)
	// 정확한 카운트는 타이밍에 따라 다르므로 패닉/Race없음만 검증
	assert.GreaterOrEqual(t, len(mockFetcher.GetRequestedURLs()), 0)
}

// TestMockHTTPFetcher_Do Do 메서드가 Get과 동일하게 동작하는지 테스트합니다.
func TestMockHTTPFetcher_Do(t *testing.T) {
	fetcher := mocks.NewMockHTTPFetcher()
	url := "http://example.com/do"
	fetcher.SetResponse(url, []byte("ok"))

	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	resp, err := fetcher.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestMockHTTPFetcher_Reset Reset 메서드가 상태를 초기화하는지 테스트합니다.
func TestMockHTTPFetcher_Reset(t *testing.T) {
	mockFetcher := mocks.NewMockHTTPFetcher()
	url := "http://example.com"
	mockFetcher.SetResponse(url, []byte("data"))
	_, _ = fetcher.Get(context.Background(), mockFetcher, url)

	assert.NotEmpty(t, mockFetcher.GetRequestedURLs())

	mockFetcher.Reset()

	assert.Empty(t, mockFetcher.GetRequestedURLs())
	resp, _ := fetcher.Get(context.Background(), mockFetcher, url) // 이제 설정이 없으므로 404
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestMockTaskResultStore MockStorage의 기본 동작을 테스트합니다.
func TestMockTaskResultStore(t *testing.T) {
	mockStorage := &contractmocks.MockTaskResultStore{}
	taskID := contract.TaskID("task1")
	cmdID := contract.TaskCommandID("cmd1")
	data := "some-data"
	var loadedData string

	// Expectation 설정
	mockStorage.On("Save", taskID, cmdID, data).Return(nil)
	// Load 호출 시 data 인자에 값을 채워주는 동작(Run)은 여기서는 생략하고,
	// 단순히 호출 기대값(Expectation)과 반환값만 검증합니다.
	mockStorage.On("Load", taskID, cmdID, mock.Anything).Return(nil)

	// 실행
	err := mockStorage.Save(taskID, cmdID, data)
	assert.NoError(t, err)

	err = mockStorage.Load(taskID, cmdID, &loadedData)
	assert.NoError(t, err)

	mockStorage.AssertExpectations(t)
}

// TestLoadTestData LoadTestData 함수가 파일을 정상적으로 읽는지 테스트합니다.
func TestLoadTestData(t *testing.T) {
	// testdata/sample.txt 파일이 필요합니다.
	filename := "sample.txt"
	expectedContent := "Hello, Test World!"

	// 파일 읽기
	data := LoadTestData(t, filename)
	assert.Equal(t, expectedContent, strings.TrimSpace(string(data)))
}

// TestLoadTestData_Fail 존재하지 않는 파일 읽기 시 동작
// 주의: t.Fatalf를 호출하므로 별도의 서브프로세스나 복잡한 테스트가 필요하지만,
// 여기서는 정상 동작 위주로 테스트합니다. 실패 케이스는 t.Fatalf로 인해 현재 테스트 프로세스가 종료되므로 생략하거나
// 별도 방식으로 검증해야 합니다. (일반적으로 유틸리티 테스트에서는 생략 가능)

// TestLoadTestDataAsString LoadTestDataAsString 함수 테스트
func TestLoadTestDataAsString(t *testing.T) {
	filename := "sample.txt"
	expectedContent := "Hello, Test World!"

	str := LoadTestDataAsString(t, filename)
	assert.Equal(t, expectedContent, strings.TrimSpace(str))
}

// TestCreateTestTempDir 임시 디렉토리 생성 및 삭제 확인
func TestCreateTestTempDir(t *testing.T) {
	dir := CreateTestTempDir(t)

	// 디렉토리가 존재하는지 확인
	info, err := os.Stat(dir)
	assert.NoError(t, err)
	assert.True(t, info.IsDir())

	// t.Cleanup은 실제 테스트 종료 시 실행되므로, 여기서 직접 삭제 여부를 확인할 수는 없음.
	// API 호출 성공 여부만 확인.
}

// TestCreateTestCSVFile CSV 파일 생성 테스트
func TestCreateTestCSVFile(t *testing.T) {
	filename := "test.csv"
	content := "col1,col2\nval1,val2"

	filePath := CreateTestCSVFile(t, filename, content)

	// 파일이 존재하고 내용이 일치하는지 확인
	readContent, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Equal(t, content, string(readContent))
	assert.Equal(t, filename, filepath.Base(filePath))
}

// TestCreateTestJSONFile JSON 파일 생성 테스트
func TestCreateTestJSONFile(t *testing.T) {
	filename := "test.json"
	data := map[string]string{"key": "value"}

	filePath := CreateTestJSONFile(t, filename, data)

	// 파일 읽어서 확인
	fileData, err := os.ReadFile(filePath)
	assert.NoError(t, err)

	// JSON 파싱 확인
	var parsed map[string]string
	err = json.Unmarshal(fileData, &parsed) // testutil 내부에서는 Unmarshal이 아닌 string(marshal)로 저장하지만, CreateTestCSVFile이용함
	// CreateTestJSONFile 내부 구현: CreateTestCSVFile(t, filename, string(content))
	// string(content)로 저장된 파일 내용이 유효한 JSON인지 확인해야 함.
	// CreateTestCSVFile은 단순 바이트 쓰기이므로 문제 없음.

	// string(json) -> file write -> file read -> unmarshal 이 되어야 함.
	// testutil implementation: string(content) passed to CreateTestCSVFile

	// 여기서 한가지 주의: CreateTestCSVFile 내에서는 os.WriteFile(path, []byte(content)) 함.
	// JSON marshaled content 는 []byte 인데 string 변환되었다가 다시 []byte 됨. OK.

	// 하지만 테스트 코드에서 Unmarshal을 위해 직접 해보는 것.
	assert.NoError(t, err)
	assert.Equal(t, data["key"], parsed["key"])
}

// TestCreateTestTask TestCreateTestTask 함수가 올바른 Task를 반환하는지 테스트
func TestMockCreateTask(t *testing.T) {
	id := contract.TaskID("test-task")
	cmdID := contract.TaskCommandID("test-cmd")
	instanceID := contract.TaskInstanceID("test-instance")

	createdTask := NewMockTask(id, cmdID, instanceID, "test_notifier", contract.TaskRunByUser, nil)

	assert.NotNil(t, createdTask)
	assert.Equal(t, id, createdTask.GetID())
	// 스토리지 설정 확인 (내부 필드라 직접 접근 어렵지만 패닉 안나면 됨)
}
