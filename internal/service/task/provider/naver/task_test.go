package naver

import (
	"context" // Added context import
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	contractmocks "github.com/darkkaiser/notify-server/internal/service/contract/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// === Mocks ===

type mockNotificationSender struct{}

func (m *mockNotificationSender) Notify(ctx context.Context, notification contract.Notification) error {
	return nil
}

func (m *mockNotificationSender) SupportsHTML(notifierTaskID contract.NotifierID) bool {
	return true
}

// === Tests ===

func TestCreateTask(t *testing.T) {
	h := newTestHelper(t)
	// createTask는 내부적으로 config를 조회하므로 appConfig가 필요함
	// initTask 호출 시 내부적으로 createTask를 검증함
	h.initTask(contract.TaskRunByUser)

	assert.NotNil(t, h.taskHandler)
	assert.IsType(t, &task{}, h.taskHandler)
}

func TestExecute_WatchNewPerformances(t *testing.T) {
	// 격리를 위한 Test Cleanup
	provider.ClearForTest()
	defer provider.ClearForTest()

	type testCase struct {
		name                 string
		runBy                contract.TaskRunBy
		mockSetup            func(b *mockResponseBuilder)
		prevSnapshot         *watchNewPerformancesSnapshot
		expectedMessage      string
		expectedSnapshotSize int
		expectError          bool
	}

	tests := []testCase{
		{
			name:  "성공: 신규 공연 발견 (User Run)",
			runBy: contract.TaskRunByUser,
			mockSetup: func(b *mockResponseBuilder) {
				b.page(1).returns("Musical A", "Musical B")
				b.page(2).returnsEmpty() // 종료 조건
			},
			prevSnapshot:         &watchNewPerformancesSnapshot{Performances: []*performance{}},
			expectedMessage:      "새로운 공연정보가 등록되었습니다.",
			expectedSnapshotSize: 2,
			expectError:          false,
		},
		{
			name:  "성공: 신규 공연 없음 - User Run (현재 상태 알림)",
			runBy: contract.TaskRunByUser,
			mockSetup: func(b *mockResponseBuilder) {
				b.page(1).returns("Musical A")
				b.page(2).returnsEmpty()
			},
			prevSnapshot: &watchNewPerformancesSnapshot{Performances: []*performance{
				{Title: "Musical A", Place: "Place"},
			}},
			expectedMessage:      "신규로 등록된 공연정보가 없습니다.", // 현재 상태 출력
			expectedSnapshotSize: -1,                    // 변경 없음 -> 스냅샷 nil
			expectError:          false,
		},
		{
			name:  "성공: 신규 공연 없음 - Scheduler Run (알림 없음)",
			runBy: contract.TaskRunByScheduler,
			mockSetup: func(b *mockResponseBuilder) {
				b.page(1).returns("Musical A")
				b.page(2).returnsEmpty()
			},
			prevSnapshot: &watchNewPerformancesSnapshot{Performances: []*performance{
				{Title: "Musical A", Place: "Place"},
			}},
			expectedMessage:      "", // 알림 없음
			expectedSnapshotSize: -1, // 변경 없음 -> 스냅샷 nil
			expectError:          false,
		},
		{
			name:  "실패: 네트워크 에러 발생",
			runBy: contract.TaskRunByUser,
			mockSetup: func(b *mockResponseBuilder) {
				b.page(1).failsWith(fmt.Errorf("network error"))
			},
			prevSnapshot:         &watchNewPerformancesSnapshot{},
			expectedMessage:      "",
			expectedSnapshotSize: 0,
			expectError:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHelper(t)
			h.initTask(tt.runBy)

			// Mock Setup
			builder := newMockResponseBuilder(h.fetcher)
			if tt.mockSetup != nil {
				tt.mockSetup(builder)
			}

			// Execution
			cmdConfig, _ := findCommandSettings(h.appConfig, TaskID, WatchNewPerformancesCommand)
			msg, newSnapshot, err := h.task.executeWatchNewPerformances(context.Background(), cmdConfig, tt.prevSnapshot, true)

			// Verification
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				if tt.expectedMessage != "" {
					assert.Contains(t, msg, tt.expectedMessage)
				} else {
					assert.Empty(t, msg)
				}

				if tt.expectedSnapshotSize >= 0 {
					snap, ok := newSnapshot.(*watchNewPerformancesSnapshot)
					if ok {
						assert.Len(t, snap.Performances, tt.expectedSnapshotSize)
					} else {
						assert.Fail(t, "Snapshot type assertion failed")
					}
				} else if tt.expectedSnapshotSize == -1 {
					// 스냅샷이 nil이어야 함
					assert.Nil(t, newSnapshot)
				}
			}
		})
	}
}

