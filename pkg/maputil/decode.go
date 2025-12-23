// Package maputil 맵(Map) 데이터 처리 및 구조체 변환을 위한 유틸리티 기능을 제공합니다.
package maputil

import (
	"encoding"
	"reflect"

	"github.com/mitchellh/mapstructure"
)

// Decode 맵(`map[string]any`) 또는 일반 인터페이스 데이터를 지정된 타입(`T`)의 구조체로 디코딩하여 반환합니다.
//
// 내부적으로 `mapstructure` 라이브러리를 사용하여 리플렉션 기반의 디코딩을 수행합니다.
// 이 함수는 JSON 마샬링/언마샬링 방식(map -> json bytes -> struct)보다 오버헤드가 적고,
// 타입 변환에 있어 훨씬 더 유연한 처리를 지원합니다.
//
// [주의사항]
//  1. `T`는 주로 구조체(struct) 타입이어야 합니다.
//  2. `mapstructure`의 특성상 구조체의 비공개 필드(Unexported Fields)는 디코딩 대상에서 제외됩니다.
func Decode[T any](input any) (*T, error) {
	output := new(T)

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
			mapstructure.StringToTimeDurationHookFunc(), // "10s" -> time.Duration
			mapstructure.StringToSliceHookFunc(","),     // "a,b,c" -> []string
			textUnmarshalerHookFunc(),                   // encoding.TextUnmarshaler 구현체(예: net.IP, url.URL) 자동 변환
		),
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return nil, err
	}

	if err := decoder.Decode(input); err != nil {
		return nil, err
	}

	return output, nil
}

// textUnmarshalerType encoding.TextUnmarshaler 인터페이스의 리플렉션 타입 정보를 캐싱합니다.
//
// [최적화 노트]
// 훅 함수가 호출될 때마다 reflect.TypeOf(...)를 수행하는 오버헤드를 줄이기 위해
// 패키지 레벨 변수로 한 번만 초기화하여 재사용합니다. 이를 통해 대량의 필드 디코딩 시 성능을 확보합니다.
var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

// textUnmarshalerHookFunc Go 표준 encoding.TextUnmarshaler 인터페이스를 지원하는 디코딩 훅을 반환합니다.
//
// [기능 설명]
// mapstructure는 기본적으로 복잡한 타입(구조체 등)에 대한 문자열 디코딩을 지원하지 않습니다.
// 이 훅은 문자열 데이터가 주어졌을 때, 대상 타입이 UnmarshalText 메서드를 구현하고 있다면
// 이를 자동으로 호출하여 해당 타입으로 안전하게 변환해줍니다.
//
// [지원 예시]
//   - net.IP ("127.0.0.1" -> net.IP{...})
//   - time.Time (RFC3339 문자열 -> time.Time 구조체)
//   - url.URL (및 이를 임베딩하거나 감싼 사용자 정의 타입)
func textUnmarshalerHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		if f.Kind() != reflect.String {
			return data, nil
		}

		strData := reflect.ValueOf(data).String()

		// Case 1: T가 포인터이고, T 자체가 TextUnmarshaler를 구현하는 경우
		// 예: *url.URL
		if t.Kind() == reflect.Ptr && t.Implements(textUnmarshalerType) {
			// T가 *url.URL이면 t.Elem()은 url.URL
			// reflect.New(t.Elem())은 *url.URL (초기화된 값, 예: &url.URL{})
			val := reflect.New(t.Elem())

			// 인터페이스 캐스팅 및 호출
			u := val.Interface().(encoding.TextUnmarshaler)
			if err := u.UnmarshalText([]byte(strData)); err != nil {
				return nil, err
			}
			return val.Interface(), nil
		}

		// Case 2: *T가 TextUnmarshaler를 구현하는 경우
		if reflect.PointerTo(t).Implements(textUnmarshalerType) {
			val := reflect.New(t)
			u, ok := val.Interface().(encoding.TextUnmarshaler)
			if !ok {
				return data, nil // Should not happen if Implements is true
			}
			if err := u.UnmarshalText([]byte(strData)); err != nil {
				return nil, err
			}
			return val.Elem().Interface(), nil
		}

		return data, nil
	}
}
