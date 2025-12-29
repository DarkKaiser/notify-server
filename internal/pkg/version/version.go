// Package version은 애플리케이션의 빌드 메타데이터를 관리합니다.
//
// # Info 사용 예제
//
// 빌드 시점에 주입되는 버전 정보를 관리하기 위해 version.Info를 사용합니다.
// 일반적으로 main 패키지에서 빌드 타임 변수로 선언하고, -ldflags를 통해 값을 주입한 후
// Set() 함수를 통해 전역에 등록합니다.
//
//	// main.go
//	var (
//	    Version     = "dev"
//	    BuildDate   = "unknown"
//	    BuildNumber = "0"
//	)
//
//	func main() {
//	    buildInfo := version.Info{
//	        Version:     Version,
//	        BuildDate:   BuildDate,
//	        BuildNumber: BuildNumber,
//	    }
//	    version.Set(buildInfo)
//
//	    // 어디서든 안전하게 접근 가능
//	    log.Printf("Starting app: %s", version.Get())
//	}
//
// 빌드 시 버전 정보 주입:
//
//	go build -ldflags "-X main.Version=v1.2.3 -X main.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X main.BuildNumber=456"
package version

import (
	"fmt"
	"runtime"
	"sync/atomic"
)

// 전역 빌드 정보 (Atomic Value를 사용하여 Thread-Safe 보장)
var globalInfo atomic.Value

// Info 애플리케이션의 빌드 정보를 담고 있습니다.
//
// 이 구조체는 애플리케이션의 버전, 빌드 날짜, 빌드 번호 등의 메타데이터를 저장합니다.
// 주로 /version API 엔드포인트나 로그 출력에 사용됩니다.
type Info struct {
	Version     string `json:"version"`      // Git 커밋 해시 또는 버전 태그 (예: "v1.2.3" 또는 "abc123def")
	BuildDate   string `json:"build_date"`   // 빌드 날짜 (ISO 8601 형식 권장, 예: "2025-12-05T11:30:00Z")
	BuildNumber string `json:"build_number"` // CI/CD 빌드 번호 (예: "456")
	GoVersion   string `json:"go_version"`   // Go 런타임 버전 (예: "go1.21.0")
	OS          string `json:"os"`           // 운영체제 (예: "linux", "darwin", "windows")
	Arch        string `json:"arch"`         // 아키텍처 (예: "amd64", "arm64")
}

// String Info를 사람이 읽기 쉬운 문자열로 포맷팅합니다.
// 예: "v1.2.3 (build: 456, date: ...)"
func (i Info) String() string {
	if i.Version == "" {
		return "unknown"
	}
	return fmt.Sprintf("%s (build: %s, date: %s, go_version: %s, os: %s, arch: %s)", i.Version, i.BuildNumber, i.BuildDate, i.GoVersion, i.OS, i.Arch)
}

// Get 전역 설정된 빌드 정보를 반환합니다.
// 설정되지 않았을 경우 빈 Info(Zero Value)를 반환합니다.
// Thread-Safe 합니다.
func Get() Info {
	val := globalInfo.Load()
	if val == nil {
		return Info{Version: "unknown", BuildDate: "unknown", BuildNumber: "0"}
	}
	return val.(Info)
}

// Set 전역 빌드 정보를 설정합니다.
// 애플리케이션 시작 시(보통 main 함수) 단 한 번만 호출해야 합니다.
// GoVersion, OS, Arch 등의 런타임 정보는 자동으로 채워집니다.
func Set(info Info) {
	if info.GoVersion == "" {
		info.GoVersion = runtime.Version()
	}
	if info.OS == "" {
		info.OS = runtime.GOOS
	}
	if info.Arch == "" {
		info.Arch = runtime.GOARCH
	}
	globalInfo.Store(info)
}
