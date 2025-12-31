package maputil

import (
	"fmt"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Documentation Examples (GoDoc)
// =============================================================================

type config struct {
	Host    string        `json:"host"`
	Port    int           `json:"port"`
	Debug   bool          `json:"debug"`
	Timeout time.Duration `json:"timeout"` // "10s" -> time.Duration 자동 변환 지원
	Tags    []string      `json:"tags"`    // "a,b,c" -> []string 자동 변환 지원
}

func ExampleDecode() {
	// 외부 입력 (예: JSON 파싱 결과 또는 설정 파일)
	input := map[string]any{
		"host":    "localhost",
		"port":    8080,
		"debug":   true,
		"timeout": "5s",
		"tags":    "server,http,api",
		// "unknown": "value", // 기본적으로 정의되지 않은 필드가 있으면 에러가 발생합니다.
	}

	// 맵 데이터를 구조체로 디코딩
	cfg, err := Decode[config](input)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Host: %s, Port: %d, Timeout: %s, Tags: %v\n", cfg.Host, cfg.Port, cfg.Timeout, cfg.Tags)

	// Output:
	// Host: localhost, Port: 8080, Timeout: 5s, Tags: [server http api]
}

func ExampleDecodeTo() {
	// 1. 기본값(Default) 설정
	cfg := &config{
		Host:    "127.0.0.1",
		Port:    9000,
		Debug:   false,
		Timeout: 30 * time.Second,
	}

	// 2. 오버라이드할 데이터 (예: 환경 변수, 설정 파일 등)
	// 일부 필드만 존재할 수 있습니다.
	override := map[string]any{
		"port":  3000,
		"debug": true,
	}

	// 3. 기존 객체(cfg)에 오버라이드 값 병합(Merge)
	// 참고: DecodeTo는 포인터를 전달받으므로 cfg 자체가 수정됩니다.
	if err := DecodeTo(override, cfg); err != nil {
		panic(err)
	}

	fmt.Printf("Host: %s, Port: %d, Debug: %v\n", cfg.Host, cfg.Port, cfg.Debug)

	// Output:
	// Host: 127.0.0.1, Port: 3000, Debug: true
}

// =============================================================================
// Unit Tests - Test Data Structures
// =============================================================================

type basicStruct struct {
	Name      string `json:"name"`
	Age       int    `json:"age"`
	IsEnabled bool   `json:"is_enabled"`
}

type nestedStruct struct {
	Title  string      `json:"title"`
	Detail basicStruct `json:"detail"`
}

type sliceMapStruct struct {
	Tags   []string       `json:"tags"`
	Config map[string]int `json:"config"`
}

type pointerStruct struct {
	Value *int    `json:"value"`
	Data  *string `json:"data"`
}

// CustomText는 encoding.TextUnmarshaler를 구현하여 커스텀 파싱 로직을 테스트합니다.
type customText struct {
	Value string
}

func (c *customText) UnmarshalText(text []byte) error {
	c.Value = "parsed:" + string(text)
	return nil
}

type hookStruct struct {
	IP     net.IP      `json:"ip"`
	Custom *customText `json:"custom"`
}

// =============================================================================
// Unit Tests - Decode
// =============================================================================

func TestDecode_StructureMapping(t *testing.T) {
	t.Parallel()

	// 1. 기본 구조체 매핑 테스트
	t.Run("BasicStruct", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name    string
			input   map[string]any
			want    basicStruct
			wantErr bool
		}{
			{
				name: "Normal Case",
				input: map[string]any{
					"name":       "Alice",
					"age":        30,
					"is_enabled": true,
				},
				want: basicStruct{Name: "Alice", Age: 30, IsEnabled: true},
			},
			{
				name: "Typo in Key (ErrorUnused=true)",
				input: map[string]any{
					"name_typo": "Alice",
				},
				wantErr: true,
			},
			{
				name: "Partial Fields",
				input: map[string]any{
					"name": "Bob",
				},
				want: basicStruct{Name: "Bob"},
			},
			{
				name:    "Nil Input",
				input:   nil,
				want:    basicStruct{}, // zero value
				wantErr: false,         // mapstructure treats nil as empty map
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := Decode[basicStruct](tt.input)
				if tt.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					assert.Equal(t, tt.want, *got)
				}
			})
		}
	})

	// 2. 중첩 구조체 매핑 테스트
	t.Run("NestedStruct", func(t *testing.T) {
		t.Parallel()
		input := map[string]any{
			"title": "Nested Test",
			"detail": map[string]any{
				"name": "Bob",
				"age":  25,
			},
		}
		got, err := Decode[nestedStruct](input)
		require.NoError(t, err)
		assert.Equal(t, "Nested Test", got.Title)
		assert.Equal(t, "Bob", got.Detail.Name)
		assert.Equal(t, 25, got.Detail.Age)
	})

	// 3. 슬라이스 및 맵 매핑 테스트
	t.Run("SliceAndMap", func(t *testing.T) {
		t.Parallel()
		input := map[string]any{
			"tags": "go,json,map", // StringToSliceHook 자동 적용 테스트
			"config": map[string]any{
				"timeout": 100,
			},
		}
		got, err := Decode[sliceMapStruct](input)
		require.NoError(t, err)
		assert.Equal(t, []string{"go", "json", "map"}, got.Tags)
		assert.Equal(t, map[string]int{"timeout": 100}, got.Config)
	})

	// 4. 포인터 필드 테스트
	t.Run("PointerFields", func(t *testing.T) {
		t.Parallel()
		val := 123
		input := map[string]any{
			"value": val,
			"data":  nil,
		}
		got, err := Decode[pointerStruct](input)
		require.NoError(t, err)
		assert.NotNil(t, got.Value)
		assert.Equal(t, 123, *got.Value)
		assert.Nil(t, got.Data)
	})
}

