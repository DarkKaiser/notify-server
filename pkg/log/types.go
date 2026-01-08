package log

import (
	"github.com/sirupsen/logrus"
)

// Level logrus.Level의 별칭입니다.
type Level = logrus.Level

const (
	// PanicLevel 가장 높은 심각도입니다. 로그를 기록한 후 panic()을 호출하여 현재 고루틴을 중단합니다.
	// 복구 불가능한 치명적인 내부 오류 발생 시 사용합니다.
	PanicLevel Level = logrus.PanicLevel

	// FatalLevel 치명적인 오류입니다. 로그를 기록한 후 os.Exit(1)을 호출하여 프로세스를 즉시 종료합니다.
	// 애플리케이션 시작 실패, 필수 리소스 로드 실패 등 프로세스가 더 이상 진행할 수 없을 때 사용합니다.
	FatalLevel Level = logrus.FatalLevel

	// ErrorLevel 에러 상황입니다. 프로세스를 종료하지는 않지만, 관리자의 개입이나 버그 수정이 필요한 상태를 나타냅니다.
	ErrorLevel Level = logrus.ErrorLevel

	// WarnLevel 경고 상황입니다. 당장 에러는 아니지만 잠재적인 문제가 있거나 주의가 필요한 상태를 나타냅니다.
	WarnLevel Level = logrus.WarnLevel

	// InfoLevel 일반적인 정보입니다. 시스템의 정상적인 작동 흐름이나 상태 변화를 기록합니다.
	InfoLevel Level = logrus.InfoLevel

	// DebugLevel 디버깅 정보입니다. 개발 및 테스트 단계에서 문제 해결을 위해 상세한 정보를 기록합니다.
	DebugLevel Level = logrus.DebugLevel

	// TraceLevel 가장 세밀한 정보입니다. Debug 레벨보다 더 상세한 데이터 흐름, 내부 변수 상태 등을 추적합니다.
	TraceLevel Level = logrus.TraceLevel
)

// AllLevels logrus.AllLevels의 별칭입니다.
var AllLevels = logrus.AllLevels

// Fields logrus.Fields의 별칭입니다.
type Fields = logrus.Fields

// Entry logrus.Entry의 별칭입니다.
type Entry = logrus.Entry

// Hook logrus.Hook의 별칭입니다.
type Hook = logrus.Hook

// Logger logrus.Logger의 별칭입니다.
type Logger = logrus.Logger

// Formatter logrus.Formatter의 별칭입니다.
type Formatter = logrus.Formatter

// JSONFormatter logrus.JSONFormatter의 별칭입니다.
type JSONFormatter = logrus.JSONFormatter

// TextFormatter logrus.TextFormatter의 별칭입니다.
type TextFormatter = logrus.TextFormatter
