package maputil

import (
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
)

// stringToBytesHookFunc 문자열을 []byte로 변환하는 훅입니다.
// "base64:" 접두사가 있는 경우에만 Base64로 디코딩하며, 그 외에는 원본 문자열의 바이트를 반환합니다.
func stringToBytesHookFunc() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data any) (any, error) {
		if f.Kind() != reflect.String {
			return data, nil
		}
		// 타겟이 []byte (Slice) 또는 [N]byte (Array)인지 확인
		if t.Kind() != reflect.Slice && t.Kind() != reflect.Array {
			return data, nil
		}
		if t.Elem().Kind() != reflect.Uint8 {
			return data, nil
		}

		s := reflect.ValueOf(data).String()

		// 앞뒤 공백을 제거하여 "  base64:..." 같은 케이스도 처리
		s = strings.TrimSpace(s)

		// 의도치 않은 바이너리 디코딩("user" -> broken bytes)을 방지하기 위해
		// 반드시 접두사가 있어야만 Base64로 처리합니다.
		const prefix = "base64:"
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimPrefix(s, prefix)
			if decoded, err := base64.StdEncoding.DecodeString(s); err == nil {
				return decoded, nil
			}
			// 사용자가 "base64:" 접두사를 통해 명시적으로 변환을 요청했으므로,
			// 디코딩에 실패할 경우 이를 무시하지 않고 에러를 반환하여 잘못된 입력임을 알립니다.
			return nil, fmt.Errorf("base64 접두사가 포함된 잘못된 문자열입니다: %w", errors.New("decoding failed"))
		}

		return []byte(s), nil
	}
}

// stringToSliceHookFunc 쉼표(,)로 구분된 문자열을 잘라서 슬라이스로 변환합니다.
//
// [중요] []byte 타입은 쪼개지 않고 원본 그대로 둡니다.
// mapstructure가 []byte를 일반 슬라이스처럼 취급하여 문자열을 분할해버리는 문제를 막기 위함입니다.
// 이를 통해 바이너리 데이터가 의도치 않게 손상되는 것을 방지합니다.
func stringToSliceHookFunc(trimSpace bool) mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data any) (any, error) {
		// 1. 입력이 문자열인지 확인
		if f.Kind() != reflect.String {
			return data, nil
		}

		// 2. 타겟이 슬라이스 또는 배열인지 확인
		if t.Kind() != reflect.Slice && t.Kind() != reflect.Array {
			return data, nil
		}

		// 3. 타겟이 []byte (alias 포함) 또는 [N]byte인 경우, 문자열 쪼개기를 수행하지 않음
		// 이는 stringToBytesHookFunc 또는 mapstructure 기본 로직에 맡김
		if t.Elem().Kind() == reflect.Uint8 {
			return data, nil
		}

		// 4. 그 외의 슬라이스 타입에 대해서는 쉼표(,)로 구분된 문자열을 슬라이스로 변환
		strData := reflect.ValueOf(data).String()
		if strData == "" {
			return []string{}, nil
		}

		parts := strings.Split(strData, ",")
		if trimSpace {
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
			}
		}
		return parts, nil
	}
}

// stringToDurationHookFunc 문자열을 time.Duration으로 변환하는 훅입니다.
//
// time.Duration의 별칭(Alias) 타입은 지원하지 않고, 오직 정확한 time.Duration 타입만 변환합니다.
// 이는 이름만 유사한 다른 정수형 타입들이 의도치 않게 시간으로 오해되어 잘못된 값으로 변환되는 것을 방지하기 위함입니다.
func stringToDurationHookFunc() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data any) (any, error) {
		if f.Kind() != reflect.String {
			return data, nil
		}

		// 타겟이 time.Duration(int64) 호환 타입인지 확인
		if t.Kind() != reflect.Int64 {
			return data, nil
		}

		// 모든 int64를 시간으로 변환하지 않도록, 이름에 기반한 불확실한 추론을 제거하고 엄격하게 타입 검사
		if t != reflect.TypeOf(time.Duration(0)) {
			return data, nil
		}

		// 안전하게 문자열 값 추출
		s := reflect.ValueOf(data).String()
		s = strings.TrimSpace(s)

		// time.ParseDuration은 "ns", "us", "ms", "s", "m", "h" 등의 단위가 필요함
		d, err := time.ParseDuration(s)
		if err != nil {
			// 파싱 실패 시, 다른 훅이나 기본 로직이 처리하도록 pass
			return data, nil
		}

		// 성공적으로 Duration으로 파싱되었다면, 타겟 타입(t)으로 변환하여 반환
		return reflect.ValueOf(d).Convert(t).Interface(), nil
	}
}
