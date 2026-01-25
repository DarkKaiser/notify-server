// Package middleware Echo 프레임워크를 위한 HTTP 미들웨어를 제공합니다.
//
// 이 패키지는 API 서버의 공통 기능을 처리하는 다양한 미들웨어를 포함합니다.
//
// 제공되는 미들웨어:
//
//   - HTTPLogger: HTTP 요청/응답 로깅 (민감 정보 자동 마스킹)
//   - RequireAuthentication: 애플리케이션 키 기반 인증
//   - RateLimit: IP 기반 요청 속도 제한
//   - PanicRecovery: 패닉 복구 및 에러 로깅
//   - DeprecatedEndpoint: 레거시 엔드포인트 경고 헤더 추가
//   - LoggerAdapter: Echo 로거를 애플리케이션 로거로 연결
//
// 사용 예시:
//
//	e := echo.New()
//	e.Use(middleware.PanicRecovery())
//	e.Use(middleware.HTTPLogger())
//	e.Use(middleware.RateLimit(10, 20))
package middleware
