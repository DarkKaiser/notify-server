package common

// BuildInfo 애플리케이션의 빌드 정보를 담고 있습니다.
type BuildInfo struct {
	Version     string // Git 커밋 해시 또는 버전
	BuildDate   string // 빌드 날짜
	BuildNumber string // 빌드 번호
}
