package maputil

import "github.com/mitchellh/mapstructure"

// Decode 맵(`map[string]any`) 데이터를 지정된 타입(`T`)의 구조체로 디코딩하여 반환합니다.
//
// 내부적으로 `mapstructure` 라이브러리를 사용하여 리플렉션 기반의 디코딩을 수행합니다.
// 이 함수는 JSON 마샬링/언마샬링 방식(map -> json bytes -> struct)보다 오버헤드가 적고,
// 타입 변환에 있어 훨씬 더 유연한 처리를 지원합니다.
//
// [주의사항]
//  1. `T`는 주로 구조체(struct) 타입이어야 합니다.
//  2. `mapstructure`의 특성상 구조체의 비공개 필드(Unexported Fields)는 디코딩 대상에서 제외됩니다.
func Decode[T any](input map[string]any) (*T, error) {
	output := new(T)

	config := &mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           output,
		TagName:          "json", // 기존 json 태그 호환
		WeaklyTypedInput: true,   // 유연한 타입 변환 지원 (예: string "123" -> int 123)
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
