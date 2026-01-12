package constants

// SensitiveQueryParams 로그 기록 시 마스킹 처리해야 할 쿼리 파라미터 목록입니다.
var SensitiveQueryParams = []string{
	QueryParamAppKey,
	"api_key",
	"password",
	"token",
	"secret",
}