func TestDecode_Hooks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   map[string]any
		assert  func(t *testing.T, got *hookStruct)
		wantErr bool
	}{
		{
			name: "TextUnmarshaler (net.IP)",
			input: map[string]any{
				"ip": "192.168.1.1",
			},
			assert: func(t *testing.T, got *hookStruct) {
				assert.Equal(t, net.ParseIP("192.168.1.1"), got.IP)
			},
		},
		{
			name: "Custom UnmarshalText",
			input: map[string]any{
				"custom": "test-data",
			},
			assert: func(t *testing.T, got *hookStruct) {
				require.NotNil(t, got.Custom)
				assert.Equal(t, "parsed:test-data", got.Custom.Value)
			},
		},
		{
			name: "Invalid IP Format",
			input: map[string]any{
				"ip": "invalid-ip", // net.IP UnmarshalText fails
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decode[hookStruct](tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.assert != nil {
					tt.assert(t, got)
				}
			}
		})
	}
}

func TestDecode_Options(t *testing.T) {
	t.Parallel()

	// 1. WithTagName
	t.Run("WithTagName", func(t *testing.T) {
		type yamlStruct struct {
			MyName string `yaml:"the_name"`
		}
		input := map[string]any{"the_name": "YAML Config"}

		// 기본값("json")으로는 실패해야 함 (ErrorUnused=true)
		_, err := Decode[yamlStruct](input)
		require.Error(t, err)

		// WithTagName("yaml") 적용 시 성공
		got, err := Decode[yamlStruct](input, WithTagName("yaml"))
		require.NoError(t, err)
		assert.Equal(t, "YAML Config", got.MyName)
	})

	// 2. WithErrorUnused
	t.Run("WithErrorUnused", func(t *testing.T) {
		input := map[string]any{
			"name":        "Valid",
			"unknown_key": "Ignored",
		}

		// 기본값(true) -> 에러
		_, err := Decode[basicStruct](input)
		require.Error(t, err)

		// false -> 에러 무시
		got, err := Decode[basicStruct](input, WithErrorUnused(false))
		require.NoError(t, err)
		assert.Equal(t, "Valid", got.Name)
	})

	// 3. WithWeaklyTypedInput
	t.Run("WithWeaklyTypedInput", func(t *testing.T) {
		input := map[string]any{"age": "42"} // string -> int

		// 기본값(true) -> 성공
		got, err := Decode[basicStruct](input)
		require.NoError(t, err)
		assert.Equal(t, 42, got.Age)

		// false -> 실패
		_, err = Decode[basicStruct](input, WithWeaklyTypedInput(false))
		require.Error(t, err)
	})

	// 4. WithMetadata
	t.Run("WithMetadata", func(t *testing.T) {
		input := map[string]any{
			"name":    "Meta",
			"unknown": 1,
		}
		var md mapstructure.Metadata
		_, err := Decode[basicStruct](input, WithMetadata(&md), WithErrorUnused(false))
		require.NoError(t, err)
		assert.Contains(t, md.Unused, "unknown")
		assert.Contains(t, md.Keys, "name")
	})

	// 5. WithDecodeHook
	t.Run("WithDecodeHook", func(t *testing.T) {
		type customHookStruct struct {
			Value string `json:"value"`
		}
		// int(123) -> string("one-two-three") 변환 훅
		// 기존 훅들(StringToSlice 등)과 충돌하지 않도록 Int -> String 변환을 테스트
		intToStringHook := func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
			if f.Kind() == reflect.Int && t.Kind() == reflect.String {
				if val, ok := data.(int); ok && val == 123 {
					return "one-two-three", nil
				}
			}
			return data, nil
		}

		input := map[string]any{"value": 123}
		got, err := Decode[customHookStruct](input, WithDecodeHook(intToStringHook))
		require.NoError(t, err)
		assert.Equal(t, "one-two-three", got.Value)
	})
}

