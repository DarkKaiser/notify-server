package constants

import "time"

// 서버 설정 기본값 상수입니다.
const (
	// RequestTimeout HTTP 요청 처리의 최대 허용 시간 (60초)
	// 이 타임아웃은 Echo의 Timeout 미들웨어에 적용되며, 핸들러 실행 시간을 제한합니다.
	// 지정된 시간을 초과하는 요청은 자동으로 중단되고 503 Service Unavailable 응답을 반환하여,
	// 장기 실행 요청으로 인한 서버 리소스 고갈 및 과부하를 방지합니다.
	RequestTimeout = 60 * time.Second
)
