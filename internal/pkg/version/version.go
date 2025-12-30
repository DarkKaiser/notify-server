// Package version 애플리케이션의 빌드 및 버저닝 정보를 관리하는 패키지입니다.
//
// 시스템의 빌드 시점(Build-Time)에 주입된 메타데이터(버전, 커밋 해시, 빌드 시간 등)와
// 실행 시점(Run-Time)의 환경 정보(Go 버전, OS, 아키텍처)를 통합하여 제공합니다.
//
// 주요 기능:
//  1. 빌드 정보 주입: 링커 플래그(-ldflags)를 통해 외부에서 버전을 주입받습니다.
//  2. 런타임 정보 통합: 실행 환경의 정보를 자동으로 감지하여 추가합니다.
//  3. Thread-Safe 접근: 전역적으로 안전하게 정보를 조회할 수 있는 메서드를 제공합니다.
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"sync/atomic"
)

// 전역 빌드 정보 (Atomic Value를 사용하여 Thread-Safe 보장)
var globalBuildInfo atomic.Value

// -----------------------------------------------------------------------------
// 빌드 정보 변수
// -----------------------------------------------------------------------------
// 다음 변수들은 Dockerfile에서 컴파일 시점에 링커 플래그(ldflags)를 통해 주입됩니다.
//
// 주의: 이 변수들은 외부에서 값을 주입받기 위한 '컨테이너' 역할만 수행합니다.
// 실제 애플리케이션 로직에서는 이 변수들에 직접 접근하지 말고,
// 반드시 Get() 함수나 Info 구조체를 통해 안전하게 접근해야 합니다.
var (
	appVersion  = "" // 애플리케이션 버전 (예: v1.0.1-155-gf25b8bf)
	gitCommit   = "" // Git 커밋 해시 (예: f25b8bf)
	buildDate   = "" // 빌드 수행 시간
	buildNumber = "" // CI/CD 파이프라인 빌드 번호
)

// init 애플리케이션의 빌드 정보를 초기화합니다.
func init() {
	bi := Info{
		Version:     appVersion,
		Commit:      gitCommit,
		BuildDate:   buildDate,
		BuildNumber: buildNumber,
	}
	set(bi)
}

// Info 애플리케이션의 빌드 정보를 담고 있습니다.
//
// 이 구조체는 애플리케이션의 버전, 빌드 날짜, 빌드 번호 등의 메타데이터를 저장합니다.
// 주로 /version API 엔드포인트나 로그 출력에 사용됩니다.
type Info struct {
	Version     string `json:"version"`      // 애플리케이션의 버전 (예: v1.0.1-155-gf25b8bf)
	Commit      string `json:"commit"`       // Git 커밋 해시 (예: f25b8bf)
	BuildDate   string `json:"build_date"`   // 빌드 날짜 (ISO 8601 형식 권장, 예: "2025-12-05T11:30:00Z")
	BuildNumber string `json:"build_number"` // CI/CD 빌드 번호 (예: "456")
	GoVersion   string `json:"go_version"`   // 빌드에 사용된 Go 컴파일러 버전 (예: "go1.21.0")
	OS          string `json:"os"`           // 실행 중인 운영체제 (예: "linux", "darwin", "windows")
	Arch        string `json:"arch"`         // 실행 중인 시스템 아키텍처 (예: "amd64")
	DirtyBuild  bool   `json:"dirty_build"`  // 빌드 시점에 로컬 소스코드에서 변경사항이 있었는지의 여부
}

// Get 애플리케이션의 빌드 정보를 반환합니다.
func Get() Info {
	bi := globalBuildInfo.Load()
	if bi == nil {
		return Info{
			Version:     "unknown",
			Commit:      "unknown",
			BuildDate:   "unknown",
			BuildNumber: "0",
		}
	}
	return bi.(Info)
}

// set 애플리케이션의 빌드 정보를 설정합니다.
func set(bi Info) {
	globalBuildInfo.Store(collectRuntimeAndBuildMetadata(bi))
}

