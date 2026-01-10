package infra

// HealthResponse 서버 상태 응답 모델
type HealthResponse struct {
	// 서버 상태 (healthy, unhealthy)
	Status string `json:"status" example:"healthy"`
	// 서버 가동 시간 (초)
	Uptime int64 `json:"uptime" example:"3600"`
	// 의존성 상태 (선택적)
	Dependencies map[string]DependencyStatus `json:"dependencies,omitempty"`
}
