// Package maputil 맵(Map) 데이터 처리 및 구조체 변환을 위한 유틸리티 기능을 제공합니다.
package maputil

import (
	"errors"
	"fmt"

	"github.com/mitchellh/mapstructure"
)

// Decode 입력된 맵(Map)이나 인터페이스 데이터를 지정된 제네릭 타입 T의 구조체로 변환하여 반환합니다.
//
// 내부적으로 `mapstructure` 라이브러리를 활용하며, 안전하고 유연한 디코딩을 위한 기본 설정이 적용되어 있습니다.
// 필요에 따라 `Option` 함수들을 가변 인자로 전달하여 동작 방식을 세밀하게 제어할 수 있습니다.
//
// [주요 특징 및 기본 동작]
//   - 유연한 타입 변환 (Weakly Typed): "123" -> 123 (int), 1 -> true (bool) 등 타입을 자동으로 보정합니다.
//   - 구조체 평탄화 (Squash): 임베디드 구조체를 자동으로 평탄화하여 상위 맵 필드와 매핑합니다.
//   - 태그 지원: 기본적으로 구조체의 `json` 태그를 기준으로 필드를 매핑합니다.
//   - 고급 타입 지원: `[]byte` (Base64 자동 디코딩), `time.Duration`, `net.IP` 등을 위한 전용 훅이 내장되어 있습니다.
//
// [주의사항]
// 기본적으로 `ErrorUnused` 옵션이 꺼져 있습니다 (`false`).
// 따라서 구조체에 정의되지 않은 필드가 입력 데이터에 포함되어 있어도 에러 없이 무시됩니다.
//
// [사용 예시]
//
//	// 기본 사용
//	cfg, err := maputil.Decode[MyConfig](inputMap)
//
//	// 옵션 적용 (엄격한 검증)
//	cfg, err := maputil.Decode[MyConfig](inputMap,
//	    maputil.WithErrorUnused(true),
//	    maputil.WithTagName("yaml"),
//	)
func Decode[T any](input any, opts ...Option) (*T, error) {
	output := new(T)

	// new(T)로 생성된 객체는 이미 모든 필드가 Zero Value이므로,
	// 기본적으로 WithZeroFields(false)를 적용하여 불필요한 중복 초기화를 방지합니다.
	//
	// 옵션 적용 순서: 기본값 먼저, 사용자 옵션 나중
	baseOpts := []Option{
		WithZeroFields(false),
	}
	allOpts := append(baseOpts, opts...)

	if err := DecodeTo(input, output, allOpts...); err != nil {
		return nil, err
	}
	return output, nil
}

// DecodeTo 입력된 데이터를 대상 구조체 포인터(output)에 디코딩하여 값을 채웁니다.
//
// [제약 사항]
// output 인자는 반드시 `nil`이 아닌 포인터여야 합니다. (Run-time Panic 방지)
//
// [동작 방식]
//
//  1. Merge Semantics (기본 동작):
//     기존 output 구조체에 값이 있다면 유지하며, 입력 데이터와 병합(Merge)합니다.
//     완전한 초기화 후 디코딩(Replace)을 원한다면 `WithZeroFields(true)` 옵션을 사용하십시오.
//
//  2. Recursive Structures:
//     순환 참조(Cyclic Reference)가 포함된 입력 데이터는 무한 루프를 유발할 수 있으므로 주의가 필요합니다.
func DecodeTo[T any](input any, output *T, opts ...Option) error {
	if output == nil {
		return errors.New("디코딩 결과를 저장할 output 포인터가 nil입니다")
	}

	// 1. 기본 설정값으로 초기화합니다.
	cfg := &decodingConfig{
		// 태그 설정
		tagName: "json",

		// 디코딩 동작 방식 제어
		weaklyTypedInput: true,
		errorUnused:      false,
		squash:           true,
		zeroFields:       false,
		trimSpace:        true,
	}

	// 2. 사용자 정의 옵션 적용
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}

	// 3. ZeroFields 처리 (필요 시 구조체 초기화)
	// output 포인터가 가리키는 값을 Zero Value로 덮어씌워 초기화합니다.
	if cfg.zeroFields {
		var zero T
		*output = zero
	}

	// 4. mapstructure.DecoderConfig 생성
	msConfig := &mapstructure.DecoderConfig{
		// 디코딩 대상 설정
		Result: output,

		// 태그 설정
		TagName: cfg.tagName,

		// 디코딩 동작 방식 제어
		WeaklyTypedInput: cfg.weaklyTypedInput,
		ErrorUnused:      cfg.errorUnused,
		Squash:           cfg.squash,

		// 확장 기능
		Metadata:   cfg.metadata,
		MatchName:  cfg.matchName,
		DecodeHook: cfg.buildDecodeHook(), // 훅 체인 조립
	}

	decoder, err := mapstructure.NewDecoder(msConfig)
	if err != nil {
		return err
	}

	if err := decoder.Decode(input); err != nil {
		return fmt.Errorf("입력 데이터를 %T(으)로 디코딩하는 데 실패했습니다: %w", output, err)
	}

	return nil
}

