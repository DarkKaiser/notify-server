package request

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var validate = validator.New()

func TestNotificationRequest_Validation(t *testing.T) {
	// 4097자를 생성 (최대 길이 4096 초과 테스트용)
	longMessage := strings.Repeat("a", 4097)

	tests := []struct {
		name        string
		input       *NotificationRequest
		wantErr     bool
		errTag      string // 기대되는 에러 태그 (예: "required", "max")
		errField    string // 기대되는 에러 필드 (예: "ApplicationID", "Message")
		description string
	}{
		{
			name: "Valid Request",
			input: &NotificationRequest{
				ApplicationID: "app-123",
				Message:       "Valid message",
				ErrorOccurred: false,
			},
			wantErr:     false,
			description: "정상적인 요청은 유효성 검사를 통과해야 합니다.",
		},
		{
			name: "Valid Request with Max Length Message",
			input: &NotificationRequest{
				ApplicationID: "app-123",
				Message:       strings.Repeat("a", 4096),
			},
			wantErr:     false,
			description: "메시지 길이가 정확히 4096자인 경우 유효성 검사를 통과해야 합니다.",
		},
		{
			name: "Missing ApplicationID",
			input: &NotificationRequest{
				ApplicationID: "",
				Message:       "Valid message",
			},
			wantErr:     true,
			errTag:      "required",
			errField:    "ApplicationID",
			description: "ApplicationID가 없으면 validation 에러가 발생해야 합니다.",
		},
		{
			name: "Missing Message",
			input: &NotificationRequest{
				ApplicationID: "app-123",
				Message:       "",
			},
			wantErr:     true,
			errTag:      "required",
			errField:    "Message",
			description: "Message가 없으면 validation 에러가 발생해야 합니다.",
		},
		{
			name: "Message Too Long",
			input: &NotificationRequest{
				ApplicationID: "app-123",
				Message:       longMessage,
			},
			wantErr:     true,
			errTag:      "max",
			errField:    "Message",
			description: "Message가 4096자를 초과하면 validation 에러가 발생해야 합니다.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)

			if tt.wantErr {
				require.Error(t, err, "에러가 발생해야 합니다")

				// 에러 타입 검증
				validationErrors, ok := err.(validator.ValidationErrors)
				require.True(t, ok, "에러는 validator.ValidationErrors 타입이어야 합니다")

				// 첫 번째 에러 확인
				found := false
				for _, fieldError := range validationErrors {
					if fieldError.Field() == tt.errField && fieldError.Tag() == tt.errTag {
						found = true
						break
					}
				}
				assert.True(t, found, "기대하는 필드(%s)에서 태그(%s) 에러가 발생해야 합니다. 실제 에러: %v", tt.errField, tt.errTag, validationErrors)
			} else {
				assert.NoError(t, err, "에러가 발생하지 않아야 합니다")
			}
		})
	}
}

func TestNotificationRequest_JSON(t *testing.T) {
	tests := []struct {
		name     string
		jsonBody string
		expected *NotificationRequest
		wantErr  bool
	}{
		{
			name: "Full JSON",
			jsonBody: `{
				"application_id": "test-app",
				"message": "test message",
				"error_occurred": true
			}`,
			expected: &NotificationRequest{
				ApplicationID: "test-app",
				Message:       "test message",
				ErrorOccurred: true,
			},
			wantErr: false,
		},
		{
			name: "Optional Field ErrorOccurred Missing (Default False)",
			jsonBody: `{
				"application_id": "test-app",
				"message": "test message"
			}`,
			expected: &NotificationRequest{
				ApplicationID: "test-app",
				Message:       "test message",
				ErrorOccurred: false,
			},
			wantErr: false,
		},
		{
			name:     "Invalid JSON",
			jsonBody: `{"application_id": "broken-json...`,
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req NotificationRequest
			err := json.Unmarshal([]byte(tt.jsonBody), &req)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, &req)
			}
		})
	}
}
