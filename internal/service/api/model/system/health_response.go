package system

// HealthResponse 서버 헬스체크 응답
type HealthResponse struct {
	// 전체 헬스체크 상태: healthy, unhealthy
	Status string `json:"status" example:"healthy"`
	// 서버 가동 시간(초)
	Uptime int64 `json:"uptime" example:"3600"`
	// 외부 의존성별 헬스체크 결과 (키: 의존성 이름)
	Dependencies map[string]DependencyStatus `json:"dependencies,omitempty"`
}
