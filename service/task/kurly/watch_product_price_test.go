package kurly

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKurlyWatchProductPriceConfig_Validate(t *testing.T) {
	t.Run("정상적인 데이터", func(t *testing.T) {
		commandConfig := &watchProductPriceCommandConfig{
			WatchProductsFile: "test.csv",
		}

		err := commandConfig.validate()
		assert.NoError(t, err, "정상적인 데이터는 검증을 통과해야 합니다")
	})

	t.Run("파일 경로가 비어있는 경우", func(t *testing.T) {
		commandConfig := &watchProductPriceCommandConfig{
			WatchProductsFile: "",
		}

		err := commandConfig.validate()
		assert.Error(t, err, "파일 경로가 비어있으면 에러가 발생해야 합니다")
		assert.Contains(t, err.Error(), "파일이 입력되지 않았습니다", "적절한 에러 메시지를 반환해야 합니다")
	})

	t.Run("CSV 파일이 아닌 경우", func(t *testing.T) {
		commandConfig := &watchProductPriceCommandConfig{
			WatchProductsFile: "test.txt",
		}

		err := commandConfig.validate()
		assert.Error(t, err, "CSV 파일이 아니면 에러가 발생해야 합니다")
		assert.Contains(t, err.Error(), ".CSV 파일만 사용할 수 있습니다", "적절한 에러 메시지를 반환해야 합니다")
	})

	t.Run("대소문자 구분 없이 CSV 확장자 허용", func(t *testing.T) {
		testCases := []string{
			"test.csv",
			"test.CSV",
			"test.Csv",
		}

		for _, filename := range testCases {
			commandConfig := &watchProductPriceCommandConfig{
				WatchProductsFile: filename,
			}

			err := commandConfig.validate()
			assert.NoError(t, err, "CSV 확장자는 대소문자 구분 없이 허용해야 합니다: %s", filename)
		}
	})
}

func TestKurlyProduct_String(t *testing.T) {
	t.Run("일반 가격 - HTML 메시지", func(t *testing.T) {
		product := &product{
			No:              12345,
			Name:            "테스트 상품",
			Price:           10000,
			DiscountedPrice: 0,
			DiscountRate:    0,
		}

		result := product.String(true, "", nil)

		assert.Contains(t, result, "테스트 상품", "상품 이름이 포함되어야 합니다")
		assert.Contains(t, result, "10,000원", "가격이 포함되어야 합니다")
		assert.Contains(t, result, "goods/12345", "상품 링크가 포함되어야 합니다")
	})

	t.Run("할인 가격 - HTML 메시지", func(t *testing.T) {
		product := &product{
			No:              12345,
			Name:            "할인 상품",
			Price:           10000,
			DiscountedPrice: 8000,
			DiscountRate:    20,
		}

		result := product.String(true, "", nil)

		assert.Contains(t, result, "할인 상품", "상품 이름이 포함되어야 합니다")
		assert.Contains(t, result, "10,000원", "원래 가격이 포함되어야 합니다")
		assert.Contains(t, result, "8,000원", "할인 가격이 포함되어야 합니다")
		assert.Contains(t, result, "20%", "할인율이 포함되어야 합니다")
		assert.Contains(t, result, "<s>", "HTML 취소선 태그가 포함되어야 합니다")
	})

	t.Run("일반 가격 - 텍스트 메시지", func(t *testing.T) {
		product := &product{
			No:              12345,
			Name:            "테스트 상품",
			Price:           10000,
			DiscountedPrice: 0,
			DiscountRate:    0,
		}

		result := product.String(false, "", nil)

		assert.Contains(t, result, "테스트 상품", "상품 이름이 포함되어야 합니다")
		assert.Contains(t, result, "10,000원", "가격이 포함되어야 합니다")
		assert.NotContains(t, result, "<a href", "HTML 태그가 포함되지 않아야 합니다")
	})

	t.Run("할인 가격 - 텍스트 메시지", func(t *testing.T) {
		product := &product{
			No:              12345,
			Name:            "할인 상품",
			Price:           10000,
			DiscountedPrice: 8000,
			DiscountRate:    20,
		}

		result := product.String(false, "", nil)

		assert.Contains(t, result, "할인 상품", "상품 이름이 포함되어야 합니다")
		assert.Contains(t, result, "10,000원", "원래 가격이 포함되어야 합니다")
		assert.Contains(t, result, "8,000원", "할인 가격이 포함되어야 합니다")
		assert.Contains(t, result, "⇒", "화살표 기호가 포함되어야 합니다")
		assert.NotContains(t, result, "<s>", "HTML 태그가 포함되지 않아야 합니다")
	})

	t.Run("마크 표시", func(t *testing.T) {
		product := &product{
			No:    12345,
			Name:  "테스트 상품",
			Price: 10000,
		}

		result := product.String(false, " 🆕", nil)

		assert.Contains(t, result, "🆕", "마크가 포함되어야 합니다")
	})

	t.Run("이전 가격 정보 포함", func(t *testing.T) {
		previousProduct := &product{
			Price:           12000,
			DiscountedPrice: 0,
			DiscountRate:    0,
		}

		currentProduct := &product{
			No:    12345,
			Name:  "가격 변경 상품",
			Price: 10000,
		}

		result := currentProduct.String(false, "", previousProduct)

		assert.Contains(t, result, "이전 가격", "이전 가격 레이블이 포함되어야 합니다")
		assert.Contains(t, result, "12,000원", "이전 가격이 포함되어야 합니다")
	})
}

