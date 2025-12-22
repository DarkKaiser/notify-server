package maputil

import "github.com/mitchellh/mapstructure"

// Decode 맵(`map[string]any`) 데이터를 Go 구조체(`output`)로 디코딩합니다.
//
// 내부적으로 `mapstructure` 라이브러리를 사용하여 리플렉션 기반의 디코딩을 수행합니다.
// 이 함수는 JSON 마샬링/언마샬링 방식(map -> json bytes -> struct)보다 오버헤드가 적고,
// 타입 변환에 있어 훨씬 더 유연한 처리를 지원합니다.
//
// [주의사항]
//  1. 매개변수 `output`은 반드시 구조체의 포인터(Pointer to Struct)여야 합니다.
//     값 타입(Value Type)이나 nil을 전달할 경우 에러가 반환되거나 디코딩 결과가 반영되지 않습니다.
//  2. `mapstructure`의 특성상 구조체의 비공개 필드(Unexported Fields)는 디코딩 대상에서 제외됩니다.
func Decode(input map[string]any, output any) error {
	config := &mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           output,
		TagName:          "json", // 기존 json 태그 호환
		WeaklyTypedInput: true,   // 유연한 타입 변환 지원 (예: string "123" -> int 123)
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}

	return decoder.Decode(input)
}
