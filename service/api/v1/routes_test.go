package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/config"
	apiauth "github.com/darkkaiser/notify-server/service/api/auth"
	"github.com/darkkaiser/notify-server/service/api/model/response"
	"github.com/darkkaiser/notify-server/service/api/v1/handler"
	"github.com/darkkaiser/notify-server/service/api/v1/model/request"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockNotificationSender는 테스트용 NotificationSender 구현체입니다.
type mockNotificationSender struct {
	notifyCalled bool
	lastMessage  string
}

func (m *mockNotificationSender) Notify(notifierID string, title string, message string, errorOccurred bool) bool {
	m.notifyCalled = true
	m.lastMessage = message
	return true
}

func (m *mockNotificationSender) NotifyToDefault(message string) bool {
	m.notifyCalled = true
	m.lastMessage = message
	return true
}

func (m *mockNotificationSender) NotifyWithErrorToDefault(message string) bool {
	m.notifyCalled = true
	m.lastMessage = message
	return true
}

// createTestAppConfig는 테스트용 AppConfig를 생성합니다.
func createTestAppConfig() *config.AppConfig {
	return &config.AppConfig{
		NotifyAPI: config.NotifyAPIConfig{
			Applications: []config.ApplicationConfig{
				{
					ID:                "test-app",
					Title:             "Test Application",
					Description:       "Test Description",
					DefaultNotifierID: "test-notifier",
					AppKey:            "test-app-key",
				},
			},
		},
	}
}

// TestSetupRoutes는 SetupRoutes 함수가 올바르게 v1 라우트를 설정하는지 테스트합니다.
func TestSetupRoutes(t *testing.T) {
	e := echo.New()

	appConfig := createTestAppConfig()
	applicationManager := apiauth.NewApplicationManager(appConfig)
	mockSender := &mockNotificationSender{}

	h := handler.NewHandler(applicationManager, mockSender)

	// v1 라우트 설정
	SetupRoutes(e, h)

	// 등록된 라우트 확인
	routes := e.Routes()

	// v1 API 라우트들이 등록되어야 함
	expectedRoutes := map[string]string{
		"/api/v1/notifications":  "POST",
		"/api/v1/notice/message": "POST", // 레거시 엔드포인트
	}

	for path, method := range expectedRoutes {
		found := false
		for _, route := range routes {
			if route.Path == path && route.Method == method {
				found = true
				break
			}
		}
		assert.True(t, found, "라우트 %s %s가 등록되지 않았습니다", method, path)
	}
}

// TestNotificationsEndpoint는 /api/v1/notifications 엔드포인트가 정상적으로 동작하는지 테스트합니다.
func TestNotificationsEndpoint(t *testing.T) {
	e := echo.New()

	appConfig := createTestAppConfig()
	applicationManager := apiauth.NewApplicationManager(appConfig)
	mockSender := &mockNotificationSender{}

	h := handler.NewHandler(applicationManager, mockSender)
	SetupRoutes(e, h)

	// 요청 본문 생성
	reqBody := request.NotificationRequest{
		ApplicationID: "test-app",
		Message:       "테스트 메시지",
		ErrorOccurred: false,
	}
	bodyBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)

	// HTTP 요청 생성
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications?app_key=test-app-key", bytes.NewReader(bodyBytes))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// 응답 검증
	assert.Equal(t, http.StatusOK, rec.Code)

	var successResp response.SuccessResponse
	err = json.Unmarshal(rec.Body.Bytes(), &successResp)
	require.NoError(t, err)

	assert.Equal(t, 0, successResp.ResultCode)

	// NotificationSender가 호출되었는지 확인
	assert.True(t, mockSender.notifyCalled, "NotificationSender.Notify가 호출되지 않았습니다")
	assert.Equal(t, "테스트 메시지", mockSender.lastMessage)
}

// TestLegacyNoticeMessageEndpoint는 레거시 /api/v1/notice/message 엔드포인트가 동작하는지 테스트합니다.
func TestLegacyNoticeMessageEndpoint(t *testing.T) {
	e := echo.New()

	appConfig := createTestAppConfig()
	applicationManager := apiauth.NewApplicationManager(appConfig)
	mockSender := &mockNotificationSender{}

	h := handler.NewHandler(applicationManager, mockSender)
	SetupRoutes(e, h)

	// 요청 본문 생성
	reqBody := request.NotificationRequest{
		ApplicationID: "test-app",
		Message:       "레거시 엔드포인트 테스트",
		ErrorOccurred: false,
	}
	bodyBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)

	// HTTP 요청 생성 (레거시 엔드포인트)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notice/message?app_key=test-app-key", bytes.NewReader(bodyBytes))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// 응답 검증
	assert.Equal(t, http.StatusOK, rec.Code)

	var successResp response.SuccessResponse
	err = json.Unmarshal(rec.Body.Bytes(), &successResp)
	require.NoError(t, err)

	assert.Equal(t, 0, successResp.ResultCode)

	// NotificationSender가 호출되었는지 확인
	assert.True(t, mockSender.notifyCalled)
	assert.Equal(t, "레거시 엔드포인트 테스트", mockSender.lastMessage)
}