func TestKurlyProduct_UpdateLowestPrice(t *testing.T) {
	t.Run("최저 가격이 없는 경우 - 일반 가격", func(t *testing.T) {
		product := &product{
			Price:           10000,
			DiscountedPrice: 0,
			LowestPrice:     0,
		}

		product.updateLowestPrice()

		assert.Equal(t, 10000, product.LowestPrice, "최저 가격이 설정되어야 합니다")
		assert.False(t, product.LowestPriceTime.IsZero(), "최저 가격 시간이 설정되어야 합니다")
	})

	t.Run("최저 가격이 없는 경우 - 할인 가격", func(t *testing.T) {
		product := &product{
			Price:           10000,
			DiscountedPrice: 8000,
			LowestPrice:     0,
		}

		product.updateLowestPrice()

		assert.Equal(t, 8000, product.LowestPrice, "할인 가격이 최저 가격으로 설정되어야 합니다")
	})

	t.Run("기존 최저 가격보다 낮은 가격", func(t *testing.T) {
		product := &product{
			Price:           7000,
			DiscountedPrice: 0,
			LowestPrice:     9000,
		}

		product.updateLowestPrice()

		assert.Equal(t, 7000, product.LowestPrice, "더 낮은 가격으로 최저 가격이 업데이트되어야 합니다")
	})

	t.Run("기존 최저 가격보다 높은 가격", func(t *testing.T) {
		product := &product{
			Price:           11000,
			DiscountedPrice: 0,
			LowestPrice:     9000,
		}

		product.updateLowestPrice()

		assert.Equal(t, 9000, product.LowestPrice, "최저 가격이 유지되어야 합니다")
	})

	t.Run("할인 가격이 최저 가격보다 낮은 경우", func(t *testing.T) {
		product := &product{
			Price:           10000,
			DiscountedPrice: 7500,
			LowestPrice:     9000,
		}

		product.updateLowestPrice()

		assert.Equal(t, 7500, product.LowestPrice, "할인 가격이 최저 가격으로 업데이트되어야 합니다")
	})
}

func TestKurlyTask_NormalizeDuplicateProducts(t *testing.T) {
	task := &task{}

	t.Run("중복이 없는 경우", func(t *testing.T) {
		products := [][]string{
			{"12345", "상품1", "1"},
			{"67890", "상품2", "1"},
		}

		distinct, duplicate := task.normalizeDuplicateProducts(products)

		assert.Equal(t, 2, len(distinct), "모든 상품이 distinct에 포함되어야 합니다")
		assert.Equal(t, 0, len(duplicate), "중복 상품이 없어야 합니다")
	})

	t.Run("중복이 있는 경우", func(t *testing.T) {
		products := [][]string{
			{"12345", "상품1", "1"},
			{"67890", "상품2", "1"},
			{"12345", "상품1 중복", "1"},
		}

		distinct, duplicate := task.normalizeDuplicateProducts(products)

		assert.Equal(t, 2, len(distinct), "중복이 제거된 상품 목록이어야 합니다")
		assert.Equal(t, 1, len(duplicate), "중복 상품이 1개 있어야 합니다")
		assert.Equal(t, "12345", duplicate[0][0], "중복 상품 코드가 일치해야 합니다")
	})

	t.Run("여러 중복이 있는 경우", func(t *testing.T) {
		products := [][]string{
			{"12345", "상품1", "1"},
			{"67890", "상품2", "1"},
			{"12345", "상품1 중복1", "1"},
			{"12345", "상품1 중복2", "1"},
			{"67890", "상품2 중복", "1"},
		}

		distinct, duplicate := task.normalizeDuplicateProducts(products)

		assert.Equal(t, 2, len(distinct), "중복이 제거된 상품 목록이어야 합니다")
		assert.Equal(t, 3, len(duplicate), "중복 상품이 3개 있어야 합니다")
	})

	t.Run("빈 행이 있는 경우", func(t *testing.T) {
		products := [][]string{
			{"12345", "상품1", "1"},
			{},
			{"67890", "상품2", "1"},
		}

		distinct, duplicate := task.normalizeDuplicateProducts(products)

		assert.Equal(t, 2, len(distinct), "빈 행은 무시되어야 합니다")
		assert.Equal(t, 0, len(duplicate), "중복 상품이 없어야 합니다")
	})
}

