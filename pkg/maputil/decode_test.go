package maputil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -------------------------------------------------------------------------
// Test Structures (테스트용 구조체 정의)
// -------------------------------------------------------------------------

type BasicStruct struct {
	Name      string `json:"name"`
	Age       int    `json:"age"`
	IsEnabled bool   `json:"is_enabled"`
}

type NestedStruct struct {
	Title  string      `json:"title"`
	Detail BasicStruct `json:"detail"`
}

type PointerStruct struct {
	Value *int    `json:"value"`
	Data  *string `json:"data"`
}

type SliceMapStruct struct {
	Tags   []string       `json:"tags"`
	Config map[string]int `json:"config"`
}

type EmbeddedStruct struct {
	BasicStruct `mapstructure:",squash"` // mapstructure 사용 시 squash 태그 필요 (Decode 함수 내부 config 확인 필요)
	Extra       string                   `json:"extra"`
}

// Unexported 필드는 mapstructure에서 무시되어야 함
type PrivateFieldStruct struct {
	Public  string `json:"public"`
	private string
}

type TimeStruct struct {
	Duration time.Duration `json:"duration"`
}

// -------------------------------------------------------------------------
// Test Functions
// -------------------------------------------------------------------------

func TestDecode(t *testing.T) {
	t.Parallel()

	t.Run("BasicStruct_Mapping", testBasicStructMapping)
	t.Run("NestedStruct_Mapping", testNestedStructMapping)
	t.Run("SliceAndMap_Mapping", testSliceAndMapMapping)
	t.Run("PointerFields_Mapping", testPointerFieldsMapping)
	t.Run("WeakTypeConversion", testWeakTypeConversion)
	t.Run("UnexportedFields_Ignored", testUnexportedFieldsIgnored)
	t.Run("ZeroValues_And_PartialInput", testZeroValuesAndPartialInput)
	t.Run("StructToStruct_Decoding", testStructToStructDecoding) // [New] Struct -> Struct 변환 테스트
	t.Run("ErrorCases", testErrorCases)
	t.Run("TimeDuration_Parsing", testTimeDurationParsing)

	// Expert Level Tests
	t.Run("StrictErrorChecking", testStrictErrorChecking)
	t.Run("SquashBehavior", testSquashBehavior)
	t.Run("TextUnmarshalerHook", testTextUnmarshalerHook)
	t.Run("AnyInputSupport", testAnyInputSupport)
}

func testBasicStructMapping(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"name":       "Alice",
		"age":        30,
		"is_enabled": true,
	}

	got, err := Decode[BasicStruct](input)
	require.NoError(t, err)
	assert.Equal(t, "Alice", got.Name)
	assert.Equal(t, 30, got.Age)
	assert.True(t, got.IsEnabled)
}

func testNestedStructMapping(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"title": "Nested Test",
		"detail": map[string]any{
			"name": "Bob",
			"age":  25,
		},
	}

	got, err := Decode[NestedStruct](input)
	require.NoError(t, err)
	assert.Equal(t, "Nested Test", got.Title)
	assert.Equal(t, "Bob", got.Detail.Name)
	assert.Equal(t, 25, got.Detail.Age)
	assert.False(t, got.Detail.IsEnabled) // Zero value
}

func testSliceAndMapMapping(t *testing.T) {
	t.Parallel()

	// 1. Basic Slice & Map
	input := map[string]any{
		"tags": []string{"go", "json", "map"},
		"config": map[string]any{
			"timeout": 100,
			"retry":   3,
		},
	}
	got, err := Decode[SliceMapStruct](input)
	require.NoError(t, err)
	assert.Equal(t, []string{"go", "json", "map"}, got.Tags)
	assert.Equal(t, map[string]int{"timeout": 100, "retry": 3}, got.Config)

	// 2. Slice Hook Test (Comma Separated String -> Slice)
	t.Run("StringToSliceHook", func(t *testing.T) {
		hookInput := map[string]any{
			"tags": "dev,qa,prod",
		}
		gotHook, err := Decode[SliceMapStruct](hookInput)
		require.NoError(t, err)
		assert.Equal(t, []string{"dev", "qa", "prod"}, gotHook.Tags)
	})

	// 3. Empty String Case
	t.Run("EmptyStringSlice", func(t *testing.T) {
		emptyInput := map[string]any{
			"tags": "",
		}
		gotEmpty, err := Decode[SliceMapStruct](emptyInput)
		require.NoError(t, err)
		// mapstructure의 StringToSliceHookFunc는 빈 문자열을 분리할 때 빈 슬라이스를 반환합니다.
		// strings.Split("", ",") -> [""] (길이 1)과는 다른 동작입니다.
		assert.Empty(t, gotEmpty.Tags)
	})
}

