package constants

// API 요청 시 URL 쿼리 스트링으로 전달되는 파라미터 키 상수입니다.
const (
	// QueryParamAppKey 애플리케이션 인증에 사용되는 쿼리 파라미터 키입니다.
	QueryParamAppKey = "app_key"
)

// API 요청 및 응답에 사용되는 HTTP 헤더 키 상수입니다.
const (
	// HeaderAppKey 애플리케이션 인증에 사용되는 HTTP 헤더 키입니다.
	// 향후 쿼리 파라미터 대신 헤더 기반 인증으로 전환 시 사용됩니다.
	HeaderAppKey = "X-App-Key"

	// HeaderWarning RFC 7234 표준 Warning 헤더입니다.
	// deprecated 엔드포인트에서 경고 메시지를 전달할 때 사용됩니다.
	HeaderWarning = "Warning"

	// HeaderXAPIDeprecated deprecated 상태를 나타내는 커스텀 헤더입니다.
	HeaderXAPIDeprecated = "X-API-Deprecated"

	// HeaderXAPIDeprecatedReplacement 대체 엔드포인트를 나타내는 커스텀 헤더입니다.
	HeaderXAPIDeprecatedReplacement = "X-API-Deprecated-Replacement"
)
