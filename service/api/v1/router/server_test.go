package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	t.Run("라우터 생성 - Debug 모드 활성화", func(t *testing.T) {
		e := New(Config{
			Debug:        true,
			AllowOrigins: []string{"*"},
		}, nil)

		assert.NotNil(t, e, "Echo 인스턴스가 생성되어야 합니다")
		assert.True(t, e.Debug, "Debug 모드가 활성화되어야 합니다")
		assert.True(t, e.HideBanner, "Banner가 숨겨져야 합니다")
	})

	t.Run("라우터 생성 - Debug 모드 비활성화", func(t *testing.T) {
		e := New(Config{
			Debug:        false,
			AllowOrigins: []string{"http://example.com"},
		}, nil)

		assert.NotNil(t, e, "Echo 인스턴스가 생성되어야 합니다")
		assert.False(t, e.Debug, "Debug 모드가 비활성화되어야 합니다")
		assert.True(t, e.HideBanner, "Banner가 숨겨져야 합니다")
	})

	t.Run("미들웨어 설정 확인", func(t *testing.T) {
		e := New(Config{
			Debug:        true,
			AllowOrigins: []string{"*"},
		}, nil)

		// Echo 인스턴스가 정상적으로 생성되었는지 확인
		assert.NotNil(t, e, "Echo 인스턴스가 생성되어야 합니다")

		// Logger가 설정되었는지 확인
		assert.NotNil(t, e.Logger, "Logger가 설정되어야 합니다")
	})

	t.Run("기본 라우트 테스트", func(t *testing.T) {
		e := New(Config{
			Debug:        true,
			AllowOrigins: []string{"*"},
		}, nil)

		// 테스트 라우트 추가
		e.GET("/test", func(c echo.Context) error {
			return c.String(200, "test")
		})

		// 라우트가 정상적으로 추가되었는지 확인
		routes := e.Routes()
		found := false
		for _, route := range routes {
			if route.Path == "/test" && route.Method == "GET" {
				found = true
				break
			}
		}

		assert.True(t, found, "테스트 라우트가 추가되어야 합니다")
	})
}

func TestRouterMiddlewares(t *testing.T) {
	t.Run("CORS 미들웨어 확인", func(t *testing.T) {
		e := New(Config{
			Debug:        true,
			AllowOrigins: []string{"*"},
		}, nil)

		// 테스트용 핸들러 등록
		e.GET("/test", func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		// HTTP 요청 생성
		req := httptest.NewRequest(http.MethodOptions, "/test", nil)
		req.Header.Set("Origin", "http://example.com")
		req.Header.Set("Access-Control-Request-Method", "GET")
		rec := httptest.NewRecorder()

		// 요청 실행
		e.ServeHTTP(rec, req)

		// CORS 헤더가 설정되었는지 확인
		assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("Recover 미들웨어 확인", func(t *testing.T) {
		e := New(Config{
			Debug:        true,
			AllowOrigins: []string{"*"},
		}, nil)

		// Panic이 발생해도 서버가 다운되지 않는지 테스트
		e.GET("/panic", func(c echo.Context) error {
			panic("test panic")
		})

		// HTTP 요청 생성
		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		rec := httptest.NewRecorder()

		// 요청 실행 (panic이 발생해도 서버가 다운되지 않아야 함)
		e.ServeHTTP(rec, req)

		// 500 에러가 반환되는지 확인 (panic이 recover되었다는 의미)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("RequestID 미들웨어 확인", func(t *testing.T) {
		e := New(Config{
			Debug:        true,
			AllowOrigins: []string{"*"},
		}, nil)

		e.GET("/test", func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		// X-Request-ID 헤더가 존재하는지 확인
		assert.NotEmpty(t, rec.Header().Get(echo.HeaderXRequestID))
	})
}
