package provider

import (
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/stretchr/testify/assert"
)

type TestSettings struct {
	Foo string `mapstructure:"foo"`
	Bar int    `mapstructure:"bar"`
}

func (s *TestSettings) Validate() error {
	return nil
}

func TestFindTaskSettings(t *testing.T) {
	appConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: "NAVER",
				Data: map[string]interface{}{
					"foo": "baz",
					"bar": 123,
				},
			},
		},
	}

	t.Run("성공: 존재하는 Task 설정 조회", func(t *testing.T) {
		settings, err := FindTaskSettings[TestSettings](appConfig, "NAVER")
		assert.NoError(t, err)
		assert.Equal(t, "baz", settings.Foo)
		assert.Equal(t, 123, settings.Bar)
	})

	t.Run("실패: 존재하지 않는 Task 설정 조회", func(t *testing.T) {
		settings, err := FindTaskSettings[TestSettings](appConfig, "KURLY")
		assert.Error(t, err)
		assert.Nil(t, settings)
		assert.ErrorIs(t, err, ErrTaskSettingsNotFound)
	})
}

func TestFindCommandSettings(t *testing.T) {
	appConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: "NAVER",
				Commands: []config.CommandConfig{
					{
						ID: "CheckPrice",
						Data: map[string]interface{}{
							"foo": "hello",
							"bar": 456,
						},
					},
				},
			},
		},
	}

	t.Run("성공: 존재하는 Command 설정 조회", func(t *testing.T) {
		settings, err := FindCommandSettings[TestSettings](appConfig, "NAVER", "CheckPrice")
		assert.NoError(t, err)
		assert.Equal(t, "hello", settings.Foo)
		assert.Equal(t, 456, settings.Bar)
	})

	t.Run("실패: Task는 존재하지만 Command가 없는 경우 (개선된 로직 검증)", func(t *testing.T) {
		settings, err := FindCommandSettings[TestSettings](appConfig, "NAVER", "UnknownCommand")
		assert.Error(t, err)
		assert.Nil(t, settings)
		assert.ErrorIs(t, err, ErrCommandSettingsNotFound)
	})

	t.Run("실패: Task 자체가 존재하지 않는 경우", func(t *testing.T) {
		settings, err := FindCommandSettings[TestSettings](appConfig, "KURLY", "CheckPrice")
		assert.Error(t, err)
		assert.Nil(t, settings)
		assert.ErrorIs(t, err, ErrCommandSettingsNotFound)
	})
}

type ValidationTestSettings struct {
	Value int `mapstructure:"value"`
}

func (s *ValidationTestSettings) Validate() error {
	if s.Value < 0 {
		return apperrors.New(apperrors.InvalidInput, "value must be non-negative")
	}
	return nil
}

func TestDecodeAndValidate_Validation(t *testing.T) {
	t.Run("성공: 유효한 값 (포인터 리시버 Validate 호출 확인)", func(t *testing.T) {
		data := map[string]any{"value": 10}
		settings, err := decodeAndValidate[ValidationTestSettings](data)
		assert.NoError(t, err)
		assert.NotNil(t, settings)
		assert.Equal(t, 10, settings.Value)
	})

	t.Run("실패: 유효하지 않은 값 (포인터 리시버 Validate 필터링 확인)", func(t *testing.T) {
		data := map[string]any{"value": -1}
		settings, err := decodeAndValidate[ValidationTestSettings](data)
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "value must be non-negative")
		}
		assert.Nil(t, settings)
	})
}
