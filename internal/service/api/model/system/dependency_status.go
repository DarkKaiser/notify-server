package system

// DependencyStatus 외부 의존성 헬스체크 결과
type DependencyStatus struct {
	// 헬스체크 상태: healthy, unhealthy, unknown
	Status string `json:"status" example:"healthy"`
	// 응답 지연시간(ms)
	LatencyMs int64 `json:"latency_ms,omitempty" example:"5"`
	// 상태 상세 정보 또는 에러 메시지
	Message string `json:"message,omitempty" example:"정상 작동 중"`
}
