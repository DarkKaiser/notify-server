package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/go-viper/mapstructure/v2"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

const (
	// AppName 애플리케이션의 전역 고유 식별자입니다.
	AppName string = "notify-server"

	// DefaultFilename 애플리케이션 초기화 시 참조하는 기본 설정 파일명입니다.
	// 실행 인자를 통해 명시적인 경로가 제공되지 않을 경우, 시스템은 이 파일을 탐색하여 구성을 로드합니다.
	DefaultFilename = AppName + ".json"

	// ------------------------------------------------------------------------------------------------
	// HTTP 재시도 정책 기본값
	// ------------------------------------------------------------------------------------------------

	// DefaultMaxRetries HTTP 요청 실패 시 기본 재시도 횟수입니다.
	DefaultMaxRetries = 3

	// DefaultRetryDelay HTTP 요청 실패 시 기본 재시도 사이의 대기 시간입니다.
	DefaultRetryDelay = 2 * time.Second
)

// newDefaultConfig 애플리케이션의 모든 설정에 대한 '기본값'을 정의하고 초기화합니다.
// 사용자 설정이 누락되더라도 안전하게 실행될 수 있도록 미리 값을 채워주는 역할을 합니다.
func newDefaultConfig() AppConfig {
	return AppConfig{
		Debug: true,
		HTTPRetry: HTTPRetryConfig{
			MaxRetries: DefaultMaxRetries,
			RetryDelay: DefaultRetryDelay,
		},
	}
}

// Load 기본 설정 파일을 읽어 애플리케이션 설정을 로드합니다.
func Load() (*AppConfig, error) {
	return LoadWithFile(DefaultFilename)
}

// LoadWithFile 지정된 경로의 설정 파일을 읽어 AppConfig 객체를 생성합니다.
func LoadWithFile(filename string) (*AppConfig, error) {
	k := koanf.New(".")

	// 1. 기본값 로드 (가장 낮은 우선순위)
	err := k.Load(structs.Provider(newDefaultConfig(), "json"), nil)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.System, "애플리케이션 기본 설정 로드에 실패했습니다")
	}

	// 2. JSON 설정 파일 로드 (기본값 덮어쓰기)
	if err := k.Load(file.Provider(filename), json.Parser()); err != nil {
		if os.IsNotExist(err) {
			return nil, apperrors.Wrap(err, apperrors.System, fmt.Sprintf("설정 파일을 찾을 수 없습니다: '%s'", filename))
		}
		return nil, apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("설정 파일 로드 중 오류가 발생했습니다: '%s'", filename))
	}

	// 3. 환경 변수 로드 (최우선 순위, JSON 설정 덮어쓰기)
	// 접두사: NOTIFY_
	// 구분자: 이중 언더스코어(__)를 점(.)으로 변환 (계층 구조 표현)
	// 예: NOTIFY_HTTP_RETRY__MAX_RETRIES -> http_retry.max_retries
	if err := k.Load(env.Provider("NOTIFY_", ".", normalizeEnvKey), nil); err != nil {
		return nil, apperrors.Wrap(err, apperrors.System, "환경 변수 로드에 실패했습니다")
	}

	// 4. 구조체 언마샬링 (Strict Validation 적용)
	unmarshalConf := koanf.UnmarshalConf{
		Tag: "json",
		DecoderConfig: &mapstructure.DecoderConfig{
			ErrorUnused:      true, // 파일에 존재하지만 구조체에 없는 필드가 있을 경우 에러를 발생시킴
			WeaklyTypedInput: true,
			DecodeHook: mapstructure.ComposeDecodeHookFunc(
				mapstructure.StringToTimeDurationHookFunc(),
			),
		},
	}
	var appConfig AppConfig
	if err := k.UnmarshalWithConf("", &appConfig, unmarshalConf); err != nil {
		return nil, apperrors.Wrap(err, apperrors.System, "설정 데이터를 애플리케이션 구조체로 변환하는데 실패했습니다")
	}

	// 5. 유효성 검사 수행
	if err := appConfig.validate(newValidator()); err != nil {
		return nil, apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("설정 파일('%s')의 유효성 검증에 실패했습니다", filename))
	}

	return &appConfig, nil
}

// normalizeEnvKey 환경 변수 키를 내부 설정 구조체에 매핑하기 위해 표준화된 키 형식으로 변환합니다.
// Koanf의 환경 변수 로더가 이 함수를 사용하여 'NOTIFY_' 접두사가 붙은 환경 변수를 올바른 설정 경로로 해석합니다.
func normalizeEnvKey(s string) string {
	s = strings.ToLower(strings.TrimPrefix(s, "NOTIFY_"))
	return strings.ReplaceAll(s, "__", ".")
}
