//go:build test

package log

import (
	"os"
	"sync"

	"github.com/sirupsen/logrus"
)

// resetForTest 테스트 간 독립성을 보장하기 위해 패키지 전역 상태를 강제 초기화합니다.
//
// [주의사항]
// 이 함수는 'go test' 실행 시에만 컴파일되는 테스트 전용 헬퍼입니다. (`//go:build test`)
// 싱글톤 패턴으로 관리되는 `setupOnce`와 전역 로거 설정을 초기화하여,
// 이전 테스트의 부작용(Side Effect)이 다음 테스트에 영향을 주지 않도록 합니다.
func resetForTest() {
	// Setup()이 다시 실행될 수 있도록 sync.Once 상태를 초기화합니다.
	// Go의 sync.Once는 내부 'done' 플래그를 되돌릴 수 없으므로, 객체 자체를 재생성해야 합니다.
	setupOnce = sync.Once{}
	globalCloser = nil
	globalSetupErr = nil

	// Logrus 전역 상태 초기화
	logrus.StandardLogger().ReplaceHooks(make(logrus.LevelHooks))
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetReportCaller(false)
	logrus.SetFormatter(&logrus.TextFormatter{})
}
