package validator_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/darkkaiser/notify-server/internal/pkg/validator"
	go_validator "github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

// TestGet_Concurrency verifies compliance with the Singleton pattern in a concurrent environment.
func TestGet_Concurrency(t *testing.T) {
	var wg sync.WaitGroup
	const routines = 100
	validators := make([]*go_validator.Validate, routines)

	wg.Add(routines)
	for i := 0; i < routines; i++ {
		go func(index int) {
			defer wg.Done()
			validators[index] = validator.Get()
		}(i)
	}
	wg.Wait()

	// All instances must be identical
	first := validators[0]
	for i := 1; i < routines; i++ {
		assert.Same(t, first, validators[i], "All validator instances should be the same")
	}
}

// Structs for testing
type TestUser struct {
	Name string `validate:"required" korean:"이름"`
	Age  int    `validate:"min=18" korean:"나이"`
}

type ComplexTestStruct struct {
	// Basic Types & Required
	RequiredField string `validate:"required" korean:"필수항목"`

	// Min/Max/Len (String)
	MinString string `validate:"min=5" korean:"최소문자"`
	MaxString string `validate:"max=5" korean:"최대문자"`
	LenString string `validate:"len=3" korean:"길이문자"`

	// Min/Max/Len (Number)
	MinInt int `validate:"min=10" korean:"최소숫자"`
	MaxInt int `validate:"max=100" korean:"최대숫자"`
	// LenInt int `validate:"len=3"` // Unsupported in validator v10 for int value length

	// Min/Max/Len (Slice - Array)
	MinSlice []int `validate:"min=2" korean:"최소배열"`
	MaxSlice []int `validate:"max=2" korean:"최대배열"`
	LenSlice []int `validate:"len=2" korean:"길이배열"`

	// LTE/GTE (String)
	LteString string `validate:"lte=3" korean:"이하문자"`
	GteString string `validate:"gte=3" korean:"이상문자"`

	// LTE/GTE (Number)
	LteInt int `validate:"lte=100" korean:"이하숫자"`
	GteInt int `validate:"gte=10" korean:"이상숫자"`

	// Formats
	Email    string `validate:"email" korean:"이메일"`
	URL      string `validate:"url" korean:"웹사이트"`
	UUID     string `validate:"uuid" korean:"식별자"`
	Alphanum string `validate:"alphanum" korean:"영문숫자"`

	// Enum & Boolean
	OneOf   string `validate:"oneof=red blue" korean:"색상"`
	Boolean string `validate:"boolean" korean:"동의여부"`

	// Edge Cases
	NoTagField string `validate:"required"`           // Should fallback to field name
	UnknownTag string `validate:"alpha" korean:"알파벳"` // Tag not explicitly handled in switch
}

