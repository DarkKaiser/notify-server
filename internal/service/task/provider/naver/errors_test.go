package naver

import (
	"testing"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestErrors(t *testing.T) {
	t.Run("ErrEmptyQuery validation", func(t *testing.T) {
		// Given
		expectedMessage := "query가 입력되지 않았거나 공백입니다"
		expectedType := apperrors.InvalidInput

		// When
		err := ErrEmptyQuery

		// Then
		// 1. 에러가 nil이 아님을 검증
		assert.Error(t, err)

		// 2. 에러 타입 검증 (apperrors.AppError로 타입 캐스팅하여 확인)
		var appErr *apperrors.AppError
		if assert.ErrorAs(t, err, &appErr) {
			assert.Equal(t, expectedType, appErr.Type(), "에러 타입이 InvalidInput이어야 합니다")
		}

		// 3. 에러 메시지 검증
		assert.Contains(t, err.Error(), expectedMessage, "에러 메시지에 예상 문구가 포함되어야 합니다")

		// 4. apperrors.Is를 사용한 타입 매칭 검증 (권장되는 방식)
		assert.True(t, apperrors.Is(err, apperrors.InvalidInput), "apperrors.Is()로 InvalidInput 타입임이 확인되어야 합니다")
	})
}
