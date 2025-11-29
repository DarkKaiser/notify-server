package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNaverWatchNewPerformancesTaskCommandData_Validate(t *testing.T) {
	t.Run("정상적인 데이터", func(t *testing.T) {
		data := &naverWatchNewPerformancesTaskCommandData{
			Query: "뮤지컬",
		}

		err := data.validate()
		assert.NoError(t, err, "정상적인 데이터는 검증을 통과해야 합니다")
	})

	t.Run("Query가 비어있는 경우", func(t *testing.T) {
		data := &naverWatchNewPerformancesTaskCommandData{
			Query: "",
		}

		err := data.validate()
		assert.Error(t, err, "Query가 비어있으면 에러가 발생해야 합니다")
		assert.Contains(t, err.Error(), "query", "적절한 에러 메시지를 반환해야 합니다")
	})
}

func TestNaverPerformance_String(t *testing.T) {
	t.Run("HTML 메시지 포맷", func(t *testing.T) {
		performance := &naverPerformance{
			Title:     "테스트 공연",
			Place:     "테스트 극장",
			Thumbnail: "https://example.com/thumb.jpg",
		}

		result := performance.String(true, "")

		assert.Contains(t, result, "테스트 공연", "공연 제목이 포함되어야 합니다")
		assert.Contains(t, result, "테스트 극장", "공연 장소가 포함되어야 합니다")
		assert.Contains(t, result, "<b>", "HTML 태그가 포함되어야 합니다")
	})

	t.Run("텍스트 메시지 포맷", func(t *testing.T) {
		performance := &naverPerformance{
			Title:     "테스트 공연",
			Place:     "테스트 극장",
			Thumbnail: "https://example.com/thumb.jpg",
		}

		result := performance.String(false, "")

		assert.Contains(t, result, "테스트 공연", "공연 제목이 포함되어야 합니다")
		assert.Contains(t, result, "테스트 극장", "공연 장소가 포함되어야 합니다")
		assert.NotContains(t, result, "<b>", "HTML 태그가 포함되지 않아야 합니다")
	})

	t.Run("마크 표시", func(t *testing.T) {
		performance := &naverPerformance{
			Title: "테스트 공연",
			Place: "테스트 극장",
		}

		result := performance.String(false, " 🆕")

		assert.Contains(t, result, "🆕", "마크가 포함되어야 합니다")
	})
}

func TestNaverTask_FilterPerformances(t *testing.T) {
	t.Run("제목 필터링 - 포함 키워드", func(t *testing.T) {
		// filter 함수는 task_utils.go에 정의되어 있으므로 별도 테스트
		includedKeywords := []string{"뮤지컬"}
		excludedKeywords := []string{}

		result := filter("뮤지컬 오페라의 유령", includedKeywords, excludedKeywords)
		assert.True(t, result, "포함 키워드가 있으면 true를 반환해야 합니다")
	})

	t.Run("제목 필터링 - 제외 키워드", func(t *testing.T) {
		includedKeywords := []string{"뮤지컬"}
		excludedKeywords := []string{"아동"}

		result := filter("뮤지컬 아동극", includedKeywords, excludedKeywords)
		assert.False(t, result, "제외 키워드가 있으면 false를 반환해야 합니다")
	})

	t.Run("장소 필터링", func(t *testing.T) {
		includedKeywords := []string{"서울"}
		excludedKeywords := []string{}

		result := filter("서울 예술의전당", includedKeywords, excludedKeywords)
		assert.True(t, result, "포함 키워드가 있으면 true를 반환해야 합니다")
	})
}
