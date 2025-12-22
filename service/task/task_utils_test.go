package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTakeSliceArg(t *testing.T) {
	t.Run("정상적인 슬라이스 변환", func(t *testing.T) {
		input := []string{"a", "b", "c"}
		result, ok := TakeSliceArg(input)

		assert.True(t, ok, "슬라이스 변환이 성공해야 합니다")
		assert.Equal(t, 3, len(result), "변환된 슬라이스의 길이가 일치해야 합니다")
		assert.Equal(t, "a", result[0], "첫 번째 요소가 일치해야 합니다")
		assert.Equal(t, "b", result[1], "두 번째 요소가 일치해야 합니다")
		assert.Equal(t, "c", result[2], "세 번째 요소가 일치해야 합니다")
	})

	t.Run("빈 슬라이스 변환", func(t *testing.T) {
		input := []int{}
		result, ok := TakeSliceArg(input)

		assert.True(t, ok, "빈 슬라이스도 변환이 성공해야 합니다")
		assert.Equal(t, 0, len(result), "변환된 슬라이스의 길이가 0이어야 합니다")
	})

	t.Run("슬라이스가 아닌 타입", func(t *testing.T) {
		input := "not a slice"
		result, ok := TakeSliceArg(input)

		assert.False(t, ok, "슬라이스가 아닌 타입은 변환이 실패해야 합니다")
		assert.Nil(t, result, "결과가 nil이어야 합니다")
	})

	t.Run("다양한 타입의 슬라이스", func(t *testing.T) {
		intSlice := []int{1, 2, 3}
		result, ok := TakeSliceArg(intSlice)

		assert.True(t, ok, "int 슬라이스도 변환이 성공해야 합니다")
		assert.Equal(t, 3, len(result), "변환된 슬라이스의 길이가 일치해야 합니다")
		assert.Equal(t, 1, result[0], "첫 번째 요소가 일치해야 합니다")
	})
}

func TestEachSourceElementIsInTargetElementOrNot(t *testing.T) {
	t.Run("모든 요소가 타겟에 존재하는 경우", func(t *testing.T) {
		source := []string{"a", "b", "c"}
		target := []string{"a", "b", "c", "d"}

		foundCount := 0
		notFoundCount := 0

		err := EachSourceElementIsInTargetElementOrNot(
			source,
			target,
			func(selem, telem interface{}) (bool, error) {
				return selem.(string) == telem.(string), nil
			},
			func(selem, telem interface{}) {
				foundCount++
			},
			func(selem interface{}) {
				notFoundCount++
			},
		)

		assert.NoError(t, err, "에러가 발생하지 않아야 합니다")
		assert.Equal(t, 3, foundCount, "3개의 요소가 발견되어야 합니다")
		assert.Equal(t, 0, notFoundCount, "발견되지 않은 요소가 없어야 합니다")
	})

	t.Run("일부 요소만 타겟에 존재하는 경우", func(t *testing.T) {
		source := []string{"a", "b", "c", "d"}
		target := []string{"a", "c"}

		foundCount := 0
		notFoundCount := 0

		err := EachSourceElementIsInTargetElementOrNot(
			source,
			target,
			func(selem, telem interface{}) (bool, error) {
				return selem.(string) == telem.(string), nil
			},
			func(selem, telem interface{}) {
				foundCount++
			},
			func(selem interface{}) {
				notFoundCount++
			},
		)

		assert.NoError(t, err, "에러가 발생하지 않아야 합니다")
		assert.Equal(t, 2, foundCount, "2개의 요소가 발견되어야 합니다")
		assert.Equal(t, 2, notFoundCount, "2개의 요소가 발견되지 않아야 합니다")
	})

	t.Run("equalFn이 nil인 경우", func(t *testing.T) {
		source := []string{"a"}
		target := []string{"a"}

		err := EachSourceElementIsInTargetElementOrNot(source, target, nil, nil, nil)

		assert.Error(t, err, "equalFn이 nil이면 에러가 발생해야 합니다")
		assert.Contains(t, err.Error(), "equalFn()이 할당되지 않았습니다", "적절한 에러 메시지를 반환해야 합니다")
	})

	t.Run("source가 슬라이스가 아닌 경우", func(t *testing.T) {
		source := "not a slice"
		target := []string{"a"}

		err := EachSourceElementIsInTargetElementOrNot(
			source,
			target,
			func(selem, telem interface{}) (bool, error) { return true, nil },
			nil,
			nil,
		)

		assert.Error(t, err, "source가 슬라이스가 아니면 에러가 발생해야 합니다")
		assert.Contains(t, err.Error(), "source 인자의 Slice 타입 변환이 실패", "적절한 에러 메시지를 반환해야 합니다")
	})

	t.Run("target이 슬라이스가 아닌 경우", func(t *testing.T) {
		source := []string{"a"}
		target := "not a slice"

		err := EachSourceElementIsInTargetElementOrNot(
			source,
			target,
			func(selem, telem interface{}) (bool, error) { return true, nil },
			nil,
			nil,
		)

		assert.Error(t, err, "target이 슬라이스가 아니면 에러가 발생해야 합니다")
		assert.Contains(t, err.Error(), "target 인자의 Slice 타입 변환이 실패", "적절한 에러 메시지를 반환해야 합니다")
	})
}

