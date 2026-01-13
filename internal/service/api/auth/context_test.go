package auth_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/api/auth"
	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/model/domain"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupContextHelper 테스트용 Echo Context와 Recorder를 생성하는 헬퍼 함수
func setupContextHelper() (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

// TestSetApplication SetApplication 함수가 Context에 올바르게 값을 설정하는지 검증합니다.
func TestSetApplication(t *testing.T) {
	// Given
	c, _ := setupContextHelper()
	expectedApp := &domain.Application{
		ID:    "test-app",
		Title: "Test Application",
	}

	// When
	auth.SetApplication(c, expectedApp)

	// Then
	// 내부 상수에 직접 접근할 수 없으므로(다른 패키지), Get으로 조회하여 간접 검증하거나
	// auth 패키지 내부 테스트(package auth)로 작성해야 하지만,
	// 여기서는 블랙박스 테스트(package auth_test)를 지향하므로 GetApplication으로 검증합니다.
	val := c.Get(constants.ContextKeyApplication)
	assert.Equal(t, expectedApp, val, "Context에 애플리케이션 정보가 저장되어야 합니다")
}

// TestGetApplication GetApplication 함수의 다양한 성공/실패 케이스를 검증합니다.
func TestGetApplication(t *testing.T) {
	t.Run("성공: 올바른 애플리케이션 정보가 있는 경우", func(t *testing.T) {
		// Given
		c, _ := setupContextHelper()
		expectedApp := &domain.Application{ID: "valid-app"}
		auth.SetApplication(c, expectedApp)

		// When
		actualApp, err := auth.GetApplication(c)

		// Then
		require.NoError(t, err)
		assert.Equal(t, expectedApp, actualApp)
	})

	t.Run("실패: Context에 키가 없는 경우", func(t *testing.T) {
		// Given
		c, _ := setupContextHelper()

		// When
		actualApp, err := auth.GetApplication(c)

		// Then
		require.Error(t, err)
		assert.Nil(t, actualApp)
		assert.Equal(t, constants.ErrMsgAuthApplicationMissingInContext, err.Error())
	})

	t.Run("실패: 잘못된 타입이 저장된 경우", func(t *testing.T) {
		// Given
		c, _ := setupContextHelper()
		c.Set(constants.ContextKeyApplication, "invalid-string-type") // *domain.Application이 아닌 값

		// When
		actualApp, err := auth.GetApplication(c)

		// Then
		require.Error(t, err)
		assert.Nil(t, actualApp)
		assert.Equal(t, constants.ErrMsgAuthApplicationTypeMismatch, err.Error())
	})

	t.Run("실패: nil이 저장된 경우", func(t *testing.T) {
		// Given
		c, _ := setupContextHelper()
		var nilApp *domain.Application = nil
		c.Set(constants.ContextKeyApplication, nilApp) // *domain.Application 타입이지만 값은 nil

		// When
		// GetApplication의 타입 단언(assertion)은 nil 포인터도 해당 타입으로 통과시키지만,
		// 비즈니스 로직상 nil이 반환되면 안 되는 경우 추가 검증이 필요할 수 있습니다.
		// 현재 구현상으로는 타입이 맞으면 리턴합니다.
		actualApp, err := auth.GetApplication(c)

		// Then
		require.NoError(t, err)
		assert.Nil(t, actualApp)
	})
}

// TestMustGetApplication MustGetApplication 함수의 동작과 패닉 발생을 검증합니다.
func TestMustGetApplication(t *testing.T) {
	t.Run("성공: 올바른 애플리케이션 정보가 있는 경우", func(t *testing.T) {
		// Given
		c, _ := setupContextHelper()
		expectedApp := &domain.Application{ID: "must-app"}
		auth.SetApplication(c, expectedApp)

		// When
		actualApp := auth.MustGetApplication(c)

		// Then
		assert.Equal(t, expectedApp, actualApp)
	})

	t.Run("패닉: Context에 정보가 없는 경우", func(t *testing.T) {
		// Given
		c, _ := setupContextHelper()

		// When & Then
		assert.PanicsWithValue(t, fmt.Sprintf(constants.PanicMsgAuthContextApplicationNotFound, constants.ErrMsgAuthApplicationMissingInContext), func() {
			auth.MustGetApplication(c)
		}, "정보가 없을 때 적절한 메시지와 함께 패닉이 발생해야 합니다")
	})

	t.Run("패닉: 잘못된 타입인 경우", func(t *testing.T) {
		// Given
		c, _ := setupContextHelper()
		c.Set(constants.ContextKeyApplication, 12345)

		// When & Then
		assert.PanicsWithValue(t, fmt.Sprintf(constants.PanicMsgAuthContextApplicationNotFound, constants.ErrMsgAuthApplicationTypeMismatch), func() {
			auth.MustGetApplication(c)
		}, "타입이 다를 때 적절한 메시지와 함께 패닉이 발생해야 합니다")
	})
}
