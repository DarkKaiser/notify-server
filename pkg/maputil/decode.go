// Package maputil 맵(Map) 데이터 처리 및 구조체 변환을 위한 유틸리티 기능을 제공합니다.
package maputil

import (
	"github.com/mitchellh/mapstructure"
)

// Decode 맵 또는 인터페이스 데이터를 지정된 타입(`T`)의 구조체로 디코딩하여 반환합니다.
//
// 내부적으로 `mapstructure` 라이브러리를 사용하여 리플렉션 기반의 디코딩을 수행합니다.
// 기본 설정만으로 충분한 경우 인자 없이 호출할 수 있으며, 필요 시 옵션을 가변 인자로 전달하여 동작을 제어할 수 있습니다.
//
// 주의: 기본 설정으로 `ErrorUnused: true`가 적용되어 있습니다.
// 텔레그램 웹훅 등 외부 서비스의 입력을 처리할 때 구조체에 정의되지 않은 필드가 포함되어 있으면 에러가 발생할 수 있습니다.
// 외부 입력을 처리할 때는 반드시 `WithErrorUnused(false)` 옵션을 사용하여 불필요한 필드를 무시하도록 설정하십시오.
func Decode[T any](input any, opts ...Option) (*T, error) {
	output := new(T)
	if err := DecodeTo(input, output, opts...); err != nil {
		return nil, err
	}
	return output, nil
}

// DecodeTo 맵 또는 인터페이스 데이터를 지정된 타입(`T`)의 구조체로 디코딩합니다.
//
// output 인자는 반드시 구조체 포인터여야 합니다.
// 제네릭을 사용하여 컴파일 타임에 포인터 여부를 강제합니다.
//
// 주의: 기본 설정으로 `ErrorUnused: true`가 적용되어 있습니다.
// 외부 입력을 처리할 때는 `WithErrorUnused(false)` 사용을 권장합니다.
func DecodeTo[T any](input any, output *T, opts ...Option) error {
	config := &mapstructure.DecoderConfig{
		Metadata: nil,

		Result: output,

		// 구조체 태그 이름을 지정합니다.
		// Go 표준 라이브러리의 `encoding/json`과 호환성을 유지하기 위해 "json" 태그를 사용합니다.
		TagName: "json",

		// 입력 데이터의 타입이 대상 필드의 타입과 정확히 일치하지 않아도 유연하게 변환을 시도합니다.
		// 예: "123"(string) -> 123(int), 1(int) -> true(bool)
		WeaklyTypedInput: true,

		// 구조체에 정의되지 않은 필드가 입력 맵에 존재할 경우 에러를 반환합니다.
		// 이는 설정 파일의 오타(Typos)나 불필요한 필드를 감지하여 잠재적인 설정 오류를 방지하는 안전장치 역할을 합니다.
		ErrorUnused: true,

		// 임베디드 구조체(Embedded Struct)에 대한 평탄화(Flattening)를 지원합니다.
		// 상위 맵의 필드가 임베디드 구조체의 필드로 직접 매핑되도록 하여 Composition 패턴을 원활하게 지원합니다.
		Squash: true,

		// 기본적으로 수행할 타입 변환 로직(Hook)들을 정의합니다.
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.TextUnmarshallerHookFunc(),     //사용자 정의 타입의 UnmarshalText 메서드 지원
			mapstructure.StringToTimeDurationHookFunc(), // "10s", "1h" 등 시간 문자열을 time.Duration으로 변환
			mapstructure.StringToSliceHookFunc(","),     //"a,b,c" 형태의 구분자 문자열을 []string으로 자동 분할
		),
	}

	// 사용자 정의 옵션 적용
	for _, opt := range opts {
		opt(config)
	}

	// mapstructure의 `ZeroFields` 옵션이 특정 상황(예: 일부 중첩 필드)에서 불완전하게 동작할 위험이 있습니다.
	// 이를 보완하기 위해 제네릭을 사용하여 타입 안전하고 빠르게 구조체를 초기화합니다.
	// 이는 리플렉션을 사용하는 것보다 성능상 이점이 있으며 코드가 더 간결합니다.
	if config.ZeroFields {
		var zero T
		*output = zero
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}

	return decoder.Decode(input)
}

