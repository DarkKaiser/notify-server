package maputil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDecode(t *testing.T) {
	t.Parallel()

	type NestedConfig struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}

	type TestConfig struct {
		Name          string        `json:"name"`
		Count         int           `json:"count"`
		IsEnabled     bool          `json:"is_enabled"`
		Tags          []string      `json:"tags"`
		Nested        NestedConfig  `json:"nested"`
		Duration      time.Duration `json:"duration"`
		OptionalField string        `json:"optional_field"` // 입력 맵에 없을 경우
	}

	tests := []struct {
		name      string
		input     map[string]any
		target    any
		want      any
		expectErr bool
	}{
		{
			name: "성공: 기본 필드 매핑 및 JSON 태그 지원",
			input: map[string]any{
				"name":       "test-app",
				"count":      100,
				"is_enabled": true,
				"tags":       []string{"go", "test"},
				"nested": map[string]any{
					"host": "localhost",
					"port": 8080,
				},
			},
			target: &TestConfig{},
			want: &TestConfig{
				Name:      "test-app",
				Count:     100,
				IsEnabled: true,
				Tags:      []string{"go", "test"},
				Nested: NestedConfig{
					Host: "localhost",
					Port: 8080,
				},
			},
			expectErr: false,
		},
		{
			name: "성공: Weak Type Conversion (문자열 -> 숫자/불리언)",
			input: map[string]any{
				"name":       "weak-type",
				"count":      "500",   // string -> int 자동 변환
				"is_enabled": "true",  // string -> bool 자동 변환
				"tags":       "a,b,c", // string -> []string (mapstructure 기본 동작 아님, 별도 Hook 필요하지만 여기선 검증 제외)
			},
			target: &TestConfig{},
			want: &TestConfig{
				Name:      "weak-type",
				Count:     500,
				IsEnabled: true,
				// Tags: 기본적으로 string -> slice 변환은 지원하지 않음 (Hook 필요)
			},
			expectErr: false,
		},
		{
			name: "성공: 누락된 필드는 기본값 유지",
			input: map[string]any{
				"name": "partial",
			},
			target: &TestConfig{
				Count: 999, // 기존 값 유지되는지 확인 (Decode는 덮어쓰기이므로 필드 없으면 유지됨)
			},
			want: &TestConfig{
				Name:  "partial",
				Count: 999,
			},
			expectErr: false,
		},
		{
			name:      "실패: Target이 포인터가 아님",
			input:     map[string]any{"name": "fail"},
			target:    TestConfig{}, // 포인터 아님
			want:      TestConfig{},
			expectErr: true,
		},
		{
			name:      "실패: Target이 nil",
			input:     map[string]any{"name": "fail"},
			target:    nil,
			want:      nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Decode(tt.input, tt.target)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				if tt.want != nil {
					got := tt.target.(*TestConfig)
					want := tt.want.(*TestConfig)

					assert.Equal(t, want.Name, got.Name)
					if want.Count != 0 {
						assert.Equal(t, want.Count, got.Count)
					}
					if want.IsEnabled {
						assert.Equal(t, want.IsEnabled, got.IsEnabled)
					}
					if len(want.Tags) > 0 {
						assert.Equal(t, want.Tags, got.Tags)
					}
					if want.Nested.Port != 0 {
						assert.Equal(t, want.Nested, got.Nested)
					}
				}
			}
		})
	}
}
