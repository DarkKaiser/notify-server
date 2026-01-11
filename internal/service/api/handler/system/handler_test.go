package system

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/internal/pkg/version"
	"github.com/darkkaiser/notify-server/internal/service/api/model/system"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LogEntry 로그 검증을 위한 구조체
type LogEntry struct {
	Level    string `json:"level"`
	Message  string `json:"msg"`
	Endpoint string `json:"endpoint"`
	Method   string `json:"method"`
	RemoteIP string `json:"remote_ip"`
}

// TestHealthCheckHandler_Table는 헬스체크 핸들러의 모든 동작을 검증합니다.
//
// 주요 검증 항목:
//   - 정상/비정상 상태에 따른 올바른 응답 코드 및 상태값
//   - 의존성 서비스 상태 반영 여부
//   - 로그 출력 (메시지, 엔드포인트, 메서드, IP)
func TestHealthCheckHandler_Table(t *testing.T) {
	// 로거 캡처 설정
	buf := new(bytes.Buffer)
	setupTestLogger(buf)
	defer restoreLogger()

	tests := []struct {
		name              string
		mockService       *mocks.MockNotificationSender
		useNilService     bool
		expectedStatus    string
		expectedDepStatus string
	}{
		{
			name:              "정상 상태_모든 서비스 정상",
			mockService:       &mocks.MockNotificationSender{},
			useNilService:     false,
			expectedStatus:    "healthy",
			expectedDepStatus: "healthy",
		},
		{
			name:              "비정상 상태_서비스 초기화 안됨",
			mockService:       nil,
			useNilService:     true,
			expectedStatus:    "unhealthy",
			expectedDepStatus: "unhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 로그 버퍼 초기화
			buf.Reset()

			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			req.Header.Set(echo.HeaderXRealIP, "10.0.0.1") // Remote IP 테스트용
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			var h *Handler
			if tt.useNilService {
				h = NewHandler(nil, version.Info{})
			} else {
				h = NewHandler(tt.mockService, version.Info{})
			}

			// 핸들러 실행 및 에러 없음 검증
			if assert.NoError(t, h.HealthCheckHandler(c)) {
				// 1. HTTP 상태 코드 검증
				assert.Equal(t, http.StatusOK, rec.Code)

				// 2. JSON 응답 본문 파싱 및 검증
				var healthResp system.HealthResponse
				err := json.Unmarshal(rec.Body.Bytes(), &healthResp)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedStatus, healthResp.Status)
				assert.Equal(t, tt.expectedDepStatus, healthResp.Dependencies["notification_service"].Status)

				if tt.expectedStatus == "healthy" {
					assert.GreaterOrEqual(t, healthResp.Uptime, int64(0), "가동 시간은 0보다 커야 합니다")
				}

				// 3. 로그 검증
				var logEntry LogEntry
				err = json.Unmarshal(buf.Bytes(), &logEntry)
				require.NoError(t, err, "로그 파싱 실패")

				assert.Equal(t, "debug", logEntry.Level)
				assert.Equal(t, "헬스체크 조회", logEntry.Message)
				assert.Equal(t, "/health", logEntry.Endpoint)
				assert.Equal(t, "GET", logEntry.Method)
				assert.Equal(t, "10.0.0.1", logEntry.RemoteIP)
			}
		})
	}
}

// TestVersionHandler_Table는 버전 핸들러의 동작을 검증합니다.
//
// 주요 검증 항목:
//   - 빌드 정보의 올바른 반환
//   - 로그 출력 (메시지, 엔드포인트, 메서드, IP)
func TestVersionHandler_Table(t *testing.T) {
	// 로거 캡처 설정
	buf := new(bytes.Buffer)
	setupTestLogger(buf)
	defer restoreLogger()

	buildInfo := version.Info{
		Version:     "1.0.0",
		BuildDate:   "2024-01-01",
		BuildNumber: "100",
	}

	tests := []struct {
		name      string
		buildInfo version.Info
	}{
		{
			name:      "버전 정보 조회_정상 반환",
			buildInfo: buildInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()

			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/version", nil)
			req.Header.Set(echo.HeaderXRealIP, "192.168.0.5") // Remote IP 테스트용
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			h := NewHandler(&mocks.MockNotificationSender{}, tt.buildInfo)

			// 핸들러 실행 및 에러 없음 검증
			if assert.NoError(t, h.VersionHandler(c)) {
				// 1. HTTP 상태 코드 검증
				assert.Equal(t, http.StatusOK, rec.Code)

				// 2. JSON 응답 본문 파싱 및 검증
				var versionResp system.VersionResponse
				err := json.Unmarshal(rec.Body.Bytes(), &versionResp)
				require.NoError(t, err)

				assert.Equal(t, tt.buildInfo.Version, versionResp.Version)
				assert.Equal(t, tt.buildInfo.BuildDate, versionResp.BuildDate)
				assert.Equal(t, tt.buildInfo.BuildNumber, versionResp.BuildNumber)
				assert.NotEmpty(t, versionResp.GoVersion, "Go 버전 정보가 포함되어야 합니다")

				// 3. 로그 검증
				var logEntry LogEntry
				err = json.Unmarshal(buf.Bytes(), &logEntry)
				require.NoError(t, err, "로그 파싱 실패")

				assert.Equal(t, "debug", logEntry.Level)
				assert.Equal(t, "버전 정보 조회", logEntry.Message)
				assert.Equal(t, "/version", logEntry.Endpoint)
				assert.Equal(t, "GET", logEntry.Method)
				assert.Equal(t, "192.168.0.5", logEntry.RemoteIP)
			}
		})
	}
}

// setupTestLogger는 테스트를 위해 로거 출력을 버퍼로 변경합니다.
func setupTestLogger(buf *bytes.Buffer) {
	applog.SetOutput(buf)
	applog.SetFormatter(&applog.JSONFormatter{})
	applog.SetLevel(applog.DebugLevel) // Debug 레벨 활성화
}

// restoreLogger는 로거 출력을 표준 출력으로 복구합니다.
func restoreLogger() {
	applog.SetOutput(applog.StandardLogger().Out)
}