// Option 디코딩 설정을 커스터마이징하기 위한 함수형 옵션(Functional Option) 타입입니다.
//
// 이 타입을 활용하여 기본 설정을 변경하거나(예: WithTagName),
// 새로운 동작을 주입(예: WithDecodeHook)하는 등 디코더의 행위를 유연하게 제어할 수 있습니다.
type Option func(*mapstructure.DecoderConfig)

// WithTagName 구조체 필드 매핑에 사용할 태그 이름을 설정합니다. (기본값: "json")
//
// 기본 동작은 "json" 태그를 따르지만, 다른 태그(예: "yaml", "toml", "structure")를
// 사용하는 구조체와 호환성을 맞추거나 사용자 정의 태그를 사용할 때 유용합니다.
func WithTagName(tagName string) Option {
	return func(c *mapstructure.DecoderConfig) {
		c.TagName = tagName
	}
}

// WithWeaklyTypedInput 입력 값의 타입이 대상 필드 타입과 다를 때 유연한 변환(Weakly Typed)을 허용할지 설정합니다. (기본값: true)
//
// 예: "123"(string) -> 123(int), 1(int) -> true(bool)
// 이 옵션을 false로 설정하면 타입이 정확히 일치해야 하며, 그렇지 않을 경우 디코딩 에러가 발생합니다.
func WithWeaklyTypedInput(enable bool) Option {
	return func(c *mapstructure.DecoderConfig) {
		c.WeaklyTypedInput = enable
	}
}

// WithErrorUnused 대상 구조체에 없는 필드가 입력 데이터에 존재할 경우 에러를 반환할지 설정합니다. (기본값: true)
//
// true로 설정 시(기본값), 오타로 인한 설정 누락이나 불필요한 데이터를 감지하여 안정성을 높일 수 있습니다.
// 하위 호환성 유지나 부분적인 데이터 매핑이 필요한 경우 false로 설정하여 무시할 수 있습니다.
func WithErrorUnused(enable bool) Option {
	return func(c *mapstructure.DecoderConfig) {
		c.ErrorUnused = enable
	}
}

// WithDecodeHook 기본 변환 로직 외에 사용자 정의 변환 로직(Decode Hook)을 추가합니다.
//
// 기존에 등록된 훅(TextUnmarshaler 등)은 유지되며, 전달된 훅들이 추가로 실행되도록 구성(Compose)됩니다.
// 특수한 타입 변환이 필요하거나 비표준 데이터 포맷을 처리할 때 사용합니다.
func WithDecodeHook(hooks ...mapstructure.DecodeHookFunc) Option {
	return func(c *mapstructure.DecoderConfig) {
		if c.DecodeHook != nil {
			// 기존 훅이 있다면 함께 실행되도록 구성
			c.DecodeHook = mapstructure.ComposeDecodeHookFunc(c.DecodeHook, mapstructure.ComposeDecodeHookFunc(hooks...))
		} else {
			c.DecodeHook = mapstructure.ComposeDecodeHookFunc(hooks...)
		}
	}
}

// WithMetadata 디코딩 과정의 메타데이터(사용되지 않은 키, 디코딩된 키 등)를 수집합니다.
//
// 이 옵션을 사용하면 구조체에 정의되지 않은 입력 필드가 무엇인지 파악하거나(Unused),
// 실제 데이터가 매핑된 구조를 추적할 수 있습니다. 스크래핑 데이터의 스키마 변경 감지에 유용합니다.
func WithMetadata(md *mapstructure.Metadata) Option {
	return func(c *mapstructure.DecoderConfig) {
		c.Metadata = md
	}
}

// WithZeroFields 디코딩 전에 대상 구조체의 모든 필드를 제로 값(Zero Value)으로 초기화할지 설정합니다.
//
// 이 옵션을 사용하면(`true`), `DecodeTo`를 호출할 때 기존 구조체의 값이 모두 지워지고
// 입력 데이터에 있는 값만 새로 채워집니다. (기존 데이터 잔존 방지)
// `Decode` 함수는 항상 새로운 구조체를 생성하므로 이 옵션의 영향이 적지만,
// 구조체 필드에 기본값이 설정된 경우에는 유효할 수 있습니다.
func WithZeroFields(enable bool) Option {
	return func(c *mapstructure.DecoderConfig) {
		c.ZeroFields = enable
	}
}
