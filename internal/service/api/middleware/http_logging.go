package middleware

import (
	"net/url"
	"strconv"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	"github.com/labstack/echo/v4"
)

const (
	// defaultBytesIn Content-Length 헤더가 없을 때 사용하는 기본값
	defaultBytesIn = "0"
)

// HTTPLogger HTTP 요청/응답을 구조화된 로그로 기록하는 미들웨어를 반환합니다.
//
// 기록되는 정보:
//   - 요청: IP, 메서드, URI, User-Agent, Content-Length
//   - 응답: 상태 코드, 응답 크기, Request ID
//   - 성능: 처리 시간 (마이크로초 및 사람이 읽기 쉬운 형식)
//   - 보안: 민감한 쿼리 파라미터 자동 마스킹 (app_key, password 등)
//
// 사용 예시:
//
//	e := echo.New()
//	e.Use(middleware.HTTPLogger())
func HTTPLogger() echo.MiddlewareFunc {
	return httpLogger
}

// httpLogger Echo 미들웨어 함수를 생성합니다.
func httpLogger(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		return httpLoggerHandler(c, next)
	}
}

// httpLoggerHandler HTTP 요청/응답을 로깅하는 핵심 핸들러입니다.
//
// 처리 흐름:
//  1. 요청 시작 시간 기록
//  2. 다음 핸들러 실행 (에러는 Echo 에러 핸들러로 전달)
//  3. 레이턴시 계산 및 민감 정보 마스킹
//  4. 구조화된 로그 기록 (JSON 형식)
func httpLoggerHandler(c echo.Context, next echo.HandlerFunc) error {
	req := c.Request()
	res := c.Response()
	start := time.Now()

	// 핸들러 실행
	if err := next(c); err != nil {
		c.Error(err)
	}

	stop := time.Now()
	latency := stop.Sub(start)

	// 경로 정규화
	path := req.URL.Path
	if path == "" {
		path = "/"
	}

	// Content-Length 헤더 가져오기
	bytesIn := req.Header.Get(echo.HeaderContentLength)
	if bytesIn == "" {
		bytesIn = defaultBytesIn
	}

	// 민감 정보 마스킹
	uri := maskSensitiveQueryParams(req.RequestURI)

	// 구조화된 로그 기록
	applog.WithFields(applog.Fields{
		// 시간 정보
		"time_rfc3339": stop.Format(time.RFC3339),

		// 요청 정보
		"method":   req.Method,
		"path":     path,
		"uri":      uri,
		"host":     req.Host,
		"protocol": req.Proto,

		// 클라이언트 정보
		"remote_ip":  c.RealIP(),
		"user_agent": req.UserAgent(),
		"referer":    req.Referer(),

		// 응답 정보
		"status":    res.Status,
		"bytes_in":  bytesIn,
		"bytes_out": strconv.FormatInt(res.Size, 10),

		// 성능 정보
		"latency":       strconv.FormatInt(latency.Nanoseconds()/1000, 10),
		"latency_human": latency.String(),

		// 추적 정보
		"request_id": res.Header().Get(echo.HeaderXRequestID),
	}).Info("HTTP 요청")

	return nil
}

// maskSensitiveQueryParams URI의 민감한 쿼리 파라미터를 마스킹합니다.
//
// constants.SensitiveQueryParams에 정의된 파라미터(app_key, password 등)의
// 값을 strutil.Mask로 마스킹합니다. URI 파싱 실패 시 원본을 반환하여
// 로깅이 중단되지 않도록 합니다.
//
// 예시:
//
//	입력: "/api/v1/test?app_key=secret123&id=100"
//	출력: "/api/v1/test?app_key=secr***&id=100"
func maskSensitiveQueryParams(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		// 파싱 실패 시 원본 반환 (로깅 중단 방지)
		return uri
	}

	q := u.Query()
	masked := false

	for _, param := range constants.SensitiveQueryParams {
		if q.Has(param) {
			val := q.Get(param)
			q.Set(param, strutil.Mask(val))
			masked = true
		}
	}

	if masked {
		u.RawQuery = q.Encode()
		return u.String()
	}

	return uri
}
