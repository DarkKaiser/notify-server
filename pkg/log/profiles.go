package log

// NewProductionOptions 운영(Production) 환경에 최적화된 로그 설정을 반환합니다.
func NewProductionOptions(appName string) Options {
	return Options{
		Name:  appName,
		Level: InfoLevel, // 운영 환경 로그 레벨

		MaxAge:     30,  // 30일 보관
		MaxSizeMB:  100, // 100MB 단위 로테이션
		MaxBackups: 20,  // 최대 20개 백업 유지

		EnableCriticalLog: true,  // 장애 대응을 위한 중요 로그 격리
		EnableVerboseLog:  true,  // 문제 추적을 위한 상세 로그 분리
		EnableConsoleLog:  false, // 터미널 출력 비활성화

		ReportCaller:     true, // 정확한 문제 원인 파악을 위한 호출 위치 기록
		CallerPathPrefix: "",   // 기본값: 전체 경로 출력
	}
}

// NewDevelopmentOptions 개발(Development) 환경에 최적화된 로그 설정을 반환합니다.
func NewDevelopmentOptions(appName string) Options {
	return Options{
		Name:  appName,
		Level: TraceLevel, // 개발 환경 로그 레벨

		MaxAge:     1,  // 1일 보관
		MaxSizeMB:  50, // 50MB 단위 로테이션
		MaxBackups: 5,  // 최대 5개 백업 유지

		EnableCriticalLog: false, // 개발 편의를 위한 로그 파일 통합
		EnableVerboseLog:  false, // 개발 편의를 위한 로그 파일 통합
		EnableConsoleLog:  true,  // 터미널 출력 활성화

		ReportCaller:     true, // 정확한 문제 원인 파악을 위한 호출 위치 기록
		CallerPathPrefix: "",   // 기본값: 전체 경로 출력
	}
}
