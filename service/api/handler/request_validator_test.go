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
