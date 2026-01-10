package system

// DependencyStatus 의존성 상태 모델
type DependencyStatus struct {
	// 상태 (healthy, unhealthy, unknown)
	Status string `json:"status" example:"healthy"`
	// 응답 시간 (밀리초, 선택적)
	LatencyMs int64 `json:"latency_ms,omitempty" example:"5"`
	// 추가 메시지 (선택적)
	Message string `json:"message,omitempty" example:"정상 작동 중"`
}
