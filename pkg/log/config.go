package log

// NewProductionConfig 운영(Production) 환경에 최적화된 로그 설정을 반환합니다.
func NewProductionConfig(appName string) Options {
	return Options{
		Name:              appName,
		MaxAge:            30,                      // 30일 보관
		EnableCriticalLog: true,                    // 장애 격리
		EnableVerboseLog:  true,                    // 상세 분석 지원
		EnableConsoleLog:  false,                   // 파일 중심 로깅
		ReportCaller:      true,                    // 스택 트레이스 지원
		CallerPathPrefix:  "github.com/darkkaiser", // 경로 단순화
	}
}

// NewDevelopmentConfig 개발(Development) 환경에 최적화된 로그 설정을 반환합니다.
func NewDevelopmentConfig(appName string) Options {
	return Options{
		Name:              appName,
		MaxAge:            1,                       // 가볍게 1일만 보관
		EnableCriticalLog: false,                   // 파일 분리 불필요
		EnableVerboseLog:  false,                   // 파일 분리 불필요
		EnableConsoleLog:  true,                    // 터미널 출력 활성화
		ReportCaller:      true,                    // 디버깅 필수
		CallerPathPrefix:  "github.com/darkkaiser", // 경로 단순화
	}
}
