package constants

// 보안상 로그에 남길 때 마스킹(가림) 처리해야 할 쿼리 파라미터 목록입니다.
var SensitiveQueryParams = []string{
	QueryParamAppKey,
	"api_key",
	"password",
	"token",
	"secret",
}