func TestStruct_General(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     interface{}
		expectErr bool
		errMsg    string
	}{
		{
			name:      "Valid Input",
			input:     TestUser{Name: "Tester", Age: 20},
			expectErr: false,
		},
		{
			name:      "Invalid Input",
			input:     TestUser{Name: "", Age: 10},
			expectErr: true,
			errMsg:    "이름는 필수입니다", // Returns the first error
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validator.Struct(tt.input)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Equal(t, tt.errMsg, validator.FormatValidationError(err))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFormatValidationError_Comprehensive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  ComplexTestStruct
		expect string
	}{
		// Required
		{
			name:   "Required",
			input:  ComplexTestStruct{RequiredField: ""},
			expect: "필수항목는 필수입니다",
		},
		// String Constraints
		{
			name:   "Min String",
			input:  ComplexTestStruct{RequiredField: "v", MinString: "abc"},
			expect: "최소문자는 최소 5자 이상이어야 합니다",
		},
		{
			name:   "Max String",
			input:  ComplexTestStruct{RequiredField: "v", MinString: "12345", MaxString: "123456"},
			expect: "최대문자는 최대 5자까지 입력 가능합니다",
		},
		{
			name:   "Len String",
			input:  ComplexTestStruct{RequiredField: "v", MinString: "12345", LenString: "ab"},
			expect: "길이문자는 3자여야 합니다",
		},
		// Number Constraints
		{
			name:   "Min Int",
			input:  ComplexTestStruct{RequiredField: "v", MinString: "12345", LenString: "abc", MinInt: 5},
			expect: "최소숫자는 최소 10 이상이어야 합니다",
		},
		{
			name:   "Max Int",
			input:  ComplexTestStruct{RequiredField: "v", MinString: "12345", LenString: "abc", MinInt: 10, MaxInt: 101},
			expect: "최대숫자는 최대 100까지 입력 가능합니다",
		},
		// Slice Constraints
		{
			name:   "Min Slice",
			input:  ComplexTestStruct{RequiredField: "v", MinString: "12345", LenString: "abc", MinInt: 10, MinSlice: []int{1}},
			expect: "최소배열는 최소 2 이상이어야 합니다", // Current implementation behavior
		},
		{
			name:   "Max Slice",
			input:  ComplexTestStruct{RequiredField: "v", MinString: "12345", LenString: "abc", MinInt: 10, MinSlice: []int{1, 2}, MaxSlice: []int{1, 2, 3}},
			expect: "최대배열는 최대 2까지 입력 가능합니다", // Current implementation behavior
		},
		{
			name:   "Len Slice",
			input:  ComplexTestStruct{RequiredField: "v", MinString: "12345", LenString: "abc", MinInt: 10, MinSlice: []int{1, 2}, LenSlice: []int{1}},
			expect: "길이배열는 갯수가 2개여야 합니다",
		},
		// Example of LTE/GTE String
		{
			name:   "Lte String",
			input:  ComplexTestStruct{RequiredField: "v", MinString: "12345", LenString: "abc", MinInt: 10, MinSlice: []int{1, 2}, LenSlice: []int{1, 2}, LteString: "1234"},
			expect: "이하문자는 최대 3자까지 입력 가능합니다",
		},
		{
			name:   "Gte String",
			input:  ComplexTestStruct{RequiredField: "v", MinString: "12345", LenString: "abc", MinInt: 10, MinSlice: []int{1, 2}, LenSlice: []int{1, 2}, LteString: "123", GteString: "12"},
			expect: "이상문자는 최소 3자 이상이어야 합니다",
		},
		// Example of LTE/GTE Int
		{
			name:   "Lte Int",
			input:  ComplexTestStruct{RequiredField: "v", MinString: "12345", LenString: "abc", MinInt: 10, MinSlice: []int{1, 2}, LenSlice: []int{1, 2}, LteString: "123", GteString: "123", LteInt: 101},
			expect: "이하숫자는 100 이하이어야 합니다",
		},
		{
			name:   "Gte Int",
			input:  ComplexTestStruct{RequiredField: "v", MinString: "12345", LenString: "abc", MinInt: 10, MinSlice: []int{1, 2}, LenSlice: []int{1, 2}, LteString: "123", GteString: "123", GteInt: 5},
			expect: "이상숫자는 10 이상이어야 합니다",
		},
		// Formats
		{
			name:   "Email",
			input:  ComplexTestStruct{RequiredField: "v", MinString: "12345", LenString: "abc", MinInt: 10, MinSlice: []int{1, 2}, LenSlice: []int{1, 2}, LteString: "123", GteString: "123", GteInt: 10, Email: "invalid-email"},
			expect: "이메일는 올바른 이메일 형식이어야 합니다",
		},
		{
			name:   "URL",
			input:  ComplexTestStruct{RequiredField: "v", MinString: "12345", LenString: "abc", MinInt: 10, MinSlice: []int{1, 2}, LenSlice: []int{1, 2}, LteString: "123", GteString: "123", GteInt: 10, Email: "a@b.com", URL: "invalid-url"},
			expect: "웹사이트는 올바른 URL 형식이어야 합니다",
		},
		{
			name:   "UUID",
			input:  ComplexTestStruct{RequiredField: "v", MinString: "12345", LenString: "abc", MinInt: 10, MinSlice: []int{1, 2}, LenSlice: []int{1, 2}, LteString: "123", GteString: "123", GteInt: 10, Email: "a@b.com", URL: "http://test.com", UUID: "invalid"},
			expect: "식별자는 올바른 UUID 형식이어야 합니다",
		},
		{
			name:   "Alphanum",
			input:  ComplexTestStruct{RequiredField: "v", MinString: "12345", LenString: "abc", MinInt: 10, MinSlice: []int{1, 2}, LenSlice: []int{1, 2}, LteString: "123", GteString: "123", GteInt: 10, Email: "a@b.com", URL: "http://test.com", UUID: "550e8400-e29b-41d4-a716-446655440000", Alphanum: "abc-def"},
			expect: "영문숫자는 영문자와 숫자만 입력 가능합니다",
		},
		// Enums & Boolean
		{
			name:   "OneOf",
			input:  ComplexTestStruct{RequiredField: "v", MinString: "12345", LenString: "abc", MinInt: 10, MinSlice: []int{1, 2}, LenSlice: []int{1, 2}, LteString: "123", GteString: "123", GteInt: 10, Email: "a@b.com", URL: "http://test.com", UUID: "550e8400-e29b-41d4-a716-446655440000", Alphanum: "abc1", OneOf: "green"},
			expect: "색상는 허용된 값 중 하나여야 합니다 [red blue]",
		},
		{
			name:   "Boolean",
			input:  ComplexTestStruct{RequiredField: "v", MinString: "12345", LenString: "abc", MinInt: 10, MinSlice: []int{1, 2}, LenSlice: []int{1, 2}, LteString: "123", GteString: "123", GteInt: 10, Email: "a@b.com", URL: "http://test.com", UUID: "550e8400-e29b-41d4-a716-446655440000", Alphanum: "abc1", OneOf: "red", Boolean: "not-bool"},
			expect: "동의여부는 true 또는 false 값이어야 합니다",
		},
		// Edge Cases
		{
			name:   "No Korean Tag (Fallback)",
			input:  ComplexTestStruct{RequiredField: "v", MinString: "12345", LenString: "abc", MinInt: 10, MinSlice: []int{1, 2}, LenSlice: []int{1, 2}, LteString: "123", GteString: "123", GteInt: 10, Email: "a@b.com", URL: "http://test.com", UUID: "550e8400-e29b-41d4-a716-446655440000", Alphanum: "abc1", OneOf: "red", Boolean: "true", NoTagField: ""},
			expect: "NoTagField는 필수입니다",
		},
		{
			name:   "Unknown Tag (Alpha)",
			input:  ComplexTestStruct{RequiredField: "v", MinString: "12345", LenString: "abc", MinInt: 10, MinSlice: []int{1, 2}, LenSlice: []int{1, 2}, LteString: "123", GteString: "123", GteInt: 10, Email: "a@b.com", URL: "http://test.com", UUID: "550e8400-e29b-41d4-a716-446655440000", Alphanum: "abc1", OneOf: "red", Boolean: "true", NoTagField: "ok", UnknownTag: "123"},
			expect: "알파벳 값 검증 실패 (alpha)",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validator.Struct(tt.input)
			assert.Error(t, err)
			assert.Equal(t, tt.expect, validator.FormatValidationError(err))
		})
	}
}

func TestFormatValidationError_EdgeCases(t *testing.T) {
	t.Parallel()

	// 1. nil error
	assert.Equal(t, "", validator.FormatValidationError(nil))

	// 2. non-validator error
	err := errors.New("just a normal error")
	assert.Equal(t, "just a normal error", validator.FormatValidationError(err))

	// 3. empty ValidationErrors (unlikely in real usage but possible types)
	emptyValErrs := go_validator.ValidationErrors{}
	// Since ValidationErrors is a slice, FormatValidationError casts it but handles len=0
	// The implementation returns err.Error() if len==0
	assert.Equal(t, emptyValErrs.Error(), validator.FormatValidationError(emptyValErrs))
}
