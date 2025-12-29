package maputil

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Documentation Examples (GoDoc)
// =============================================================================

type Config struct {
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
	}

	// 맵 데이터를 구조체로 디코딩
	cfg, err := Decode[Config](input)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Host: %s, Port: %d, Timeout: %s, Tags: %v\n", cfg.Host, cfg.Port, cfg.Timeout, cfg.Tags)

	// Output:
	// Host: localhost, Port: 8080, Timeout: 5s, Tags: [server http api]
}

func ExampleDecodeTo() {
	// 기본값(Default) 설정
	cfg := &Config{
		Host:    "127.0.0.1",
		Port:    9000,
		Debug:   false,
		Timeout: 30 * time.Second,
	}

	// 파일이나 환경 변수에서 읽어온 오버라이드 값
	// (일부 필드만 존재할 수 있음)
	override := map[string]any{
		"port":  3000,
		"debug": true,
	}

	// 기존 객체(cfg)에 오버라이드 값 병합(Merge)
	if err := DecodeTo(override, cfg); err != nil {
		panic(err)
	}

	fmt.Printf("Host: %s, Port: %d, Debug: %v\n", cfg.Host, cfg.Port, cfg.Debug)

	// Output:
	// Host: 127.0.0.1, Port: 3000, Debug: true
}

// =============================================================================
// Unit Tests
// =============================================================================

// -------------------------------------------------------------------------
// Test Structures
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
	BasicStruct `mapstructure:",squash"`
	Extra       string `json:"extra"`
}

type PrivateFieldStruct struct {
	Public  string `json:"public"`
	private string `json:"private"`
}

type TimeStruct struct {
	Duration time.Duration `json:"duration"`
}

// CustomText는 encoding.TextUnmarshaler를 구현하는 테스트용 구조체입니다.
type CustomText struct {
	Value string
}

func (c *CustomText) UnmarshalText(text []byte) error {
	c.Value = "parsed:" + string(text)
	return nil
}

type HookTestStruct struct {
	IP     net.IP      `json:"ip"`
	Custom *CustomText `json:"custom"`
}

// -------------------------------------------------------------------------
// Tests
// -------------------------------------------------------------------------

func TestDecode(t *testing.T) {
	t.Parallel()

	t.Run("BasicStruct_Mapping", func(t *testing.T) {
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
	})

	t.Run("NestedStruct_Mapping", func(t *testing.T) {
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
	})

	t.Run("SliceAndMap_Mapping", func(t *testing.T) {
		t.Parallel()
		input := map[string]any{
			"tags": "go,json,map", // StringToSliceHook
			"config": map[string]any{
				"timeout": 100,
			},
		}
		got, err := Decode[SliceMapStruct](input)
		require.NoError(t, err)
		assert.Equal(t, []string{"go", "json", "map"}, got.Tags)
		assert.Equal(t, map[string]int{"timeout": 100}, got.Config)
	})

	t.Run("TextUnmarshalerHook", func(t *testing.T) {
		t.Parallel()
		input := map[string]any{
			"ip":     "192.168.1.1",
			"custom": "custom-value",
		}
		got, err := Decode[HookTestStruct](input)
		require.NoError(t, err)

		// 1. net.IP (Case 2: *net.IP implements, field is net.IP)의 동작 검증
		assert.Equal(t, net.ParseIP("192.168.1.1"), got.IP)

		// 2. Custom struct (Case 1: *CustomText implements, field is *CustomText) 검증
		// 훅이 정상 작동했다면 "parsed:" 접두사가 붙어야 함
		assert.NotNil(t, got.Custom)
		assert.Equal(t, "parsed:custom-value", got.Custom.Value)
	})

	t.Run("StrictErrorChecking_UnusedField", func(t *testing.T) {
		t.Parallel()
		input := map[string]any{
			"name":        "Valid",
			"unknown_key": "Should Fail",
		}
		_, err := Decode[BasicStruct](input)
		require.Error(t, err) // ErrorUnused: true
	})

	t.Run("WeakTypeConversion", func(t *testing.T) {
		t.Parallel()
		input := map[string]any{
			"age": "42", // string -> int
		}
		got, err := Decode[BasicStruct](input)
		require.NoError(t, err)
		assert.Equal(t, 42, got.Age)
	})
}

func TestDecodeTo(t *testing.T) {
	t.Parallel()

	t.Run("Merge_Config", func(t *testing.T) {
		t.Parallel()
		// 기본값
		cfg := &BasicStruct{
			Name:      "Default",
			Age:       10,
			IsEnabled: false,
		}

		// 오버라이드 (일부 필드)
		input := map[string]any{
			"age":        20,
			"is_enabled": true,
		}

		err := DecodeTo(input, cfg)
		require.NoError(t, err)

		// Name 보존, 나머지는 변경
		assert.Equal(t, "Default", cfg.Name)
		assert.Equal(t, 20, cfg.Age)
		assert.True(t, cfg.IsEnabled)
	})

	t.Run("Error_If_Not_Pointer", func(t *testing.T) {
		t.Parallel()
		cfg := BasicStruct{}
		err := DecodeTo(map[string]any{"age": 20}, cfg)
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
		_, _ = Decode[BasicStruct](input)
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
		_, _ = Decode[NestedStruct](input)
	}
}

func BenchmarkDecodeTo_Reuse(b *testing.B) {
	input := map[string]any{
		"name": "Update",
		"age":  50,
	}
	// 구조체 재사용 (Alloc 줄이기)
	target := &BasicStruct{
		Name: "Original",
		Age:  0,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DecodeTo(input, target)
	}
}