func TestKurlyWatchProductPriceConfig_Validate_ErrorCases(t *testing.T) {
	t.Run("빈 파일 경로", func(t *testing.T) {
		commandConfig := &watchProductPriceCommandConfig{
			WatchProductsFile: "",
		}

		err := commandConfig.validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "파일이 입력되지 않았습니다")
	})

	t.Run("잘못된 파일 확장자", func(t *testing.T) {
		testCases := []string{
			"test.txt",
			"test.xlsx",
			"test.json",
			"test",
		}

		for _, filename := range testCases {
			commandConfig := &watchProductPriceCommandConfig{
				WatchProductsFile: filename,
			}

			err := commandConfig.validate()
			assert.Error(t, err, "파일 확장자가 CSV가 아니면 에러가 발생해야 합니다: %s", filename)
		}
	})
}

func TestKurlyProduct_UpdateLowestPrice_EdgeCases(t *testing.T) {
	t.Run("가격이 0인 경우", func(t *testing.T) {
		product := &product{
			Price:           0,
			DiscountedPrice: 0,
			LowestPrice:     0,
		}

		product.updateLowestPrice()

		// 가격이 0이면 최저가가 업데이트되지 않아야 함
		assert.Equal(t, 0, product.LowestPrice)
	})
}

func TestKurlyTask_NormalizeDuplicateProducts_EdgeCases(t *testing.T) {
	task := &task{}

	t.Run("빈 입력", func(t *testing.T) {
		products := [][]string{}

		distinct, duplicate := task.normalizeDuplicateProducts(products)

		assert.Equal(t, 0, len(distinct))
		assert.Equal(t, 0, len(duplicate))
	})

	t.Run("모두 빈 행인 경우", func(t *testing.T) {
		products := [][]string{
			{},
			{},
			{},
		}

		distinct, duplicate := task.normalizeDuplicateProducts(products)

		assert.Equal(t, 0, len(distinct))
		assert.Equal(t, 0, len(duplicate))
	})

	t.Run("모두 중복인 경우", func(t *testing.T) {
		products := [][]string{
			{"12345", "상품1", "1"},
			{"12345", "상품1 중복1", "1"},
			{"12345", "상품1 중복2", "1"},
		}

		distinct, duplicate := task.normalizeDuplicateProducts(products)

		assert.Equal(t, 1, len(distinct), "첫 번째 항목만 distinct에 포함되어야 합니다")
		assert.Equal(t, 2, len(duplicate), "나머지는 모두 중복이어야 합니다")
	})

	t.Run("불완전한 행 처리", func(t *testing.T) {
		products := [][]string{
			{"12345", "상품1", "1"},
			{"67890"}, // 컬럼이 부족한 행
			{"11111", "상품3", "1"},
		}

		distinct, duplicate := task.normalizeDuplicateProducts(products)

		// 불완전한 행도 처리되어야 함
		assert.Equal(t, 3, len(distinct))
		assert.Equal(t, 0, len(duplicate))
	})

}

func TestKurlyProduct_String_EdgeCases(t *testing.T) {
	t.Run("특수 문자가 포함된 상품명 - HTML", func(t *testing.T) {
		product := &product{
			No:    12345,
			Name:  "<script>alert('test')</script>",
			Price: 10000,
		}

		result := product.String(true, "", nil)

		// HTML 이스케이프 처리 확인
		assert.NotContains(t, result, "<script>", "스크립트 태그가 이스케이프되어야 합니다")
		assert.Contains(t, result, "&lt;script&gt;", "이스케이프된 형태로 포함되어야 합니다")
	})

	t.Run("매우 긴 상품명", func(t *testing.T) {
		longName := string(make([]byte, 1000))
		for i := range longName {
			longName = longName[:i] + "가"
		}

		product := &product{
			No:    12345,
			Name:  longName[:500], // 500자 상품명
			Price: 10000,
		}

		result := product.String(false, "", nil)

		assert.Contains(t, result, "10,000원")
		assert.Greater(t, len(result), 500, "긴 상품명도 처리할 수 있어야 합니다")
	})

	t.Run("가격이 매우 큰 경우", func(t *testing.T) {
		product := &product{
			No:              12345,
			Name:            "고가 상품",
			Price:           999999999,
			DiscountedPrice: 888888888,
			DiscountRate:    11,
		}

		result := product.String(false, "", nil)

		assert.Contains(t, result, "999,999,999원", "큰 가격도 올바르게 포맷되어야 합니다")
		assert.Contains(t, result, "888,888,888원", "큰 할인 가격도 올바르게 포맷되어야 합니다")
	})
}
