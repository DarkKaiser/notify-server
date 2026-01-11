package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/api/model/domain"
	"github.com/darkkaiser/notify-server/internal/service/api/model/response"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Utils & Helpers
// =============================================================================

// LogEntry 로그 검증을 위한 구조체
type LogEntry struct {
	Level         string `json:"level"`
	Message       string `json:"msg"`
	ApplicationID string `json:"application_id"`
	AppTitle      string `json:"app_title"`
}

// createTestAppConfig 테스트용 AppConfig를 생성합니다.
func createTestAppConfig(apps ...config.ApplicationConfig) *config.AppConfig {
	return &config.AppConfig{
		NotifyAPI: config.NotifyAPIConfig{
			Applications: apps,
		},
	}
}

// setupTestLogger는 테스트를 위해 로거 출력을 버퍼로 변경합니다.
func setupTestLogger(buf *bytes.Buffer) {
	applog.SetOutput(buf)
	applog.SetFormatter(&applog.JSONFormatter{})
	applog.SetLevel(applog.DebugLevel)
}

// restoreLogger는 로거 출력을 표준 출력으로 복구합니다.
func restoreLogger() {
	applog.SetOutput(applog.StandardLogger().Out)
}

// =============================================================================
// Constructor Tests
// =============================================================================

func TestNewAuthenticator_Table(t *testing.T) {
	tests := []struct {
		name          string
		appConfig     *config.AppConfig
		expectedCount int
		verifyApps    func(*testing.T, map[string]*domain.Application)
	}{
		{
			name: "단일 애플리케이션 생성",
			appConfig: createTestAppConfig(
				config.ApplicationConfig{
					ID:                "test-app",
					Title:             "Test Application",
					Description:       "Test Description",
					DefaultNotifierID: "test-notifier",
					AppKey:            "test-key",
				},
			),
			expectedCount: 1,
			verifyApps: func(t *testing.T, apps map[string]*domain.Application) {
				app, ok := apps["test-app"]
				assert.True(t, ok)
				assert.Equal(t, "test-app", app.ID)
				assert.Equal(t, "Test Application", app.Title)
				assert.Equal(t, "Test Description", app.Description)
				assert.Equal(t, "test-notifier", app.DefaultNotifierID)
				// 중요: AppKey는 보안을 위해 Application 구조체에 저장되지 않아야 함
				// Application 구조체에 AppKey 필드가 없으므로 컴파일 레벨에서 보장되지만,
				// 의도적으로 주석을 통해 확인.
			},
		},
		{
			name: "다중 애플리케이션 생성",
			appConfig: createTestAppConfig(
				config.ApplicationConfig{ID: "app1", AppKey: "key1"},
				config.ApplicationConfig{ID: "app2", AppKey: "key2"},
				config.ApplicationConfig{ID: "app3", AppKey: "key3"},
			),
			expectedCount: 3,
			verifyApps: func(t *testing.T, apps map[string]*domain.Application) {
				assert.Contains(t, apps, "app1")
				assert.Contains(t, apps, "app2")
				assert.Contains(t, apps, "app3")
			},
		},
		{
			name:          "애플리케이션 없음",
			appConfig:     createTestAppConfig(),
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authenticator := NewAuthenticator(tt.appConfig)

			assert.NotNil(t, authenticator)
			assert.NotNil(t, authenticator.applications)
			assert.Equal(t, tt.expectedCount, len(authenticator.applications))

			if tt.verifyApps != nil {
				tt.verifyApps(t, authenticator.applications)
			}
		})
	}
}

// =============================================================================
// Authenticate Tests
// =============================================================================

