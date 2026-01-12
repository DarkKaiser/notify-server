package constants

import "time"

// 서버 설정 기본값 상수입니다.
const (
	// DefaultRequestTimeout HTTP 요청 처리의 기본 타임아웃 시간 (60초)
	// 별도의 타임아웃 설정이 없는 경우 이 값이 적용되며, 요청 처리가 이 시간을 초과하면
	// 자동으로 취소되어 서버 리소스를 보호합니다.
	DefaultRequestTimeout = 60 * time.Second
)