// decodingConfig 디코딩에 필요한 모든 옵션을 한곳에 모아 관리하는 비공개 설정 구조체입니다.
//
// 사용자가 Option 함수(예: WithTagName, WithSquash 등)를 통해 전달한 값들이 이곳에 저장되며,
// Decode 함수가 실행될 때 이 설정을 참조하여 동작 방식을 결정합니다.
type decodingConfig struct {
	// 태그 설정
	tagName string // Go 표준 json 패키지와 호환성을 위해 "json" 태그 사용

	// 디코딩 동작 방식 제어
	weaklyTypedInput bool // 유연한 타입 변환 허용 (예: string -> int)
	errorUnused      bool // 알 수 없는 필드 무시 (유연성 확보)
	squash           bool // 임베디드 구조체 평탄화 지원 (Composition 패턴)
	zeroFields       bool // 기본적으로는 덮어쓰기/병합 모드
	trimSpace        bool // 기본적으로 문자열 슬라이스 변환 시 공백 제거 (Backward Compatibility)

	// 확장 기능
	metadata   *mapstructure.Metadata
	matchName  func(key, field string) bool
	extraHooks []mapstructure.DecodeHookFunc
}

// buildDecodeHook 설정된 옵션을 기반으로 최적화된 mapstructure.DecodeHookFunc를 생성하여 반환합니다.
//
// 핵심 특징:
//  1. Thread-Safe: 전역 변수를 쓰지 않고 매 호출마다 독립적인 훅 체인을 구성합니다.
//  2. 실행 순서: [사용자 정의 훅] -> [기본 내장 훅 (IP, Time, Base64, Slice ...)] 순으로 실행됩니다.
//     즉, 사용자가 추가한 로직이 가장 높은 우선순위를 가집니다.
func (c *decodingConfig) buildDecodeHook() mapstructure.DecodeHookFunc {
	// 슬라이스 재할당 오버헤드를 줄이기 위해 필요한 용량을 미리 확보합니다.
	// 용량 = [사용자 정의 훅 개수] + [기본 내장 훅 4개]
	// 기본 훅 목록: TextUnmarshaller, TimeDuration, StringToBytes, StringToSlice
	expectedSize := len(c.extraHooks) + 4
	hooks := make([]mapstructure.DecodeHookFunc, 0, expectedSize)

	// 1. User Custom Hooks (우선순위를 높게 둠)
	if len(c.extraHooks) > 0 {
		hooks = append(hooks, c.extraHooks...)
	}

	// 2. Default Hooks (기본 제공 기능)
	hooks = append(hooks,
		mapstructure.TextUnmarshallerHookFunc(), // unmarshal interface 지원
		stringToDurationHookFunc(),              // "10s" -> time.Duration (strict type check, does not support aliases)
		stringToBytesHookFunc(),                 // Base64 string -> []byte
		stringToSliceHookFunc(c.trimSpace),      // "a,b" -> []string (Configurable Trim)
	)

	return mapstructure.ComposeDecodeHookFunc(hooks...)
}

// Option 디코딩 설정을 커스터마이징하기 위한 함수형 옵션 타입입니다.
type Option func(*decodingConfig)