// TestAuthenticator_Authenticate_Table는 인증 로직을 상세 검증합니다.
//
// 주요 검증 항목:
//   - 정상 인증 시 Application 객체 반환
//   - 등록되지 않은 App ID 처리 및 에러 메시지
//   - App Key 불일치 시 처리, 에러 메시지, 그리고 보안 로그 기록
func TestAuthenticator_Authenticate_Table(t *testing.T) {
	// 로거 캡처 설정
	buf := new(bytes.Buffer)
	setupTestLogger(buf)
	defer restoreLogger()

	appConfig := createTestAppConfig(
		config.ApplicationConfig{
			ID:     "test-app",
			Title:  "테스트 앱",
			AppKey: "valid-key",
		},
	)
	authenticator := NewAuthenticator(appConfig)

	tests := []struct {
		name          string
		appID         string
		appKey        string
		expectedError bool
		checkError    func(*testing.T, error)
		checkLog      func(*testing.T, *bytes.Buffer) // 로그 검증 로직
		checkApp      func(*testing.T, *domain.Application)
	}{
		{
			name:          "인증 성공_정상 키 입력",
			appID:         "test-app",
			appKey:        "valid-key",
			expectedError: false,
			checkApp: func(t *testing.T, app *domain.Application) {
				assert.Equal(t, "test-app", app.ID)
				assert.Equal(t, "테스트 앱", app.Title)
			},
		},
		{
			name:          "인증 실패_등록되지 않은 ID",
			appID:         "unknown-app",
			appKey:        "valid-key",
			expectedError: true,
			checkError: func(t *testing.T, err error) {
				httpErr, ok := err.(*echo.HTTPError)
				require.True(t, ok, "에러는 *echo.HTTPError 타입이어야 함")
				assert.Equal(t, 401, httpErr.Code)

				// 에러 메시지 검증: "등록되지 않은 application_id입니다 (ID: %s)"
				errResp, ok := httpErr.Message.(response.ErrorResponse)
				require.True(t, ok)
				assert.Equal(t, fmt.Sprintf("등록되지 않은 application_id입니다 (ID: %s)", "unknown-app"), errResp.Message)
			},
		},
		{
			name:          "인증 실패_Key 불일치_로그 기록",
			appID:         "test-app",
			appKey:        "invalid-key",
			expectedError: true,
			checkError: func(t *testing.T, err error) {
				httpErr, ok := err.(*echo.HTTPError)
				require.True(t, ok)
				assert.Equal(t, 401, httpErr.Code)

				// 에러 메시지 검증: "app_key가 유효하지 않습니다 (application_id: %s)"
				errResp, ok := httpErr.Message.(response.ErrorResponse)
				require.True(t, ok)
				assert.Equal(t, fmt.Sprintf("app_key가 유효하지 않습니다 (application_id: %s)", "test-app"), errResp.Message)
			},
			checkLog: func(t *testing.T, logBuf *bytes.Buffer) {
				var logEntry LogEntry
				err := json.Unmarshal(logBuf.Bytes(), &logEntry)
				require.NoError(t, err, "로그 파싱 실패")

				assert.Equal(t, "warning", logEntry.Level)
				assert.Equal(t, "인증 실패: App Key 불일치", logEntry.Message)
				assert.Equal(t, "test-app", logEntry.ApplicationID)
				assert.Equal(t, "테스트 앱", logEntry.AppTitle)
			},
		},
		{
			name:          "인증 실패_빈 Key",
			appID:         "test-app",
			appKey:        "",
			expectedError: true,
			checkError: func(t *testing.T, err error) {
				httpErr, ok := err.(*echo.HTTPError)
				require.True(t, ok)
				assert.Equal(t, 401, httpErr.Code)
			},
			checkLog: func(t *testing.T, logBuf *bytes.Buffer) {
				// 빈 키도 불일치로 간주되므로 로그가 남아야 함
				assert.Contains(t, logBuf.String(), "App Key 불일치")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset() // 로그 버퍼 초기화

			app, err := authenticator.Authenticate(tt.appID, tt.appKey)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, app)
				if tt.checkError != nil {
					tt.checkError(t, err)
				}
				if tt.checkLog != nil {
					tt.checkLog(t, buf)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, app)
				if tt.checkApp != nil {
					tt.checkApp(t, app)
				}
				// 성공 시에는 보안 경고 로그가 없어야 함
				assert.Empty(t, buf.String(), "성공 시에는 로그가 없어야 함")
			}
		})
	}
}

// =============================================================================
// Concurrency Tests
// =============================================================================

// TestAuthenticator_ConcurrentAccess는 동시성 안전성을 검증합니다.
func TestAuthenticator_ConcurrentAccess(t *testing.T) {
	appConfig := createTestAppConfig(
		config.ApplicationConfig{ID: "app1", AppKey: "key1"},
		config.ApplicationConfig{ID: "app2", AppKey: "key2"},
	)
	authenticator := NewAuthenticator(appConfig)

	// 동시에 고루틴에서 Authenticate 호출
	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines * 2) // app1과 app2 동시 호출

	// 에러 발생 여부 기록 (성공해야 하는 케이스들만 수행)
	errCh := make(chan error, goroutines*2)

	// app1 호출 그룹
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			app, err := authenticator.Authenticate("app1", "key1")
			if err != nil {
				errCh <- fmt.Errorf("app1 auth failed: %w", err)
				return
			}
			if app.ID != "app1" {
				errCh <- fmt.Errorf("app1 returned wrong app: %s", app.ID)
			}
		}()
	}

	// app2 호출 그룹 (실패 케이스 포함하여 동시성 부하 증가)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			// 절반은 성공, 절반은 실패 요청을 보내 더 복잡한 동시성 상황 연출
			if idx%2 == 0 {
				app, err := authenticator.Authenticate("app2", "key2")
				if err != nil {
					errCh <- fmt.Errorf("app2 auth failed: %w", err)
				}
				if app != nil && app.ID != "app2" {
					errCh <- fmt.Errorf("app2 returned wrong app: %s", app.ID)
				}
			} else {
				// 실패 요청 (동시 읽기 상황에서 RLock 경합 테스트)
				_, err := authenticator.Authenticate("app2", "wrong-key")
				if err == nil {
					errCh <- fmt.Errorf("expected error for wrong key but got nil")
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	// 에러 확인
	for err := range errCh {
		t.Errorf("Concurrent test error: %v", err)
	}
}
