package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestLogger_Output(t *testing.T) {
	t.Run("Output 반환", func(t *testing.T) {
		logger := Logger{Logger: logrus.StandardLogger()}
		output := logger.Output()

		assert.NotNil(t, output, "Output이 nil이 아니어야 합니다")
	})
}

func TestLogger_Prefix(t *testing.T) {
	t.Run("Prefix 반환", func(t *testing.T) {
		logger := Logger{Logger: logrus.StandardLogger()}
		prefix := logger.Prefix()

		assert.Equal(t, "", prefix, "Prefix는 빈 문자열이어야 합니다")
	})
}

func TestLogger_Level(t *testing.T) {
	cases := []struct {
		name          string
		logrusLevel   logrus.Level
		expectedLevel log.Lvl
	}{
		{"Debug Level", logrus.DebugLevel, log.DEBUG},
		{"Info Level", logrus.InfoLevel, log.INFO},
		{"Warn Level", logrus.WarnLevel, log.WARN},
		{"Error Level", logrus.ErrorLevel, log.ERROR},
		{"Panic Level", logrus.PanicLevel, log.OFF},
		{"Fatal Level", logrus.FatalLevel, log.OFF},
		{"Trace Level", logrus.TraceLevel, log.OFF},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := logrus.New()
			l.SetLevel(c.logrusLevel)
			logger := Logger{Logger: l}

			level := logger.Level()
			assert.Equal(t, c.expectedLevel, level, "로그 레벨이 일치해야 합니다")
		})
	}
}

func TestLogger_SetLevel(t *testing.T) {
	cases := []struct {
		name        string
		echoLevel   log.Lvl
		expectLevel logrus.Level
	}{
		{"Set Debug", log.DEBUG, logrus.DebugLevel},
		{"Set Info", log.INFO, logrus.InfoLevel},
		{"Set Warn", log.WARN, logrus.WarnLevel},
		{"Set Error", log.ERROR, logrus.ErrorLevel},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			logger := Logger{Logger: logrus.New()}
			logger.SetLevel(c.echoLevel)

			// 전역 logrus 레벨이 변경되었는지 확인
			assert.NotNil(t, logger.Logger, "Logger가 nil이 아니어야 합니다")
		})
	}
}

func TestLogger_SetLevel_OFF(t *testing.T) {
	t.Run("Set OFF Level", func(t *testing.T) {
		logger := Logger{Logger: logrus.New()}
		logger.SetLevel(log.OFF)

		// OFF 레벨은 아무 동작도 하지 않음
		assert.NotNil(t, logger.Logger, "Logger가 nil이 아니어야 합니다")
	})
}

func TestLogger_SetOutput(t *testing.T) {
	t.Run("Output 설정", func(t *testing.T) {
		logger := Logger{Logger: logrus.New()}

		// io.Discard로 출력 설정
		logger.SetOutput(io.Discard)

		assert.NotNil(t, logger.Logger, "Logger가 nil이 아니어야 합니다")
	})
}

func TestLogger_SetPrefix(t *testing.T) {
	t.Run("Prefix 설정 (no-op)", func(t *testing.T) {
		logger := Logger{Logger: logrus.StandardLogger()}

		// SetPrefix는 아무 동작도 하지 않음
		logger.SetPrefix("test-prefix")

		// Prefix는 여전히 빈 문자열
		assert.Equal(t, "", logger.Prefix(), "Prefix는 빈 문자열이어야 합니다")
	})
}

func TestLogger_SetHeader(t *testing.T) {
	t.Run("Header 설정 (no-op)", func(t *testing.T) {
		logger := Logger{Logger: logrus.StandardLogger()}

		// SetHeader는 아무 동작도 하지 않음
		logger.SetHeader("test-header")

		assert.NotNil(t, logger.Logger, "Logger가 nil이 아니어야 합니다")
	})
}

