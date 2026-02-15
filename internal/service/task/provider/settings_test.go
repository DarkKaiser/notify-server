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
				ID: "VALID_TASK",
				Data: map[string]interface{}{
					"foo": "baz",
					"bar": 123,
				},
			},
			{
				ID: "INVALID_DECODE_TASK",
				Data: map[string]interface{}{
					"bar": "not_an_int", // Type mismatch for decoding
				},
			},
			{
				ID: "INVALID_VALIDATE_TASK",
				Data: map[string]interface{}{
					"value": -1, // Invalid business value (for ValidationTestSettings)
				},
			},
		},
	}

	t.Run("성공: 존재하는 Task 설정 조회", func(t *testing.T) {
		settings, err := FindTaskSettings[TestSettings](appConfig, "VALID_TASK")
		assert.NoError(t, err)
		assert.Equal(t, "baz", settings.Foo)
		assert.Equal(t, 123, settings.Bar)
	})

	t.Run("실패: 존재하지 않는 Task 설정 조회", func(t *testing.T) {
		settings, err := FindTaskSettings[TestSettings](appConfig, "UNKNOWN_TASK")
		assert.Error(t, err)
		assert.Nil(t, settings)
		assert.ErrorIs(t, err, ErrTaskNotFound)
	})

	t.Run("실패: 디코딩 실패 (Type Mismatch)", func(t *testing.T) {
		// maputil.Decode에서 에러 발생 유도 (string -> int)
		settings, err := FindTaskSettings[TestSettings](appConfig, "INVALID_DECODE_TASK")
		assert.Error(t, err)
		assert.Nil(t, settings)
		// Check error message since ErrTaskSettingsProcessingFailed is not exported sentinel
		assert.Contains(t, err.Error(), "추가 설정 정보 처리에 실패했습니다")
	})

	t.Run("실패: 유효성 검증 실패 (Validator Error)", func(t *testing.T) {
		settings, err := FindTaskSettings[ValidationTestSettings](appConfig, "INVALID_VALIDATE_TASK")
		assert.Error(t, err)
		assert.Nil(t, settings)
		assert.Contains(t, err.Error(), "추가 설정 정보 처리에 실패했습니다")
		// 내부 Cause가 Validation 에러인지 확인 (Error string check)
		assert.Contains(t, err.Error(), "value must be non-negative")
	})
}

func TestFindCommandSettings(t *testing.T) {
	appConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: "TASK_1",
				Commands: []config.CommandConfig{
					{
						ID: "VALID_CMD",
						Data: map[string]interface{}{
							"foo": "hello",
							"bar": 456,
						},
					},
					{
						ID: "INVALID_DECODE_CMD",
						Data: map[string]interface{}{
							"bar": "not_an_int",
						},
					},
					{
						ID: "INVALID_VALIDATE_CMD",
						Data: map[string]interface{}{
							"value": -5,
						},
					},
				},
			},
		},
	}

	t.Run("성공: 존재하는 Command 설정 조회", func(t *testing.T) {
		settings, err := FindCommandSettings[TestSettings](appConfig, "TASK_1", "VALID_CMD")
		assert.NoError(t, err)
		assert.Equal(t, "hello", settings.Foo)
		assert.Equal(t, 456, settings.Bar)
	})

	t.Run("실패: Task는 존재하지만 Command가 없는 경우", func(t *testing.T) {
		settings, err := FindCommandSettings[TestSettings](appConfig, "TASK_1", "UNKNOWN_CMD")
		assert.Error(t, err)
		assert.Nil(t, settings)
		assert.ErrorIs(t, err, ErrCommandNotFound)
	})

	t.Run("실패: Task 자체가 존재하지 않는 경우", func(t *testing.T) {
		settings, err := FindCommandSettings[TestSettings](appConfig, "UNKNOWN_TASK", "VALID_CMD")
		assert.Error(t, err)
		assert.Nil(t, settings)
		assert.ErrorIs(t, err, ErrTaskNotFound)
	})

	t.Run("실패: 디코딩 실패 (Type Mismatch)", func(t *testing.T) {
		settings, err := FindCommandSettings[TestSettings](appConfig, "TASK_1", "INVALID_DECODE_CMD")
		assert.Error(t, err)
		assert.Nil(t, settings)
		assert.Contains(t, err.Error(), "추가 설정 정보 처리에 실패했습니다")
	})

	t.Run("실패: 유효성 검증 실패 (Validator Error)", func(t *testing.T) {
		settings, err := FindCommandSettings[ValidationTestSettings](appConfig, "TASK_1", "INVALID_VALIDATE_CMD")
		assert.Error(t, err)
		assert.Nil(t, settings)
		assert.Contains(t, err.Error(), "추가 설정 정보 처리에 실패했습니다")
		assert.Contains(t, err.Error(), "value must be non-negative")
	})
}

// ValidationTestSettings Validator 인터페이스 구현체 (테스트용)
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

	t.Run("실패: 디코딩 실패 (Type Mismatch)", func(t *testing.T) {
		data := map[string]any{"value": "not_an_int"}
		settings, err := decodeAndValidate[ValidationTestSettings](data)
		assert.Error(t, err)
		assert.Nil(t, settings)
		// 구체적인 디코딩 에러 메시지는 mapstructure 의존적이므로 Error 발생 여부만 확인
	})
}
