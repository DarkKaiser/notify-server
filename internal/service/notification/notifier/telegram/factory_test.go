package telegram

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	contractmocks "github.com/darkkaiser/notify-server/internal/service/contract/mocks"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Helper Mocks & Types
// =============================================================================

// mockRoundTripper intercepts HTTP requests for testing without external network.
type mockRoundTripper struct {
	roundTripFunc func(*http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}

// =============================================================================
// Bot Client Tests
// =============================================================================

func TestDefaultBotClient_GetSelf(t *testing.T) {
	t.Parallel()

	// given
	expectedUser := tgbotapi.User{
		ID:        123456,
		UserName:  "test_bot",
		FirstName: "Test",
		LastName:  "Bot",
	}
	mockBotAPI := &tgbotapi.BotAPI{
		Self: expectedUser,
	}

	client := &tgClient{BotAPI: mockBotAPI}

	// when
	user := client.GetSelf()

	// then
	assert.Equal(t, expectedUser, user)
}

// =============================================================================
// Factory Creator Tests (NewCreator, BuildCreator)
// =============================================================================

func TestNewCreator(t *testing.T) {
	t.Parallel()

	// when
	creator := NewCreator()

	// then
	assert.NotNil(t, creator, "NewCreator는 nil이 아닌 함수를 반환해야 합니다")
}

func TestBuildCreator(t *testing.T) {
	t.Parallel()

	mockExecutor := &contractmocks.MockTaskExecutor{}

	tests := []struct {
		name        string
		config      *config.AppConfig
		mockCtor    notifierCtor
		expectedLen int
		expectErr   bool
	}{
		{
			name: "성공: 모든 Notifier가 정상적으로 생성되어야 한다",
			config: &config.AppConfig{
				Notifier: config.NotifierConfig{
					Telegrams: []config.TelegramConfig{
						{ID: "telegram-1", BotToken: "token-1", ChatID: 1001},
						{ID: "telegram-2", BotToken: "token-2", ChatID: 1002},
					},
				},
			},
			mockCtor: func(id contract.NotifierID, executor contract.TaskExecutor, args creationArgs) (notifier.Notifier, error) {
				assert.Equal(t, mockExecutor, executor)
				return &telegramNotifier{}, nil
			},
			expectedLen: 2,
			expectErr:   false,
		},
		{
			name: "실패: 생성자 중 하나라도 실패하면 에러를 반환해야 한다",
			config: &config.AppConfig{
				Notifier: config.NotifierConfig{
					Telegrams: []config.TelegramConfig{
						{ID: "t1"}, {ID: "t2"},
					},
				},
			},
			mockCtor: func(id contract.NotifierID, executor contract.TaskExecutor, args creationArgs) (notifier.Notifier, error) {
				if id == "t2" {
					return nil, errors.New("creation failed")
				}
				return &telegramNotifier{}, nil
			},
			expectedLen: 0,
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// when
			creator := buildCreator(tt.mockCtor)
			notifiers, err := creator(tt.config, mockExecutor)

			// then
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, notifiers)
			} else {
				assert.NoError(t, err)
				assert.Len(t, notifiers, tt.expectedLen)
			}
		})
	}
}

// =============================================================================
// Notifier Construction Tests (newNotifier, newNotifierWithClient)
// =============================================================================

func TestNewNotifier_Integration(t *testing.T) {
	// 이제 HTTPClient 주입이 가능하므로 통합 테스트를 활성화합니다.
	t.Parallel()

	tests := []struct {
		name          string
		token         string
		debug         bool
		roundTripFunc func(*http.Request) (*http.Response, error)
		expectErr     bool
		errContains   string
	}{
		{
			name:  "성공: 유효한 토큰으로 봇 API 클라이언트 초기화 성공",
			token: "123:valid-token",
			debug: true,
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				// tgbotapi.NewBotAPI calls getMe to validate token
				if strings.Contains(req.URL.Path, "/getMe") {
					return &http.Response{
						StatusCode: 200,
						Body: io.NopCloser(bytes.NewBufferString(`{
							"ok": true,
							"result": {"id": 123, "first_name": "TestBot", "username": "test_bot"}
						}`)),
					}, nil
				}
				return nil, errors.New("unexpected request: " + req.URL.Path)
			},
			expectErr: false,
		},
		{
			name:  "실패: API 호출 실패 (네트워크 에러)",
			token: "123:network-error",
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("connection refused")
			},
			expectErr:   true,
			errContains: "connection refused",
		},
		{
			name:  "실패: 유효하지 않은 응답 (401 Unauthorized)",
			token: "123:invalid-token",
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 401,
					Body: io.NopCloser(bytes.NewBufferString(`{
						"ok": false,
						"error_code": 401,
						"description": "Unauthorized"
					}`)),
				}, nil
			},
			expectErr:   true, // NewBotAPI checks info, so it will fail
			errContains: "Unauthorized",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			mockTransport := &mockRoundTripper{roundTripFunc: tt.roundTripFunc}
			httpClient := &http.Client{Transport: mockTransport}

			args := creationArgs{
				BotToken:   tt.token,
				ChatID:     12345,
				AppConfig:  &config.AppConfig{Debug: tt.debug},
				HTTPClient: httpClient, // Inject Mock Client
			}

			// when
			n, err := newNotifier("test-notifier", &contractmocks.MockTaskExecutor{}, args)

			// then
			if tt.expectErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, n)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, n)
			}
		})
	}
}