func TestTask_Run_Integration_Simulation(t *testing.T) {
	provider.ClearForTest()
	defer provider.ClearForTest()

	h := newTestHelper(t)

	// Mock Storage 설정 (Integration Test 전용)
	// Load 호출 시 "데이터 없음" -> 빈 스냅샷 시작
	h.storage.On("Load", TaskID, WatchNewPerformancesCommand, mock.Anything).Return(fmt.Errorf("no data"))
	// Save 호출 성공
	h.storage.On("Save", TaskID, WatchNewPerformancesCommand, mock.Anything).Return(nil)

	// Mock HTTP 설정
	b := newMockResponseBuilder(h.fetcher)
	b.page(1).returns("New Musical")
	b.page(2).returnsEmpty()

	// Task 초기화 (User Run)
	h.initTask(contract.TaskRunByUser)

	// Task 실행 (Run 메서드 직접 호출)
	var wg sync.WaitGroup
	quit := make(chan contract.TaskInstanceID, 1)
	sender := &mockNotificationSender{}

	wg.Add(1)
	// RunByUser는 한 번 실행 후 종료됨
	h.task.Run(context.Background(), sender, &wg, quit)

	// Verify Storage Interaction
	h.storage.AssertExpectations(t)

	// Run 로그 확인은 어렵지만 에러 없이 끝났으면 성공
}

// === Test Helpers ===

type testHelper struct {
	t           *testing.T
	fetcher     *mocks.MockHTTPFetcher
	storage     *contractmocks.MockTaskResultStore
	appConfig   *config.AppConfig
	taskHandler provider.Task
	task        *task
}

func newTestHelper(t *testing.T) *testHelper {
	// 항상 깨끗한 상태에서 시작 (Registry 초기화)
	provider.ClearForTest()

	// 모의 객체 생성
	fetcher := mocks.NewMockHTTPFetcher()
	storage := &contractmocks.MockTaskResultStore{}

	// 매 테스트마다 설정을 확실하게 다시 등록해야 함
	provider.Register(TaskID, &provider.Config{
		Commands: []*provider.CommandConfig{{
			ID: WatchNewPerformancesCommand,

			AllowMultiple: true,

			NewSnapshot: func() interface{} { return &watchNewPerformancesSnapshot{} },
		}},

		NewTask: newTask,
	})

	// 기본 설정 생성
	cfgBuilder := newConfigBuilder().
		withTask(string(TaskID), string(WatchNewPerformancesCommand), defaultTaskData())

	return &testHelper{
		t:         t,
		fetcher:   fetcher,
		storage:   storage,
		appConfig: cfgBuilder.build(),
	}
}

// initTask Task 인스턴스를 생성하고 의존성을 주입합니다.
func (h *testHelper) initTask(runBy contract.TaskRunBy) {
	req := &contract.TaskSubmitRequest{
		TaskID:     TaskID,
		CommandID:  WatchNewPerformancesCommand,
		NotifierID: "test_notifier",
		RunBy:      runBy, // Scheduler or User
	}

	handler, err := createTask("test_instance", req, h.appConfig, h.fetcher)
	require.NoError(h.t, err)

	h.taskHandler = handler
	h.task = handler.(*task)
	h.task.SetStorage(h.storage)
}

