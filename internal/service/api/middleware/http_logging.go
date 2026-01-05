// Package middleware HTTP 요청/응답 로깅을 위한 미들웨어를 제공합니다.
//
// 이 패키지는 Echo 프레임워크와 통합되어 모든 HTTP 요청과 응답을 구조화된 로그로 기록합니다.
// 민감한 쿼리 파라미터는 자동으로 마스킹되어 보안을 유지합니다.
package middleware

import (
	"net/url"
	"strconv"
	"time"

	"github.com/darkkaiser/notify-server/pkg/strutil"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

const (
	// defaultBytesIn Content-Length 헤더가 없을 때 사용하는 기본값
	defaultBytesIn = "0"
)

// sensitiveQueryParams 민감 정보로 간주할 쿼리 파라미터 목록입니다.
// 로그에 기록될 때 이 목록의 파라미터 값은 마스킹됩니다.
var sensitiveQueryParams = []string{
	"app_key",
	"api_key",
	"password",
	"token",
	"secret",
}

// HTTPLogger HTTP 요청/응답 정보를 로깅하는 미들웨어를 반환합니다.
//
// 이 미들웨어는 다음 정보를 구조화된 로그로 기록합니다:
//   - 요청 정보: IP, 메서드, URI, User-Agent 등
//   - 응답 정보: 상태 코드, 응답 크기
//   - 성능 정보: 요청 처리 시간
//   - 민감 정보: 쿼리 파라미터의 민감 정보는 자동으로 마스킹됨
//
// 로그 필드:
//   - time_rfc3339: 요청 완료 시각 (RFC3339 형식)
//   - remote_ip: 클라이언트 IP 주소
//   - host: 요청 호스트
//   - uri: 요청 URI (민감 정보 마스킹됨)
//   - method: HTTP 메서드 (GET, POST 등)
//   - path: URL 경로
//   - referer: Referer 헤더
//   - user_agent: User-Agent 헤더
//   - status: HTTP 응답 상태 코드
//   - latency: 요청 처리 시간 (마이크로초)
//   - latency_human: 요청 처리 시간 (사람이 읽기 쉬운 형식)
//   - bytes_in: 요청 본문 크기 (바이트)
//   - bytes_out: 응답 본문 크기 (바이트)
//   - request_id: 요청 ID (X-Request-ID 헤더)
//
// 사용 예시:
//
//	e := echo.New()
//	e.Use(middleware.HTTPLogger())
func HTTPLogger() echo.MiddlewareFunc {
	return httpLogger
}

// httpLogger 실제 로깅 미들웨어 함수입니다.
func httpLogger(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		return httpLoggerHandler(c, next)
	}
}

// httpLoggerHandler HTTP 요청/응답 정보를 로깅하는 미들웨어 핸들러입니다.
//
// 이 함수는 다음 순서로 동작합니다:
//  1. 요청 시작 시간 기록
//  2. 다음 핸들러 실행 (에러 발생 시 Echo의 에러 핸들러로 전달)
//  3. 요청 완료 시간 기록 및 레이턴시 계산
//  4. 요청/응답 정보를 구조화된 로그로 기록
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
	logrus.WithFields(map[string]interface{}{
		"time_rfc3339":  stop.Format(time.RFC3339),
		"remote_ip":     c.RealIP(),
		"host":          req.Host,
		"uri":           uri,
		"method":        req.Method,
		"path":          path,
		"referer":       req.Referer(),
		"user_agent":    req.UserAgent(),
		"status":        res.Status,
		"latency":       strconv.FormatInt(latency.Nanoseconds()/1000, 10),
		"latency_human": latency.String(),
		"bytes_in":      bytesIn,
		"bytes_out":     strconv.FormatInt(res.Size, 10),
		"request_id":    res.Header().Get(echo.HeaderXRequestID),
	}).Info("HTTP request")

	return nil
}

// maskSensitiveQueryParams URI의 민감 정보를 마스킹합니다.
//
// sensitiveQueryParams 목록에 있는 쿼리 파라미터의 값을 strutils.Mask로 대체합니다.
// URI 파싱에 실패한 경우, 원본 URI를 그대로 반환하여 로깅이 중단되지 않도록 합니다.
//
// 예시:
//
//	입력: "/api/v1/test?app_key=secret123&id=100"
//	출력: "/api/v1/test?app_key=secr***123&id=100"
func maskSensitiveQueryParams(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		// 파싱 실패 시 원본 반환 (로깅 중단 방지)
		return uri
	}

	q := u.Query()
	masked := false

	for _, param := range sensitiveQueryParams {
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
