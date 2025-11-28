package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJdcOnlineEducationCourse_String(t *testing.T) {
	t.Run("HTML 메시지 포맷", func(t *testing.T) {
		course := &jdcOnlineEducationCourse{
			Title1:         "테스트 교육",
			Title2:         "상세 제목",
			TrainingPeriod: "2025-01-01 ~ 2025-01-31",
			URL:            "https://example.com/course/1",
		}

		result := course.String(true, "")

		assert.Contains(t, result, "테스트 교육", "교육 제목이 포함되어야 합니다")
		assert.Contains(t, result, "2025-01-01", "교육 기간이 포함되어야 합니다")
		assert.Contains(t, result, "<a href", "HTML 링크 태그가 포함되어야 합니다")
	})

	t.Run("텍스트 메시지 포맷", func(t *testing.T) {
		course := &jdcOnlineEducationCourse{
			Title1:         "테스트 교육",
			Title2:         "상세 제목",
			TrainingPeriod: "2025-01-01 ~ 2025-01-31",
			URL:            "https://example.com/course/1",
		}

		result := course.String(false, "")

		assert.Contains(t, result, "테스트 교육", "교육 제목이 포함되어야 합니다")
		assert.NotContains(t, result, "<a href", "HTML 태그가 포함되지 않아야 합니다")
	})
}