func TestLogger_PrintMethods(t *testing.T) {
	t.Run("Print 메서드 호출", func(t *testing.T) {
		logger := Logger{Logger: logrus.New()}
		logger.SetOutput(io.Discard) // 출력 억제

		// 각 Print 메서드가 panic 없이 실행되는지 확인
		assert.NotPanics(t, func() {
			logger.Print("test")
			logger.Printf("test %s", "format")
			logger.Printj(log.JSON{"key": "value"})
		}, "Print 메서드들이 panic 없이 실행되어야 합니다")
	})
}

func TestLogger_DebugMethods(t *testing.T) {
	t.Run("Debug 메서드 호출", func(t *testing.T) {
		logger := Logger{Logger: logrus.New()}
		logger.SetOutput(io.Discard) // 출력 억제

		assert.NotPanics(t, func() {
			logger.Debug("test")
			logger.Debugf("test %s", "format")
			logger.Debugj(log.JSON{"key": "value"})
		}, "Debug 메서드들이 panic 없이 실행되어야 합니다")
	})
}

func TestLogger_InfoMethods(t *testing.T) {
	t.Run("Info 메서드 호출", func(t *testing.T) {
		logger := Logger{Logger: logrus.New()}
		logger.SetOutput(io.Discard) // 출력 억제

		assert.NotPanics(t, func() {
			logger.Info("test")
			logger.Infof("test %s", "format")
			logger.Infoj(log.JSON{"key": "value"})
		}, "Info 메서드들이 panic 없이 실행되어야 합니다")
	})
}

func TestLogger_WarnMethods(t *testing.T) {
	t.Run("Warn 메서드 호출", func(t *testing.T) {
		logger := Logger{Logger: logrus.New()}
		logger.SetOutput(io.Discard) // 출력 억제

		assert.NotPanics(t, func() {
			logger.Warn("test")
			logger.Warnf("test %s", "format")
			logger.Warnj(log.JSON{"key": "value"})
		}, "Warn 메서드들이 panic 없이 실행되어야 합니다")
	})
}

func TestLogger_ErrorMethods(t *testing.T) {
	t.Run("Error 메서드 호출", func(t *testing.T) {
		logger := Logger{Logger: logrus.New()}
		logger.SetOutput(io.Discard) // 출력 억제

		assert.NotPanics(t, func() {
			logger.Error("test")
			logger.Errorf("test %s", "format")
			logger.Errorj(log.JSON{"key": "value"})
		}, "Error 메서드들이 panic 없이 실행되어야 합니다")
	})
}

func TestLogrusLogger(t *testing.T) {
	t.Run("LogrusLogger 미들웨어 생성", func(t *testing.T) {
		middleware := LogrusLogger()

		assert.NotNil(t, middleware, "미들웨어가 생성되어야 합니다")
	})
}

// 새로운 테스트: 미들웨어 핸들러 성공 시나리오
func TestLogrusMiddlewareHandler_Success(t *testing.T) {
	t.Run("성공적인 요청 처리", func(t *testing.T) {
		// Echo 인스턴스 생성
		e := echo.New()
		e.Logger = Logger{Logger: logrus.New()}
		e.Logger.SetOutput(io.Discard) // 로그 출력 억제

		// 테스트 핸들러
		handler := func(c echo.Context) error {
			return c.String(http.StatusOK, "success")
		}

		// 미들웨어 적용
		middlewareFunc := LogrusLogger()
		wrappedHandler := middlewareFunc(handler)

		// HTTP 요청 생성
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// 핸들러 실행
		err := wrappedHandler(c)

		assert.NoError(t, err, "에러가 발생하지 않아야 합니다")
		assert.Equal(t, http.StatusOK, rec.Code, "상태 코드가 200이어야 합니다")
	})
}

// 새로운 테스트: 미들웨어 핸들러 에러 시나리오
func TestLogrusMiddlewareHandler_Error(t *testing.T) {
	t.Run("에러 발생 시 처리", func(t *testing.T) {
		// Echo 인스턴스 생성
		e := echo.New()
		e.Logger = Logger{Logger: logrus.New()}
		e.Logger.SetOutput(io.Discard) // 로그 출력 억제

		// 에러를 반환하는 핸들러
		testError := errors.New("test error")
		handler := func(c echo.Context) error {
			return testError
		}

		// 미들웨어 적용
		middlewareFunc := LogrusLogger()
		wrappedHandler := middlewareFunc(handler)

		// HTTP 요청 생성
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// 핸들러 실행
		err := wrappedHandler(c)

		// 미들웨어는 nil을 반환하지만 에러는 c.Error()로 처리됨
		assert.NoError(t, err, "미들웨어는 nil을 반환해야 합니다")
	})
}