func TestNewNotifierWithClient_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		appConfig         *config.AppConfig
		validateStructure func(*testing.T, *telegramNotifier)
	}{
		{
			name:      "기본 설정: 필수 필드 및 도움말 명령어가 초기화되어야 한다",
			appConfig: &config.AppConfig{},
			validateStructure: func(t *testing.T, n *telegramNotifier) {
				assert.NotNil(t, n.rateLimiter, "Rate Limiter가 초기화되어야 함")
				assert.NotNil(t, n.commandSemaphore, "세마포어가 초기화되어야 함")
				assert.Equal(t, defaultRetryDelay, n.retryDelay)
				assert.Equal(t, notifierBufferSize, cap(n.NotificationC()))

				// 기본 도움말 명령어 확인
				assert.Len(t, n.botCommands, 1)
				assert.Equal(t, "help", n.botCommands[0].name)
				assert.Contains(t, n.botCommandsByName, "help")
			},
		},
		{
			name: "명령어 등록: Task 명령어가 올바르게 인덱싱되어야 한다",
			appConfig: &config.AppConfig{
				Tasks: []config.TaskConfig{
					{
						ID:    "Deploy",
						Title: "Deployment",
						Commands: []config.CommandConfig{
							{
								ID:          "Start",
								Title:       "Start Deploy",
								Description: "Starts deployment",
								Notifier:    config.CommandNotifierConfig{Usable: true},
							},
						},
					},
				},
			},
			validateStructure: func(t *testing.T, n *telegramNotifier) {
				expectedCmdName := "deploy_start" // snake_case 변환 확인

				// 1. 리스트 확인 (Task Cmd + Help)
				assert.Len(t, n.botCommands, 2)

				// 2. Name Map 확인
				cmd, exists := n.botCommandsByName[expectedCmdName]
				require.True(t, exists, "명령어가 이름으로 검색되어야 함")
				assert.Equal(t, "Deployment > Start Deploy", cmd.title)

				// 3. Task Map 확인
				taskCmds, taskExists := n.botCommandsByTask["Deploy"]
				require.True(t, taskExists, "TaskID로 맵이 생성되어야 함")
				cmdFromTask, cmdExists := taskCmds["Start"]
				require.True(t, cmdExists, "CommandID로 명령어를 찾을 수 있어야 함")
				assert.Equal(t, expectedCmdName, cmdFromTask.name)
			},
		},
		{
			name: "사용 불가 명령어: Usable이 false인 경우 등록되지 않아야 한다",
			appConfig: &config.AppConfig{
				Tasks: []config.TaskConfig{
					{
						ID: "Backup",
						Commands: []config.CommandConfig{
							{ID: "Run", Notifier: config.CommandNotifierConfig{Usable: false}},
						},
					},
				},
			},
			validateStructure: func(t *testing.T, n *telegramNotifier) {
				assert.Len(t, n.botCommands, 1, "도움말 명령어만 존재해야 함")
				_, exists := n.botCommandsByName["backup_run"]
				assert.False(t, exists)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			mockBot := &MockTelegramBot{}
			mockExecutor := &contractmocks.MockTaskExecutor{}
			args := creationArgs{
				BotToken:  "test-token",
				ChatID:    12345,
				AppConfig: tt.appConfig,
			}

			// when
			n, err := newNotifierWithClient("test-notifier", mockBot, mockExecutor, args)

			// then
			require.NoError(t, err)
			notifier := n.(*telegramNotifier)

			// 공통 검증: 봇 클라이언트 주입
			assert.Equal(t, mockBot, notifier.client)
			assert.Equal(t, int64(12345), notifier.chatID)

			// 케이스별 상세 검증
			if tt.validateStructure != nil {
				tt.validateStructure(t, notifier)
			}
		})
	}
}

func TestNewNotifierWithClient_Failure(t *testing.T) {
	t.Parallel()

	// Refined collision case
	collisionConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{ID: "foo_bar", Commands: []config.CommandConfig{{ID: "baz", Notifier: config.CommandNotifierConfig{Usable: true}}}},
			{ID: "foo", Commands: []config.CommandConfig{{ID: "bar_baz", Notifier: config.CommandNotifierConfig{Usable: true}}}},
		},
	}

	t.Run("명령어 이름 충돌 발생 시 에러 반환", func(t *testing.T) {
		t.Parallel()
		mockBot := &MockTelegramBot{}
		args := creationArgs{BotToken: "t", ChatID: 1, AppConfig: collisionConfig}

		n, err := newNotifierWithClient("id", mockBot, &contractmocks.MockTaskExecutor{}, args)
		require.Error(t, err)
		assert.Nil(t, n)
		assert.Contains(t, err.Error(), "/foo_bar_baz")
	})

	t.Run("필수 ID 누락 시 에러 반환", func(t *testing.T) {
		t.Parallel()
		mockBot := &MockTelegramBot{}
		invalidConfig := &config.AppConfig{
			Tasks: []config.TaskConfig{{ID: "", Commands: []config.CommandConfig{{ID: "cmd", Notifier: config.CommandNotifierConfig{Usable: true}}}}},
		}
		args := creationArgs{BotToken: "t", ChatID: 1, AppConfig: invalidConfig}

		n, err := newNotifierWithClient("id", mockBot, &contractmocks.MockTaskExecutor{}, args)
		require.Error(t, err)
		assert.Nil(t, n)
		assert.Contains(t, err.Error(), "필수입니다")
	})
}