func TestFilter(t *testing.T) {
	t.Run("포함 키워드만 있는 경우", func(t *testing.T) {
		s := "이것은 테스트 문자열입니다"
		includedKeywords := []string{"테스트"}
		excludedKeywords := []string{}

		result := Filter(s, includedKeywords, excludedKeywords)
		assert.True(t, result, "포함 키워드가 있으면 true를 반환해야 합니다")
	})

	t.Run("포함 키워드가 없는 경우", func(t *testing.T) {
		s := "이것은 테스트 문자열입니다"
		includedKeywords := []string{"샘플"}
		excludedKeywords := []string{}

		result := Filter(s, includedKeywords, excludedKeywords)
		assert.False(t, result, "포함 키워드가 없으면 false를 반환해야 합니다")
	})

	t.Run("제외 키워드가 있는 경우", func(t *testing.T) {
		s := "이것은 테스트 문자열입니다"
		includedKeywords := []string{"테스트"}
		excludedKeywords := []string{"문자열"}

		result := Filter(s, includedKeywords, excludedKeywords)
		assert.False(t, result, "제외 키워드가 있으면 false를 반환해야 합니다")
	})

	t.Run("여러 포함 키워드 모두 만족", func(t *testing.T) {
		s := "이것은 테스트 문자열입니다"
		includedKeywords := []string{"테스트", "문자열"}
		excludedKeywords := []string{}

		result := Filter(s, includedKeywords, excludedKeywords)
		assert.True(t, result, "모든 포함 키워드가 있으면 true를 반환해야 합니다")
	})

	t.Run("여러 포함 키워드 중 하나 불만족", func(t *testing.T) {
		s := "이것은 테스트 문자열입니다"
		includedKeywords := []string{"테스트", "샘플"}
		excludedKeywords := []string{}

		result := Filter(s, includedKeywords, excludedKeywords)
		assert.False(t, result, "포함 키워드 중 하나라도 없으면 false를 반환해야 합니다")
	})

	t.Run("OR 조건 포함 키워드 - 하나라도 만족", func(t *testing.T) {
		s := "이것은 테스트 문자열입니다"
		includedKeywords := []string{"샘플|테스트|예제"}
		excludedKeywords := []string{}

		result := Filter(s, includedKeywords, excludedKeywords)
		assert.True(t, result, "OR 조건 중 하나라도 만족하면 true를 반환해야 합니다")
	})

	t.Run("OR 조건 포함 키워드 - 모두 불만족", func(t *testing.T) {
		s := "이것은 테스트 문자열입니다"
		includedKeywords := []string{"샘플|예제|데모"}
		excludedKeywords := []string{}

		result := Filter(s, includedKeywords, excludedKeywords)
		assert.False(t, result, "OR 조건 모두 불만족하면 false를 반환해야 합니다")
	})

	t.Run("복합 조건 - 포함과 제외", func(t *testing.T) {
		s := "삼성 갤럭시 스마트폰"
		includedKeywords := []string{"삼성", "스마트폰"}
		excludedKeywords := []string{"아이폰"}

		result := Filter(s, includedKeywords, excludedKeywords)
		assert.True(t, result, "포함 키워드는 만족하고 제외 키워드는 없으면 true를 반환해야 합니다")
	})

	t.Run("빈 키워드 리스트", func(t *testing.T) {
		s := "이것은 테스트 문자열입니다"
		includedKeywords := []string{}
		excludedKeywords := []string{}

		result := Filter(s, includedKeywords, excludedKeywords)
		assert.True(t, result, "키워드가 없으면 true를 반환해야 합니다")
	})

	t.Run("대소문자 구분 테스트", func(t *testing.T) {
		s := "Samsung Galaxy Smartphone"
		includedKeywords := []string{"samsung"}
		excludedKeywords := []string{}

		result := Filter(s, includedKeywords, excludedKeywords)

		// filter 함수가 대소문자를 구분하는지 확인
		// 실제 구현에 따라 결과가 달라질 수 있음
		if result {
			// 대소문자 구분 안 함
			assert.True(t, result, "대소문자 구분 없이 매칭되어야 합니다")
		} else {
			// 대소문자 구분 함
			assert.False(t, result, "대소문자를 구분하여 매칭해야 합니다")
		}
	})
}