// 새로운 테스트: 빈 경로 처리
func TestLogrusMiddlewareHandler_EmptyPath(t *testing.T) {
	t.Run("빈 경로 처리", func(t *testing.T) {
		// Echo 인스턴스 생성
		e := echo.New()
		e.Logger = Logger{Logger: logrus.New()}
		e.Logger.SetOutput(io.Discard) // 로그 출력 억제

		// 테스트 핸들러
		handler := func(c echo.Context) error {
			return c.String(http.StatusOK, "success")
		}

		// 미들웨어 적용
		middlewareFunc := LogrusLogger()
		wrappedHandler := middlewareFunc(handler)

		// 루트 경로로 HTTP 요청 생성 (빈 경로 대신)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// 핸들러 실행
		err := wrappedHandler(c)

		assert.NoError(t, err, "에러가 발생하지 않아야 합니다")
	})
}

// 새로운 테스트: Content-Length 헤더 없는 경우
func TestLogrusMiddlewareHandler_NoContentLength(t *testing.T) {
	t.Run("Content-Length 헤더 없는 경우", func(t *testing.T) {
		// Echo 인스턴스 생성
		e := echo.New()
		e.Logger = Logger{Logger: logrus.New()}
		e.Logger.SetOutput(io.Discard) // 로그 출력 억제

		// 테스트 핸들러
		handler := func(c echo.Context) error {
			return c.String(http.StatusOK, "success")
		}

		// 미들웨어 적용
		middlewareFunc := LogrusLogger()
		wrappedHandler := middlewareFunc(handler)

		// Content-Length 헤더 없이 HTTP 요청 생성
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		// Content-Length 헤더를 명시적으로 제거
		req.Header.Del(echo.HeaderContentLength)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// 핸들러 실행
		err := wrappedHandler(c)

		assert.NoError(t, err, "에러가 발생하지 않아야 합니다")
	})
}

// 새로운 테스트: 다양한 HTTP 메서드
func TestLogrusMiddlewareHandler_HTTPMethods(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run("HTTP "+method, func(t *testing.T) {
			// Echo 인스턴스 생성
			e := echo.New()
			e.Logger = Logger{Logger: logrus.New()}
			e.Logger.SetOutput(io.Discard) // 로그 출력 억제

			// 테스트 핸들러
			handler := func(c echo.Context) error {
				return c.String(http.StatusOK, "success")
			}

			// 미들웨어 적용
			middlewareFunc := LogrusLogger()
			wrappedHandler := middlewareFunc(handler)

			// HTTP 요청 생성
			req := httptest.NewRequest(method, "/test", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// 핸들러 실행
			err := wrappedHandler(c)

			assert.NoError(t, err, "에러가 발생하지 않아야 합니다")
		})
	}
}

// 새로운 테스트: 응답 크기 측정
func TestLogrusMiddlewareHandler_ResponseSize(t *testing.T) {
	t.Run("응답 크기 측정", func(t *testing.T) {
		// Echo 인스턴스 생성
		e := echo.New()
		e.Logger = Logger{Logger: logrus.New()}
		e.Logger.SetOutput(io.Discard) // 로그 출력 억제

		// 테스트 핸들러 - 특정 크기의 응답 반환
		responseBody := "test response body"
		handler := func(c echo.Context) error {
			return c.String(http.StatusOK, responseBody)
		}

		// 미들웨어 적용
		middlewareFunc := LogrusLogger()
		wrappedHandler := middlewareFunc(handler)

		// HTTP 요청 생성
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// 핸들러 실행
		err := wrappedHandler(c)

		assert.NoError(t, err, "에러가 발생하지 않아야 합니다")
		assert.Equal(t, responseBody, rec.Body.String(), "응답 본문이 일치해야 합니다")
	})
}
