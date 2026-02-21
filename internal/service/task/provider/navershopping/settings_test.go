package navershopping

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// taskSettings.Validate()
// =============================================================================

// TestTaskSettings_Validate 필수 인증 설정값 유효성 검증 로직을 검증합니다.
//
// 검증 항목:
//   - 성공 경로: ClientID와 ClientSecret이 모두 설정된 경우
//   - 실패 경로: 각 필드가 빈 문자열이거나 공백만 있는 경우
//   - Whitespace Trim: 공백이 제거된 결과가 필드에 반영되는지
//   - Sentinel 에러 동일성: errors.Is 로 정확한 에러 상수를 반환하는지
func TestTaskSettings_Validate(t *testing.T) {
	t.Parallel()

	t.Run("성공: ClientID와 ClientSecret이 모두 유효", func(t *testing.T) {
		t.Parallel()
		s := taskSettings{ClientID: "valid_id", ClientSecret: "valid_secret"}
		require.NoError(t, s.Validate())
	})

	t.Run("성공: 공백 포함 값이 Trim 후 유효", func(t *testing.T) {
		t.Parallel()
		s := taskSettings{ClientID: "  my_id  ", ClientSecret: " my_secret "}
		require.NoError(t, s.Validate())
		// Trim이 필드에 반영되었는지 확인
		assert.Equal(t, "my_id", s.ClientID)
		assert.Equal(t, "my_secret", s.ClientSecret)
	})

	t.Run("실패: ClientID가 빈 문자열 → ErrClientIDMissing", func(t *testing.T) {
		t.Parallel()
		s := taskSettings{ClientID: "", ClientSecret: "valid_secret"}
		err := s.Validate()
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrClientIDMissing))
	})

	t.Run("실패: ClientID가 공백만 있음 → ErrClientIDMissing (Trim 후 빈 문자열)", func(t *testing.T) {
		t.Parallel()
		s := taskSettings{ClientID: "   ", ClientSecret: "valid_secret"}
		err := s.Validate()
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrClientIDMissing))
		// Trim이 필드에 반영되었는지 확인
		assert.Equal(t, "", s.ClientID)
	})

	t.Run("실패: ClientSecret이 빈 문자열 → ErrClientSecretMissing", func(t *testing.T) {
		t.Parallel()
		s := taskSettings{ClientID: "valid_id", ClientSecret: ""}
		err := s.Validate()
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrClientSecretMissing))
	})

	t.Run("실패: ClientSecret이 공백만 있음 → ErrClientSecretMissing (Trim 후 빈 문자열)", func(t *testing.T) {
		t.Parallel()
		s := taskSettings{ClientID: "valid_id", ClientSecret: "   "}
		err := s.Validate()
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrClientSecretMissing))
		// Trim이 필드에 반영되었는지 확인
		assert.Equal(t, "", s.ClientSecret)
	})

	t.Run("실패: ClientID 검증을 ClientSecret보다 먼저 수행 (순서 보장)", func(t *testing.T) {
		t.Parallel()
		s := taskSettings{ClientID: "", ClientSecret: ""}
		err := s.Validate()
		require.Error(t, err)
		// ClientID가 먼저 검증되어야 하므로 ErrClientSecretMissing이 아닌 ErrClientIDMissing 반환
		assert.True(t, errors.Is(err, ErrClientIDMissing), "ClientID 검증이 ClientSecret보다 먼저 실행되어야 합니다")
		assert.False(t, errors.Is(err, ErrClientSecretMissing))
	})
}

// =============================================================================
// watchPriceSettings.Validate()
// =============================================================================

