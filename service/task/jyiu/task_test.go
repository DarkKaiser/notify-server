package jyiu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJyiuNotice_String(t *testing.T) {
	t.Run("HTML 메시지 포맷", func(t *testing.T) {
		notice := &notice{
			Title: "테스트 공지",
			Date:  "2025-01-01",
			URL:   "https://example.com/notice/1",
		}

		result := notice.String(true, "")

		assert.Contains(t, result, "테스트 공지", "공지 제목이 포함되어야 합니다")
		assert.Contains(t, result, "<a href", "HTML 링크 태그가 포함되어야 합니다")
	})

	t.Run("텍스트 메시지 포맷", func(t *testing.T) {
		notice := &notice{
			Title: "테스트 공지",
			Date:  "2025-01-01",
			URL:   "https://example.com/notice/1",
		}

		result := notice.String(false, "")

		assert.Contains(t, result, "테스트 공지", "공지 제목이 포함되어야 합니다")
		assert.NotContains(t, result, "<a href", "HTML 태그가 포함되지 않아야 합니다")
	})
}

func TestJyiuEducation_String(t *testing.T) {
	t.Run("HTML 메시지 포맷", func(t *testing.T) {
		education := &education{
			Title:            "테스트 교육",
			TrainingPeriod:   "2025-01-01 ~ 2025-01-31",
			AcceptancePeriod: "2024-12-01 ~ 2024-12-31",
			URL:              "https://example.com/education/1",
		}

		result := education.String(true, "")

		assert.Contains(t, result, "테스트 교육", "교육 제목이 포함되어야 합니다")
		assert.Contains(t, result, "2025-01-01", "교육 기간이 포함되어야 합니다")
		assert.Contains(t, result, "2024-12-01", "접수 기간이 포함되어야 합니다")
		assert.Contains(t, result, "<a href", "HTML 링크 태그가 포함되어야 합니다")
	})

	t.Run("텍스트 메시지 포맷", func(t *testing.T) {
		education := &education{
			Title:            "테스트 교육",
			TrainingPeriod:   "2025-01-01 ~ 2025-01-31",
			AcceptancePeriod: "2024-12-01 ~ 2024-12-31",
			URL:              "https://example.com/education/1",
		}

		result := education.String(false, "")

		assert.Contains(t, result, "테스트 교육", "교육 제목이 포함되어야 합니다")
		assert.NotContains(t, result, "<a href", "HTML 태그가 포함되지 않아야 합니다")
	})
}