// collectRuntimeAndBuildMetadata 초기화되지 않은 빌드 정보에 런타임 환경 값(Go 버전, OS, Arch)을 채워 넣습니다.
//
// 또한, 버전 정보가 누락된 경우 실행 파일의 디버그 메타데이터(debug.ReadBuildInfo)를 분석하여
// VCS 리비전이나 수정 상태(Dirty) 등의 정보를 보강(Enrich)하는 역할을 수행합니다.
// 순수 함수(Pure Function)로 설계되어 단위 테스트가 용이합니다.
func collectRuntimeAndBuildMetadata(bi Info) Info {
	if bi.GoVersion == "" {
		bi.GoVersion = runtime.Version()
	}
	if bi.OS == "" {
		bi.OS = runtime.GOOS
	}
	if bi.Arch == "" {
		bi.Arch = runtime.GOARCH
	}
	if bi.Commit == "" || bi.Commit == "none" {
		bi.Commit = "unknown"
	}

	// 이미 필수 정보가 모두 있다면 VCS(Git) 메타데이터 확인 스킵 (최적화)
	if bi.Version != "" && bi.Commit != "unknown" && !bi.DirtyBuild {
		return bi
	}

	// Go 모듈(debug.BuildInfo)을 통해 VCS(Git) 메타데이터 추출을 시도합니다.
	// 이는 -ldflags 주입이 누락된 개발 환경(go run 등)에서도 최소한의 버전 정보를 확보하기 위함입니다.
	if func() bool {
		// 테스트 가능성 확보를 위한 래퍼 함수 패턴 (현재는 단순 실행)
		return true
	}() {
		if val, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range val.Settings {
				switch setting.Key {
				case "vcs.revision":
					// VCS 리비전은 항상 Commit 필드로 매핑
					// 외부에서 주입된 값이 "unknown"이나 "none"일 경우에만 덮어씀
					if bi.Commit == "unknown" || bi.Commit == "none" {
						bi.Commit = setting.Value
					}
				case "vcs.time":
					if bi.BuildDate == "" || bi.BuildDate == "unknown" {
						bi.BuildDate = setting.Value
					}
				case "vcs.modified":
					if setting.Value == "true" {
						bi.DirtyBuild = true
					}
				}
			}
			// 여전히 버전이 비어있다면 Main 모듈 버전 사용 시도
			if bi.Version == "" && val.Main.Version != "(devel)" {
				bi.Version = val.Main.Version
			}
		}
	}

	return bi
}

// Version 애플리케이션의 버전 문자열을 반환합니다.
func Version() string {
	return Get().Version
}

// Commit Git 커밋 해시를 반환합니다.
func Commit() string {
	return Get().Commit
}

// ToMap 빌드 정보를 맵(Map) 형태로 반환합니다. (구조적 로깅용)
func (i Info) ToMap() map[string]string {
	return map[string]string{
		"version":      i.Version,
		"commit":       i.Commit,
		"build_date":   i.BuildDate,
		"build_number": i.BuildNumber,
		"go_version":   i.GoVersion,
		"os":           i.OS,
		"arch":         i.Arch,
		"dirty_build":  fmt.Sprintf("%t", i.DirtyBuild),
	}
}

// String 빌드 정보를 사람이 읽기 쉬운 하나의 문자열로 요약해 반환합니다.
func (i Info) String() string {
	if i.Version == "" {
		return "unknown"
	}
	version := i.Version
	if i.DirtyBuild {
		version += "-dirty"
	}

	// 커밋 해시가 있으면 포함하여 출력
	commit := i.Commit
	if commit == "" {
		commit = "unknown"
	} else if len(commit) > 7 {
		// 로그 가독성을 위해 Short 해시로 축약
		commit = commit[:7]
	}

	return fmt.Sprintf("%s (commit: %s, build: %s, date: %s, go_version: %s, os: %s, arch: %s)",
		version, commit, i.BuildNumber, i.BuildDate, i.GoVersion, i.OS, i.Arch)
}
