package constants

import "time"

// 보안 관련 상수입니다.
const (
	// DefaultMaxBodySize 요청 본문의 최대 크기 (128KB)
	// DoS 공격 방지 및 메모리 보호를 위해 제한합니다.
	DefaultMaxBodySize = "128K"

	// DefaultReadHeaderTimeout HTTP 헤더 읽기 최대 대기 시간 (10초)
	// Slowloris DoS 공격을 방어하기 위해 헤더를 매우 느리게 전송하는
	// 악의적인 클라이언트의 연결 고갈 공격을 방지합니다.
	DefaultReadHeaderTimeout = 10 * time.Second
)

// SensitiveQueryParams 로그 기록 시 마스킹 처리해야 할 쿼리 파라미터 목록입니다.
var SensitiveQueryParams = []string{
	QueryParamAppKey,
	"api_key",
	"password",
	"token",
	"secret",
}
