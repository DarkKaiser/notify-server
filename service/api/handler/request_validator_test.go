package handler

import (
	"testing"

	"github.com/darkkaiser/notify-server/service/api/v1/model/request"
	"github.com/stretchr/testify/assert"
)

func TestValidateRequest(t *testing.T) {
	t.Run("정상적인 요청", func(t *testing.T) {
		req := &request.NotificationRequest{
			ApplicationID: "test-app",
			Message:       "테스트 메시지",
			ErrorOccurred: false,
		}

		err := ValidateRequest(req)
		assert.NoError(t, err)
	})

	t.Run("ApplicationID 누락", func(t *testing.T) {
		req := &request.NotificationRequest{
			ApplicationID: "",
			Message:       "테스트 메시지",
		}

		err := ValidateRequest(req)
		assert.Error(t, err)
	})

	t.Run("Message 누락", func(t *testing.T) {
		req := &request.NotificationRequest{
			ApplicationID: "test-app",
			Message:       "",
		}

		err := ValidateRequest(req)
		assert.Error(t, err)
	})

	t.Run("Message 길이 초과", func(t *testing.T) {
		// 4096자를 초과하는 메시지 생성
		longMessage := make([]byte, 4097)
		for i := range longMessage {
			longMessage[i] = 'a'
		}

		req := &request.NotificationRequest{
			ApplicationID: "test-app",
			Message:       string(longMessage),
		}

		err := ValidateRequest(req)
		assert.Error(t, err)
	})

	t.Run("Message 최대 길이 (4096자)", func(t *testing.T) {
		// 정확히 4096자인 메시지
		maxMessage := make([]byte, 4096)
		for i := range maxMessage {
			maxMessage[i] = 'a'
		}

		req := &request.NotificationRequest{
			ApplicationID: "test-app",
			Message:       string(maxMessage),
		}

		err := ValidateRequest(req)
		assert.NoError(t, err)
	})

	t.Run("Message 최소 길이 (1자)", func(t *testing.T) {
		req := &request.NotificationRequest{
			ApplicationID: "test-app",
			Message:       "a",
		}

		err := ValidateRequest(req)
		assert.NoError(t, err)
	})
}

func TestFormatValidationError(t *testing.T) {
	t.Run("nil 에러", func(t *testing.T) {
		result := FormatValidationError(nil)
		assert.Equal(t, "", result)
	})

	t.Run("ApplicationID required 에러", func(t *testing.T) {
		req := &request.NotificationRequest{
			ApplicationID: "",
			Message:       "테스트",
		}

		err := ValidateRequest(req)
		assert.Error(t, err)

		message := FormatValidationError(err)
		assert.Contains(t, message, "애플리케이션 ID")
		assert.Contains(t, message, "필수")
	})

	t.Run("Message required 에러", func(t *testing.T) {
		req := &request.NotificationRequest{
			ApplicationID: "test-app",
			Message:       "",
		}

		err := ValidateRequest(req)
		assert.Error(t, err)

		message := FormatValidationError(err)
		assert.Contains(t, message, "메시지")
		assert.Contains(t, message, "필수")
	})

	t.Run("Message max 에러", func(t *testing.T) {
		longMessage := make([]byte, 4097)
		for i := range longMessage {
			longMessage[i] = 'a'
		}

		req := &request.NotificationRequest{
			ApplicationID: "test-app",
			Message:       string(longMessage),
		}

		err := ValidateRequest(req)
		assert.Error(t, err)

		message := FormatValidationError(err)
		assert.Contains(t, message, "메시지")
		assert.Contains(t, message, "최대")
		assert.Contains(t, message, "4096")
	})
}

// TestFormatValidationError_ComplexStruct 모든 검증 태그 테스트를 위한 구조체
type TestStruct struct {
	Name     string `validate:"required,min=2" korean:"이름"`
	Age      int    `validate:"min=18" korean:"나이"`
	Email    string `validate:"email" korean:"이메일"`
	Homepage string `validate:"url" korean:"홈페이지"`
	Bio      string `validate:"max=10" korean:"자기소개"`
}

func TestFormatValidationError_AllTags(t *testing.T) {
	t.Run("min 검증 (문자열)", func(t *testing.T) {
		req := &TestStruct{Name: "a"} // 2글자 미만
		err := ValidateRequest(req)
		assert.Error(t, err)
		msg := FormatValidationError(err)
		assert.Contains(t, msg, "이름")
		assert.Contains(t, msg, "최소 2자")
	})

	t.Run("min 검증 (숫자)", func(t *testing.T) {
		req := &TestStruct{Name: "Lee", Age: 10} // 18세 미만
		err := ValidateRequest(req)
		assert.Error(t, err)
		msg := FormatValidationError(err)
		assert.Contains(t, msg, "나이")
		assert.Contains(t, msg, "최소 18 이상")
	})

	t.Run("email 검증", func(t *testing.T) {
		req := &TestStruct{Name: "Lee", Age: 20, Email: "invalid-email"}
		err := ValidateRequest(req)
		assert.Error(t, err)
		msg := FormatValidationError(err)
		assert.Contains(t, msg, "이메일")
		assert.Contains(t, msg, "올바른 이메일 형식")
	})

	t.Run("url 검증", func(t *testing.T) {
		req := &TestStruct{Name: "Lee", Age: 20, Email: "test@example.com", Homepage: "invalid-url"}
		err := ValidateRequest(req)
		assert.Error(t, err)
		msg := FormatValidationError(err)
		assert.Contains(t, msg, "홈페이지")
		assert.Contains(t, msg, "올바른 URL 형식")
	})

	t.Run("max 검증 (문자열 - 짧은 케이스)", func(t *testing.T) {
		req := &TestStruct{
			Name:     "Lee",
			Age:      20,
			Email:    "test@example.com",
			Homepage: "https://example.com",
			Bio:      "Too long bio description",
		}
		err := ValidateRequest(req)
		assert.Error(t, err)
		msg := FormatValidationError(err)
		assert.Contains(t, msg, "자기소개")
		assert.Contains(t, msg, "최대 10자")
	})

	t.Run("성공 케이스", func(t *testing.T) {
		req := &TestStruct{
			Name:     "Lee",
			Age:      20,
			Email:    "test@example.com",
			Homepage: "https://example.com",
			Bio:      "Short bio",
		}
		err := ValidateRequest(req)
		assert.NoError(t, err)
	})
}
