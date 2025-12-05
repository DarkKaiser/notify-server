package response

// VersionResponse 서버 버전 정보 응답 모델
type VersionResponse struct {
	// Git 커밋 해시
	Version string `json:"version" example:"abc1234"`
	// 빌드 날짜 (UTC)
	BuildDate string `json:"build_date" example:"2025-12-01T14:00:00Z"`
	// 빌드 번호
	BuildNumber string `json:"build_number" example:"100"`
	// Go 버전
	GoVersion string `json:"go_version" example:"go1.24.0"`
}
