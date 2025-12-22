package maputil

import "github.com/mitchellh/mapstructure"

// Decode `map[string]interface{}` 형태의 데이터를 지정된 구조체(`d`)로 디코딩합니다.
// 내부적으로 `mapstructure` 패키지를 사용하여 JSON 마샬링/언마샬링 방식보다 더 높은 성능과 유연성을 제공합니다.
//
// [주요 특징]
// 1. JSON 태그 호환: 구조체 필드에 `json` 태그가 정의되어 있다면 이를 우선적으로 사용합니다.
// 2. 유연한 타입 변환 (Weak Type Conversion):
//   - 문자열로 된 숫자를 정수형으로 변환 (예: "100" -> 100)
//   - 문자열 "true"/"false"를 불리언으로 변환
//   - 단일 값을 슬라이스로 자동 변환하는 등 유연한 입력을 허용합니다.
//
// [주의]
// 매개변수 `d`는 반드시 구조체의 '포인터'여야 합니다. (그렇지 않을 경우 에러가 반환되거나 변경 사항이 반영되지 않습니다.)
func Decode(d interface{}, m map[string]interface{}) error {
	config := &mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           d,
		TagName:          "json", // 기존 json 태그 호환
		WeaklyTypedInput: true,   // 유연한 타입 변환 지원 (예: string "123" -> int 123)
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}

	return decoder.Decode(m)
}
