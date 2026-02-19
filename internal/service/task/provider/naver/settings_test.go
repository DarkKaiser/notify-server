package naver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSettings_Validate(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		expect  error
		message string
	}{
		{
			name:    "성공: 유효한 Query",
			query:   "뮤지컬",
			expect:  nil,
			message: "",
		},
		{
			name:    "실패: 빈 Query",
			query:   "",
			expect:  ErrEmptyQuery,
			message: "",
		},
		{
			name:    "실패: 공백만 있는 Query",
			query:   "   ",
			expect:  ErrEmptyQuery,
			message: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &watchNewPerformancesSettings{Query: tt.query}
			err := s.Validate()

			if tt.expect != nil {
				assert.ErrorIs(t, err, tt.expect)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSettings_ApplyDefaults(t *testing.T) {
	t.Run("기본값 적용 확인: 값이 없거나 0 이하일 때", func(t *testing.T) {
		// Given: 빈 설정 (Zero Value)
		s := &watchNewPerformancesSettings{}

		// When
		s.ApplyDefaults()

		// Then
		assert.Equal(t, 50, s.MaxPages, "MaxPages 기본값은 50이어야 합니다")
		assert.Equal(t, 100, s.PageFetchDelay, "PageFetchDelay 기본값은 100(ms)이어야 합니다")
	})

	t.Run("기본값 유지 확인: 사용자가 유효한 값을 설정했을 때", func(t *testing.T) {
		// Given: 사용자가 설정한 값
		s := &watchNewPerformancesSettings{
			MaxPages:       10,
			PageFetchDelay: 500,
		}

		// When
		s.ApplyDefaults()

		// Then
		assert.Equal(t, 10, s.MaxPages, "설정된 MaxPages 값은 유지되어야 합니다")
		assert.Equal(t, 500, s.PageFetchDelay, "설정된 PageFetchDelay 값은 유지되어야 합니다")
	})

	t.Run("음수 값 처리 확인: 음수 입력 시 기본값으로 보정", func(t *testing.T) {
		// Given: 잘못된 음수 값
		s := &watchNewPerformancesSettings{
			MaxPages:       -5,
			PageFetchDelay: -10,
		}

		// When
		s.ApplyDefaults()

		// Then
		assert.Equal(t, 50, s.MaxPages, "음수 MaxPages는 기본값(50)으로 보정되어야 합니다")
		assert.Equal(t, 100, s.PageFetchDelay, "음수 PageFetchDelay는 기본값(100)으로 보정되어야 합니다")
	})
}