func testPointerFieldsMapping(t *testing.T) {
	t.Parallel()

	t.Run("값이_있는_경우", func(t *testing.T) {
		input := map[string]any{
			"value": 123,
			"data":  "ptr",
		}
		got, err := Decode[PointerStruct](input)
		require.NoError(t, err)
		assert.NotNil(t, got.Value)
		assert.Equal(t, 123, *got.Value)
		assert.NotNil(t, got.Data)
		assert.Equal(t, "ptr", *got.Data)
	})

	t.Run("값이_없는_경우", func(t *testing.T) {
		input := map[string]any{}
		got, err := Decode[PointerStruct](input)
		require.NoError(t, err)
		assert.Nil(t, got.Value)
		assert.Nil(t, got.Data)
	})
}

func testWeakTypeConversion(t *testing.T) {
	t.Parallel()

	// mapstructure.DecoderConfig.WeaklyTypedInput = true 효과 검증
	input := map[string]any{
		"name":       12345,  // int -> string (주의: mapstructure 기본 동작에서 int->string은 지원되지 않을 수 있음. 확인 필요)
		"age":        "42",   // string -> int
		"is_enabled": "true", // string -> bool
	}

	got, err := Decode[BasicStruct](input)
	require.NoError(t, err)
	assert.Equal(t, "12345", got.Name)
	assert.Equal(t, 42, got.Age)
	assert.True(t, got.IsEnabled)

	// Single Value to Slice Check
	sliceInput := map[string]any{
		"tags": "single-tag", // string -> []string
	}
	gotSlice, err := Decode[SliceMapStruct](sliceInput)
	require.NoError(t, err)
	assert.Equal(t, []string{"single-tag"}, gotSlice.Tags)
}

func testUnexportedFieldsIgnored(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"public":  "visible",
		"private": "hidden", // 소문자 필드는 매핑되지 않아야 하며, ErrorUnused가 true이면 에러가 반환되어야 함
	}

	_, err := Decode[PrivateFieldStruct](input)
	// ErrorUnused: true 설정으로 인해 매핑되지 않는 키("private")가 있으면 에러가 발생해야 합니다.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private") // 에러 메시지에 필드명이 포함되는지 확인
}

func testZeroValuesAndPartialInput(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"name": "Partial",
	}

	got, err := Decode[BasicStruct](input)
	require.NoError(t, err)
	assert.Equal(t, "Partial", got.Name)
	assert.Equal(t, 0, got.Age)
	assert.False(t, got.IsEnabled)
}

func testStructToStructDecoding(t *testing.T) {
	t.Parallel()

	// Source struct (different type but same fields/tags)
	type SourceStruct struct {
		Name      string `json:"name"`
		Age       int    `json:"age"`
		IsEnabled bool   `json:"is_enabled"`
	}

	source := SourceStruct{
		Name:      "StructSource",
		Age:       99,
		IsEnabled: true,
	}

	got, err := Decode[BasicStruct](source)
	require.NoError(t, err)
	assert.Equal(t, "StructSource", got.Name)
	assert.Equal(t, 99, got.Age)
	assert.True(t, got.IsEnabled)
}

func testErrorCases(t *testing.T) {
	t.Parallel()

	t.Run("Unsupported_Target_Type", func(t *testing.T) {
		input := map[string]any{"key": "value"}
		_, err := Decode[int](input) // map -> int
		assert.Error(t, err)
	})

	t.Run("Nil_Input", func(t *testing.T) {
		var input any = nil // any 타입으로 변경
		got, err := Decode[BasicStruct](input)
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, "", got.Name) // Zero Value
	})
}

func testTimeDurationParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		want      time.Duration
		expectErr bool
	}{
		{name: "Seconds", input: "10s", want: 10 * time.Second, expectErr: false},
		{name: "Minutes", input: "5m", want: 5 * time.Minute, expectErr: false},
		{name: "Combined", input: "1h30m", want: 90 * time.Minute, expectErr: false},
		{name: "Microseconds", input: "500us", want: 500 * time.Microsecond, expectErr: false},
		{name: "Zero", input: "0s", want: 0, expectErr: false},
		{name: "Invalid", input: "invalid", want: 0, expectErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := map[string]any{"duration": tt.input}
			got, err := Decode[TimeStruct](input)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got.Duration)
			}
		})
	}
}

// -------------------------------------------------------------------------
// Expert Level Tests
// -------------------------------------------------------------------------

func testStrictErrorChecking(t *testing.T) {
	t.Parallel()

	// 1. 단일 미사용 필드
	t.Run("Single_Unused_Field", func(t *testing.T) {
		input := map[string]any{
			"name":   "Valid",
			"unused": "This should cause error",
		}
		_, err := Decode[BasicStruct](input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unused")
	})

	// 2. 중첩 구조체 내 미사용 필드
	t.Run("Nested_Unused_Field", func(t *testing.T) {
		input := map[string]any{
			"title": "Nested Root",
			"detail": map[string]any{
				"name":        "Bob",
				"age":         30,
				"unknown_key": "fail",
			},
		}
		_, err := Decode[NestedStruct](input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown_key")
	})
}

func testSquashBehavior(t *testing.T) {
	t.Parallel()

	// Squash: true 동작 검증
	// 상위 레벨의 키가 임베디드 구조체(BasicStruct)의 필드(Name)로 직접 매핑되어야 함
	input := map[string]any{
		"name":  "Embedded Name",
		"age":   100,
		"extra": "Extra Data",
	}

	got, err := Decode[EmbeddedStruct](input)
	require.NoError(t, err)
	assert.Equal(t, "Embedded Name", got.Name) // BasicStruct.Name
	assert.Equal(t, 100, got.Age)              // BasicStruct.Age
	assert.Equal(t, "Extra Data", got.Extra)
}

func testTextUnmarshalerHook(t *testing.T) {
	t.Parallel()

	type CustomTextType struct {
		Value string
	}

	// encoding.TextUnmarshaler 인터페이스 구현
	// (참고: 포인터 리시버여야 함)

	type HookStruct struct {
		TimePtr  *time.Time `json:"time"`
		MyCustom *Custom    `json:"custom"`
	}

	input := map[string]any{
		"time":   "2023-12-25T00:00:00Z",
		"custom": "custom-value",
	}

	got, err := Decode[HookStruct](input)
	require.NoError(t, err)

	// 1. *time.Time 검증
	assert.NotNil(t, got.TimePtr)
	expectedTime, _ := time.Parse(time.RFC3339, "2023-12-25T00:00:00Z")
	assert.Equal(t, expectedTime.UTC(), got.TimePtr.UTC())

	// 2. Custom Type 검증
	assert.NotNil(t, got.MyCustom)
	assert.Equal(t, "parsed:custom-value", got.MyCustom.Data)
}

// Custom 타입 정의 (encoding.TextUnmarshaler 구현)
type Custom struct {
	Data string
}

func (c *Custom) UnmarshalText(text []byte) error {
	c.Data = "parsed:" + string(text)
	return nil
}

func testAnyInputSupport(t *testing.T) {
	t.Parallel()

	// 1. map[interface{}]interface{} (YAML 파싱 시 흔한 형태)
	t.Run("MapInterfaceInterface", func(t *testing.T) {
		input := map[interface{}]interface{}{
			"name": "Generic Map",
			"age":  88,
		}
		got, err := Decode[BasicStruct](input)
		require.NoError(t, err)
		assert.Equal(t, "Generic Map", got.Name)
		assert.Equal(t, 88, got.Age)
	})

	// 2. Scalar Value to Struct (Error Expected)
	// 구조체로 디코딩을 요청했으나 입력이 스칼라 값인 경우 에러가 발생해야 정상
	t.Run("ScalarInput_Error", func(t *testing.T) {
		input := "just string"
		_, err := Decode[BasicStruct](input)
		require.Error(t, err)
	})
}