// =============================================================================
// Unit Tests - DecodeTo
// =============================================================================

func TestDecodeTo_Functionality(t *testing.T) {
	t.Parallel()

	// 1. 기본 Merge 동작 확인
	t.Run("MergeBehavior", func(t *testing.T) {
		target := &basicStruct{
			Name: "Original",
			Age:  10,
		}
		input := map[string]any{
			"age": 20, // Name은 유지되고 Age만 변경되어야 함
		}
		err := DecodeTo(input, target)
		require.NoError(t, err)
		assert.Equal(t, "Original", target.Name)
		assert.Equal(t, 20, target.Age)
	})

	// 2. WithZeroFields 확인 (덮어쓰기 전 초기화)
	t.Run("WithZeroFields", func(t *testing.T) {
		target := &basicStruct{
			Name:      "Original",
			Age:       10,
			IsEnabled: true,
		}
		input := map[string]any{
			"name": "New", // Age와 IsEnabled는 초기화되어야 함
		}
		err := DecodeTo(input, target, WithZeroFields(true))
		require.NoError(t, err)
		assert.Equal(t, "New", target.Name)
		assert.Equal(t, 0, target.Age)
		assert.False(t, target.IsEnabled)
	})

	// 3. 잘못된 아웃풋 타입 처리
	t.Run("InvalidOutput", func(t *testing.T) {
		// Output must be a pointer
		// 그러나 DecodeTo[T] 제네릭 함수 특성상 T는 컴파일 타임에는 any가 될 수 있어도,
		// 내부적으로 mapstructure.DecoderConfig.Result에는 포인터가 들어가야 함.
		// DecodeTo의 시그니처는 `output *T`이므로 항상 포인터임이 보장됨.
		// 따라서 nil 포인터를 넘기는 케이스를 테스트.

		var target *basicStruct = nil // nil pointer
		input := map[string]any{"name": "test"}

		// mapstructure는 Result가 nil이거나 포인터가 아니면 에러를 반환
		// 하지만 우리는 *T를 넘기는데 그 값이 nil인 경우.
		// mapstructure 내부에서 reflect.ValueOf(Result).Elem() 할 때 panic 가능성이 있음
		// 또는 라이브러리가 에러를 리턴.
		// DecodeTo 구현을 보면 `Result: output`으로 설정함. output이 nil이면 Result도 nil.
		err := DecodeTo(input, target)

		// mapstructure behavior: Result must be a pointer to a struct...
		// If Result is nil, it usually errors "result must be a pointer".
		require.Error(t, err)
	})
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkDecode_SmallStruct(b *testing.B) {
	input := map[string]any{
		"name":       "Benchmark",
		"age":        123,
		"is_enabled": true,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Decode[basicStruct](input)
	}
}

func BenchmarkDecode_NestedStruct(b *testing.B) {
	input := map[string]any{
		"title": "Nested Benchmark",
		"detail": map[string]any{
			"name": "Inner",
			"age":  99,
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Decode[nestedStruct](input)
	}
}

func BenchmarkDecodeTo_Reuse(b *testing.B) {
	input := map[string]any{
		"name": "Update",
		"age":  50,
	}
	target := &basicStruct{
		Name: "Original",
		Age:  0,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DecodeTo(input, target)
	}
}
