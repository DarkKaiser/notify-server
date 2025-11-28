package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAlganicmallEvent_String(t *testing.T) {
	t.Run("HTML 메시지 포맷", func(t *testing.T) {
		event := &alganicmallEvent{
			Name: "테스트 이벤트",
			URL:  "https://www.alganicmall.com/event/1",
		}

		result := event.String(true, "")

		assert.Contains(t, result, "테스트 이벤트", "이벤트 이름이 포함되어야 합니다")
		assert.Contains(t, result, "<a href", "HTML 링크 태그가 포함되어야 합니다")
	})

	t.Run("텍스트 메시지 포맷", func(t *testing.T) {
		event := &alganicmallEvent{
			Name: "테스트 이벤트",
			URL:  "https://www.alganicmall.com/event/1",
		}

		result := event.String(false, "")

		assert.Contains(t, result, "테스트 이벤트", "이벤트 이름이 포함되어야 합니다")
		assert.NotContains(t, result, "<a href", "HTML 태그가 포함되지 않아야 합니다")
	})
}

func TestAlganicmallProduct_String(t *testing.T) {
	t.Run("HTML 메시지 포맷", func(t *testing.T) {
		product := &alganicmallProduct{
			Name:  "테스트 상품",
			Price: 10000,
			URL:   "https://www.alganicmall.com/product/1",
		}

		result := product.String(true, "")

		assert.Contains(t, result, "테스트 상품", "상품 이름이 포함되어야 합니다")
		assert.Contains(t, result, "10,000원", "가격이 포함되어야 합니다")
		assert.Contains(t, result, "<a href", "HTML 링크 태그가 포함되어야 합니다")
	})

	t.Run("텍스트 메시지 포맷", func(t *testing.T) {
		product := &alganicmallProduct{
			Name:  "테스트 상품",
			Price: 10000,
			URL:   "https://www.alganicmall.com/product/1",
		}

		result := product.String(false, "")

		assert.Contains(t, result, "테스트 상품", "상품 이름이 포함되어야 합니다")
		assert.Contains(t, result, "10,000원", "가격이 포함되어야 합니다")
		assert.NotContains(t, result, "<a href", "HTML 태그가 포함되지 않아야 합니다")
	})
}
