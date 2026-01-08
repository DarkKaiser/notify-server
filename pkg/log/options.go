package log

import (
	"fmt"
	"os"
)

// Options 로거 설정을 위한 구조체입니다.
type Options struct {
	Name  string // 로그 파일명 생성에 사용될 애플리케이션 식별자
	Dir   string // 로그 파일이 저장될 디렉토리 경로
	Level Level  // 로그 레벨

	MaxAge     int // 오래된 로그 삭제 기준일 (일 단위, 0: 삭제 안 함)
	MaxSizeMB  int // 로그 파일 최대 크기 (MB, 0: 기본값 100MB 사용)
	MaxBackups int // 최대 백업 파일 수 (0: 기본값 20개 사용)

	EnableCriticalLog bool // ERROR 이상(ERROR, FATAL, PANIC)의 치명적 로그를 별도 파일로 분리 저장할지 여부
	EnableVerboseLog  bool // DEBUG 이하(DEBUG, TRACE)의 상세 로그를 별도 파일로 분리 저장할지 여부
	EnableConsoleLog  bool // 표준 출력(Stdout)에도 로그를 출력할지 여부 (개발 환경 권장)

	// 로그를 호출한 소스 코드의 위치(파일명:라인번호)를 함께 기록할지 여부
	// 예: true로 설정 시 "main.go:55" 처럼 로그가 발생한 위치를 알 수 있어 디버깅에 유용합니다.
	ReportCaller bool

	// 로그에 출력되는 파일 경로가 너무 길 때, 앞부분을 잘라내어 보기 좋게 만듭니다.
	// 예: "github.com/my/project/pkg/server.go" -> prefix가 "github.com/my/project"라면 "pkg/server.go"만 출력됨
	CallerPathPrefix string
}

// Validate는 Options 구조체의 필드 값이 유효한지 검증합니다.
func (opts *Options) Validate() error {
	if opts.Name == "" {
		return fmt.Errorf("애플리케이션 식별자(Name)가 설정되지 않았습니다")
	}

	// Dir이 이미 파일로 존재하는지 확인
	if opts.Dir != "" {
		if info, err := os.Stat(opts.Dir); err == nil && !info.IsDir() {
			return fmt.Errorf("로그 디렉토리 경로(%s)가 이미 파일로 존재합니다", opts.Dir)
		}
	}

	if opts.MaxAge < 0 {
		return fmt.Errorf("MaxAge는 0 이상이어야 합니다: %d", opts.MaxAge)
	}
	if opts.MaxSizeMB < 0 {
		return fmt.Errorf("MaxSizeMB는 0 이상이어야 합니다: %d", opts.MaxSizeMB)
	}
	if opts.MaxBackups < 0 {
		return fmt.Errorf("MaxBackups는 0 이상이어야 합니다: %d", opts.MaxBackups)
	}

	return nil
}