// === Mock Response Builder ===

type mockResponseBuilder struct {
	fetcher *mocks.MockHTTPFetcher
	baseURL string
	params  url.Values
}

func newMockResponseBuilder(fetcher *mocks.MockHTTPFetcher) *mockResponseBuilder {
	// 기본 파라미터 설정
	params := url.Values{}
	params.Set("where", "nexearch")
	params.Set("key", "kbList")
	params.Set("pkid", "269")
	params.Set("u1", "뮤지컬") // Default Query
	params.Set("u2", "all")
	params.Set("u3", "")
	params.Set("u4", "ingplan")
	params.Set("u5", "date")
	params.Set("u6", "N")
	params.Set("u8", "all")

	return &mockResponseBuilder{
		fetcher: fetcher,
		baseURL: "https://m.search.naver.com/p/csearch/content/nqapirender.nhn",
		params:  params,
	}
}

func (b *mockResponseBuilder) withQuery(query string) *mockResponseBuilder {
	b.params.Set("u1", query)
	return b
}

func (b *mockResponseBuilder) page(pageNum int) *pageBuilder {
	// 복사본을 만들어 해당 페이지 전용 빌더 반환
	pageParams := url.Values{}
	for k, v := range b.params {
		pageParams[k] = v
	}
	pageParams.Set("u7", strconv.Itoa(pageNum))

	return &pageBuilder{
		parent:  b,
		fullURL: fmt.Sprintf("%s?%s", b.baseURL, pageParams.Encode()),
	}
}

type pageBuilder struct {
	parent  *mockResponseBuilder
	fullURL string
}

func (p *pageBuilder) returns(items ...string) {
	itemHTMLs := ""
	for _, item := range items {
		itemHTMLs += fmt.Sprintf(`<li><div class="item"><div class="title_box"><strong class="name">%s</strong><span class="sub_text">Place</span></div><div class="thumb"><img src="http://example.com/thumb.jpg"></div></div></li>`, item)
	}
	rawHTML := fmt.Sprintf(`<ul>%s</ul>`, itemHTMLs)

	// JSON Wrapping
	htmlBytes, _ := json.Marshal(rawHTML)
	jsonResp := fmt.Sprintf(`{"html": %s}`, string(htmlBytes))

	p.parent.fetcher.SetResponse(p.fullURL, []byte(jsonResp))
}

func (p *pageBuilder) returnsEmpty() {
	p.parent.fetcher.SetResponse(p.fullURL, []byte(`{"html": ""}`))
}

func (p *pageBuilder) failsWith(err error) {
	p.parent.fetcher.SetError(p.fullURL, err)
}

// === Config Builder (Fluent Interface) ===

type configBuilder struct {
	appConfig *config.AppConfig
}

func newConfigBuilder() *configBuilder {
	return &configBuilder{
		appConfig: &config.AppConfig{
			Tasks: []config.TaskConfig{},
		},
	}
}

func (b *configBuilder) withTask(taskTaskID, commandTaskID string, data map[string]interface{}) *configBuilder {
	b.appConfig.Tasks = append(b.appConfig.Tasks, config.TaskConfig{
		ID: taskTaskID,
		Commands: []config.CommandConfig{
			{
				ID:   commandTaskID,
				Data: data,
			},
		},
	})
	return b
}

func (b *configBuilder) build() *config.AppConfig {
	return b.appConfig
}

func defaultTaskData() map[string]interface{} {
	return map[string]interface{}{
		"query": "뮤지컬",
		"filters": map[string]interface{}{
			"title": map[string]interface{}{
				"included_keywords": "",
				"excluded_keywords": "",
			},
			"place": map[string]interface{}{
				"included_keywords": "",
				"excluded_keywords": "",
			},
		},
		"max_pages":           50,
		"page_fetch_delay_ms": 10,
	}
}
