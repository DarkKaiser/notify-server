// Package common은 애플리케이션 전반에서 사용되는 공통 타입과 유틸리티를 제공합니다.
//
// # BuildInfo 사용 예제
//
// 빌드 시점에 주입되는 버전 정보를 관리하기 위해 BuildInfo를 사용합니다.
// 일반적으로 main 패키지에서 빌드 타임 변수로 선언하고, -ldflags를 통해 값을 주입합니다.
//
//	// main.go
//	var (
//	    Version     = "dev"
//	    BuildDate   = "unknown"
//	    BuildNumber = "0"
//	)
//
//	func main() {
//	    buildInfo := common.BuildInfo{
//	        Version:     Version,
//	        BuildDate:   BuildDate,
//	        BuildNumber: BuildNumber,
//	    }
//
//	    log.Printf("Starting %s version %s (build: %s, date: %s)",
//	        config.AppName,
//	        buildInfo.Version,
//	        buildInfo.BuildNumber,
//	        buildInfo.BuildDate,
//	    )
//	}
//
// 빌드 시 버전 정보 주입:
//
//	go build -ldflags "-X main.Version=v1.2.3 -X main.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X main.BuildNumber=456"
package common

// BuildInfo 애플리케이션의 빌드 정보를 담고 있습니다.
//
// 이 구조체는 애플리케이션의 버전, 빌드 날짜, 빌드 번호 등의 메타데이터를 저장합니다.
// 주로 /version API 엔드포인트나 로그 출력에 사용됩니다.
type BuildInfo struct {
	Version     string // Git 커밋 해시 또는 버전 태그 (예: "v1.2.3" 또는 "abc123def")
	BuildDate   string // 빌드 날짜 (ISO 8601 형식 권장, 예: "2025-12-05T11:30:00Z")
	BuildNumber string // CI/CD 빌드 번호 (예: "456")
}