// TestWatchPriceSettings_Validate 상품 가격 감시 설정값의 유효성 검증 로직을 검증합니다.
//
// 검증 항목:
//   - 성공 경로: Query와 PriceLessThan이 모두 유효한 경우
//   - 실패 경로: Query가 누락되거나 공백인 경우 → ErrEmptyQuery
//   - 실패 경로: PriceLessThan이 0 또는 음수인 경우 → newErrInvalidPrice
//   - 에러 메시지에 실제 입력값이 포함되는지 (newErrInvalidPrice 검증)
//   - Query Trim이 필드에 반영되는지
func TestWatchPriceSettings_Validate(t *testing.T) {
	t.Parallel()

	t.Run("성공: Query와 PriceLessThan이 모두 유효", func(t *testing.T) {
		t.Parallel()
		s := NewSettingsBuilder().WithQuery("Samsung Galaxy S24").WithPriceLessThan(800000).Build()
		require.NoError(t, s.Validate())
	})

	t.Run("성공: 공백 포함 Query가 Trim 후 유효", func(t *testing.T) {
		t.Parallel()
		s := NewSettingsBuilder().WithQuery("  iPhone 15  ").WithPriceLessThan(1000000).Build()
		require.NoError(t, s.Validate())
		assert.Equal(t, "iPhone 15", s.Query, "Query는 Trim 값이 필드에 반영되어야 합니다")
	})

	t.Run("실패: Query 빈 문자열 → ErrEmptyQuery", func(t *testing.T) {
		t.Parallel()
		s := NewSettingsBuilder().WithQuery("").WithPriceLessThan(10000).Build()
		err := s.Validate()
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrEmptyQuery))
	})

	t.Run("실패: Query가 공백만 있음 → ErrEmptyQuery (Trim 후 빈 문자열)", func(t *testing.T) {
		t.Parallel()
		s := NewSettingsBuilder().WithQuery("   ").WithPriceLessThan(10000).Build()
		err := s.Validate()
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrEmptyQuery))
		assert.Equal(t, "", s.Query, "Trim 결과가 Query 필드에 반영되어야 합니다")
	})

	t.Run("실패: PriceLessThan이 0 → price_less_than 관련 에러", func(t *testing.T) {
		t.Parallel()
		const invalidPrice = 0
		s := NewSettingsBuilder().WithQuery("test").WithPriceLessThan(invalidPrice).Build()
		err := s.Validate()
		require.Error(t, err)
		// 입력값이 에러 메시지에 포함되어야 함 (newErrInvalidPrice 포맷 검증)
		assert.Contains(t, err.Error(), fmt.Sprintf("(입력값: %d)", invalidPrice))
	})

	t.Run("실패: PriceLessThan이 음수 → price_less_than 관련 에러 (입력값 포함)", func(t *testing.T) {
		t.Parallel()
		const invalidPrice = -100
		s := NewSettingsBuilder().WithQuery("test").WithPriceLessThan(invalidPrice).Build()
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("(입력값: %d)", invalidPrice))
	})

	t.Run("실패: Query 검증이 PriceLessThan보다 먼저 수행 (순서 보장)", func(t *testing.T) {
		t.Parallel()
		// 두 항목이 모두 유효하지 않을 때 Query 에러가 먼저 반환되어야 함
		s := NewSettingsBuilder().WithQuery("").WithPriceLessThan(0).Build()
		err := s.Validate()
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrEmptyQuery), "Query 검증이 PriceLessThan보다 먼저 실행되어야 합니다")
	})
}

// =============================================================================
// watchPriceSettings.ApplyDefaults()
// =============================================================================

// TestWatchPriceSettings_ApplyDefaults 미설정 필드에 기본값이 올바르게 적용되는지 검증합니다.
func TestWatchPriceSettings_ApplyDefaults(t *testing.T) {
	t.Parallel()

	t.Run("PageFetchDelay가 0이면 기본값 100ms 적용", func(t *testing.T) {
		t.Parallel()
		s := watchPriceSettings{}
		s.ApplyDefaults()
		assert.Equal(t, 100, s.PageFetchDelay)
	})

	t.Run("PageFetchDelay가 음수이면 기본값 100ms 적용", func(t *testing.T) {
		t.Parallel()
		s := watchPriceSettings{PageFetchDelay: -1}
		s.ApplyDefaults()
		assert.Equal(t, 100, s.PageFetchDelay)
	})

	t.Run("PageFetchDelay가 유효한 양수이면 값이 유지됨", func(t *testing.T) {
		t.Parallel()
		s := watchPriceSettings{PageFetchDelay: 500}
		s.ApplyDefaults()
		assert.Equal(t, 500, s.PageFetchDelay)
	})

	t.Run("PageFetchDelay가 정확히 기본값(100)이어도 유지됨", func(t *testing.T) {
		t.Parallel()
		s := watchPriceSettings{PageFetchDelay: 100}
		s.ApplyDefaults()
		assert.Equal(t, 100, s.PageFetchDelay)
	})
}