// TestNotificationsEndpoint_MissingAppKey는 app_key가 없을 때 400 에러를 반환하는지 테스트합니다.
func TestNotificationsEndpoint_MissingAppKey(t *testing.T) {
	e := echo.New()

	appConfig := createTestAppConfig()
	applicationManager := apiauth.NewApplicationManager(appConfig)
	mockSender := &mockNotificationSender{}

	h := handler.NewHandler(applicationManager, mockSender)
	SetupRoutes(e, h)

	// 요청 본문 생성
	reqBody := request.NotificationRequest{
		ApplicationID: "test-app",
		Message:       "테스트 메시지",
		ErrorOccurred: false,
	}
	bodyBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)

	// app_key 없이 요청
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications", bytes.NewReader(bodyBytes))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// 400 에러 응답 검증
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errorResp response.ErrorResponse
	err = json.Unmarshal(rec.Body.Bytes(), &errorResp)
	require.NoError(t, err)

	assert.Contains(t, errorResp.Message, "app_key")
}

// TestNotificationsEndpoint_InvalidAppKey는 잘못된 app_key로 요청 시 401 에러를 반환하는지 테스트합니다.
func TestNotificationsEndpoint_InvalidAppKey(t *testing.T) {
	e := echo.New()

	appConfig := createTestAppConfig()
	applicationManager := apiauth.NewApplicationManager(appConfig)
	mockSender := &mockNotificationSender{}

	h := handler.NewHandler(applicationManager, mockSender)
	SetupRoutes(e, h)

	// 요청 본문 생성
	reqBody := request.NotificationRequest{
		ApplicationID: "test-app",
		Message:       "테스트 메시지",
		ErrorOccurred: false,
	}
	bodyBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)

	// 잘못된 app_key로 요청
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications?app_key=wrong-key", bytes.NewReader(bodyBytes))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// 401 에러 응답 검증
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var errorResp response.ErrorResponse
	err = json.Unmarshal(rec.Body.Bytes(), &errorResp)
	require.NoError(t, err)

	assert.NotEmpty(t, errorResp.Message)
}

// TestNotificationsEndpoint_InvalidJSON는 잘못된 JSON 요청 시 400 에러를 반환하는지 테스트합니다.
func TestNotificationsEndpoint_InvalidJSON(t *testing.T) {
	e := echo.New()

	appConfig := createTestAppConfig()
	applicationManager := apiauth.NewApplicationManager(appConfig)
	mockSender := &mockNotificationSender{}

	h := handler.NewHandler(applicationManager, mockSender)
	SetupRoutes(e, h)

	// 잘못된 JSON
	invalidJSON := []byte(`{"invalid json`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications?app_key=test-app-key", bytes.NewReader(invalidJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// 400 에러 응답 검증
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestBothEndpointsUseSameHandler는 두 엔드포인트가 동일한 핸들러를 사용하는지 확인합니다.
func TestBothEndpointsUseSameHandler(t *testing.T) {
	e := echo.New()

	appConfig := createTestAppConfig()
	applicationManager := apiauth.NewApplicationManager(appConfig)
	mockSender := &mockNotificationSender{}

	h := handler.NewHandler(applicationManager, mockSender)
	SetupRoutes(e, h)

	routes := e.Routes()

	var notificationsHandler string
	var noticeMessageHandler string

	for _, route := range routes {
		if route.Path == "/api/v1/notifications" && route.Method == "POST" {
			notificationsHandler = route.Name
		}
		if route.Path == "/api/v1/notice/message" && route.Method == "POST" {
			noticeMessageHandler = route.Name
		}
	}

	// 두 엔드포인트가 동일한 핸들러를 사용해야 함
	assert.Equal(t, notificationsHandler, noticeMessageHandler,
		"두 엔드포인트는 동일한 핸들러(PublishNotificationHandler)를 사용해야 합니다")
}
