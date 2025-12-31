// Package maputil 맵(Map) 데이터 처리 및 구조체 변환을 위한 유틸리티 기능을 제공합니다.
package maputil

import (
	"github.com/mitchellh/mapstructure"
)

// Decode 맵 또는 인터페이스 데이터를 지정된 타입(`T`)의 구조체로 디코딩하여 반환합니다.
//
// 내부적으로 `mapstructure` 라이브러리를 사용하여 리플렉션 기반의 디코딩을 수행합니다.
// 새로운 구조체 인스턴스를 생성하여 반환하므로, 기존 값을 덮어쓰지 않고 온전히 새로운 객체가 필요할 때 사용합니다.
func Decode[T any](input any) (*T, error) {
	output := new(T)
	if err := DecodeTo(input, output); err != nil {
		return nil, err
	}
	return output, nil
}

// DecodeTo 맵 또는 인터페이스 데이터를 주어진 대상 객체(`output`)로 디코딩합니다.
func DecodeTo(input any, output any) error {
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

		// 기본 변환 로직 외에 추가적인 타입 변환 규칙을 정의합니다.
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.TextUnmarshallerHookFunc(),     // encoding.TextUnmarshaler 지원 (Built-in)
			mapstructure.StringToTimeDurationHookFunc(), // "10s" -> time.Duration
			mapstructure.StringToSliceHookFunc(","),     // "a,b,c" -> []string
		),
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}

	return decoder.Decode(input)
}