// WithTagName 구조체 필드 매핑에 사용할 태그 이름을 지정합니다. (기본값: "json")
//
// "json" 대신 "yaml", "toml" 등의 태그를 기준으로 매핑하려면 이 옵션을 사용하십시오.
func WithTagName(tagName string) Option {
	return func(c *decodingConfig) {
		c.tagName = tagName
	}
}

// WithWeaklyTypedInput 타입이 달라도 가능한 경우 자동으로 변환(Weakly Typed)할지 설정합니다. (기본값: true)
//
// 예시 (활성화 시):
//   - "123" (string) -> 123 (int) : 문자열을 숫자로 자동 변환
//   - 1 (int) -> true (bool) : 0이 아닌 숫자를 true로 변환
//   - []string{"val"} -> "val" (string) : 단일 요소 슬라이스를 값으로 변환
func WithWeaklyTypedInput(enable bool) Option {
	return func(c *decodingConfig) {
		c.weaklyTypedInput = enable
	}
}

// WithErrorUnused 대상 구조체에 없는 필드가 입력 데이터에 존재할 경우, 무시하지 않고 에러를 발생시킵니다. (기본값: false)
//
// 오타 등으로 인해 의도치 않게 설정이 누락되는 것을 방지하기 위해 엄격한 검증이 필요할 때 유용합니다.
func WithErrorUnused(enable bool) Option {
	return func(c *decodingConfig) {
		c.errorUnused = enable
	}
}

// WithSquash 임베디드 구조체(Embedded Struct)를 평탄화(Flattening)하여 처리할지 설정합니다. (기본값: true)
//
// true로 설정하면, 중첩된 맵 구조를 따르지 않고 상위 레벨의 맵 필드를 임베디드 구조체의 필드에 직접 매핑합니다.
func WithSquash(enable bool) Option {
	return func(c *decodingConfig) {
		c.squash = enable
	}
}

// WithDecodeHook 기본 제공되는 변환 로직 외에 사용자 정의 변환 훅(Hook)을 추가합니다.
//
// 여러 개의 훅을 추가할 수 있으며, 추가된 훅들은 기본 훅보다 먼저 실행되어 커스텀 변환 로직을 우선 적용합니다.
func WithDecodeHook(hooks ...mapstructure.DecodeHookFunc) Option {
	return func(c *decodingConfig) {
		c.extraHooks = append(c.extraHooks, hooks...)
	}
}

// WithMetadata 디코딩 과정에서 사용된 키(Keys)나 사용되지 않은 키(Unused) 등의 메타데이터를 수집합니다.
//
// 디코딩 결과에 대한 상세한 분석이 필요할 때 유용합니다.
func WithMetadata(md *mapstructure.Metadata) Option {
	return func(c *decodingConfig) {
		c.metadata = md
	}
}

// WithMatchName 필드 이름과 입력 키를 매칭하는 로직을 커스터마이징합니다.
//
// 기본 동작은 대소문자를 구분하지 않고 매칭하는 것입니다. 이를 변경하여 대소문자를 구분하거나 특수한 매칭 규칙을 적용할 수 있습니다.
func WithMatchName(matchFunc func(mapKey, fieldName string) bool) Option {
	return func(c *decodingConfig) {
		c.matchName = matchFunc
	}
}

// WithZeroFields 디코딩 전에 대상 구조체의 모든 필드를 제로 값(Zero Value)으로 초기화할지 설정합니다.
//
// true로 설정하면, 구조체에 미리 설정된 기본값들을 모두 지우고 입력 데이터에 있는 값으로만 채웁니다.
// 즉, 덮어쓰기(Merge)가 아니라 교체(Replace) 방식으로 동작하게 됩니다.
func WithZeroFields(enable bool) Option {
	return func(c *decodingConfig) {
		c.zeroFields = enable
	}
}

// WithTrimSpace 콤마(,)로 구분된 문자열을 슬라이스로 변환할 때, 각 요소의 앞뒤 공백을 자동으로 제거할지 설정합니다. (기본값: true)
//
// 예: "a, b" 입력 시
//   - true(활성, 기본값): ["a", "b"] (깔끔한 데이터)
//   - false(비활성): ["a", " b"] (공백 유지)
func WithTrimSpace(enable bool) Option {
	return func(c *decodingConfig) {
		c.trimSpace = enable
	}
}
