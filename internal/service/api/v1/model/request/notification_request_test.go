package request

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNotificationRequest_Validation NotificationRequest 구조체의 유효성 검사 규칙을 테스트합니다.
func TestNotificationRequest_Validation(t *testing.T) {
	validate := validator.New()

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
			description: "모든 필수 필드가 존재하고 제약조건을 만족하면 유효성 검사를 통과해야 합니다.",
		},
		{
			name: "Valid Request - Max Length Message",
			input: &NotificationRequest{
				ApplicationID: "app-123",
				Message:       strings.Repeat("a", 4096),
			},
			wantErr:     false,
			description: "메시지 길이가 정확히 4096자인 경우(최대 허용치) 유효성 검사를 통과해야 합니다.",
		},
		{
			name: "Invalid Request - Missing ApplicationID",
			input: &NotificationRequest{
				ApplicationID: "",
				Message:       "Valid message",
			},
			wantErr:     true,
			errTag:      "required",
			errField:    "ApplicationID",
			description: "ApplicationID가 빈 문자열이면 required 에러가 발생해야 합니다.",
		},
		{
			name: "Invalid Request - Missing Message",
			input: &NotificationRequest{
				ApplicationID: "app-123",
				Message:       "",
			},
			wantErr:     true,
			errTag:      "required",
			errField:    "Message",
			description: "Message가 빈 문자열이면 required 에러가 발생해야 합니다.",
		},
		{
			name: "Invalid Request - Message Too Long",
			input: &NotificationRequest{
				ApplicationID: "app-123",
				Message:       longMessage,
			},
			wantErr:     true,
			errTag:      "max",
			errField:    "Message",
			description: "Message가 4096자를 초과하면 max 에러가 발생해야 합니다.",
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

				// 기대하는 필드와 태그에 대한 에러가 포함되어 있는지 확인
				found := false
				for _, fieldError := range validationErrors {
					if fieldError.Field() == tt.errField && fieldError.Tag() == tt.errTag {
						found = true
						break
					}
				}
				assert.True(t, found, "기대하는 필드(%s)에서 태그(%s) 에러가 발생해야 합니다. 실제 에러: %v", tt.errField, tt.errTag, validationErrors)
			} else {
				assert.NoError(t, err, "정상적인 요청에 대해서는 에러가 발생하지 않아야 합니다")
			}
		})
	}
}

// TestNotificationRequest_JSON JSON 언마샬링 동작을 테스트합니다.
func TestNotificationRequest_JSON(t *testing.T) {
	tests := []struct {
		name        string
		jsonBody    string
		expected    *NotificationRequest
		wantErr     bool
		description string
	}{
		{
			name: "Success - Full Fields",
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
			wantErr:     false,
			description: "모든 필드가 포함된 정상적인 JSON 요청을 파싱할 수 있어야 합니다.",
		},
		{
			name: "Success - Optional Field Omitted",
			jsonBody: `{
				"application_id": "test-app",
				"message": "test message"
			}`,
			expected: &NotificationRequest{
				ApplicationID: "test-app",
				Message:       "test message",
				ErrorOccurred: false, // 기본값 false
			},
			wantErr:     false,
			description: "선택적 필드(error_occurred)가 누락되면 제로 값(false)으로 설정되어야 합니다.",
		},
		{
			name: "Success - Extra Fields Ignored",
			jsonBody: `{
				"application_id": "test-app",
				"message": "test message",
				"unknown_field": "some value"
			}`,
			expected: &NotificationRequest{
				ApplicationID: "test-app",
				Message:       "test message",
				ErrorOccurred: false,
			},
			wantErr:     false,
			description: "정의되지 않은 필드가 포함되어도 무시하고 정상적으로 파싱해야 합니다.",
		},
		{
			name:        "Failure - Invalid JSON Format",
			jsonBody:    `{"application_id": "broken-json...`,
			expected:    nil,
			wantErr:     true,
			description: "JSON 형식이 잘못된 경우 언마샬링 에러가 발생해야 합니다.",
		},
		{
			name: "Failure - Type Mismatch (Message)",
			jsonBody: `{
				"application_id": "test-app",
				"message": 12345
			}`,
			expected:    nil,
			wantErr:     true,
			description: "문자열 필드(message)에 숫자가 전달되면 타입 에러가 발생해야 합니다.",
		},
		{
			name: "Failure - Type Mismatch (ErrorOccurred)",
			jsonBody: `{
				"application_id": "test-app",
				"message": "msg",
				"error_occurred": "not-a-bool"
			}`,
			expected:    nil,
			wantErr:     true,
			description: "불리언 필드(error_occurred)에 문자열이 전달되면 타입 에러가 발생해야 합니다.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req NotificationRequest
			err := json.Unmarshal([]byte(tt.jsonBody), &req)

			if tt.wantErr {
				assert.Error(t, err, "에러가 발생해야 합니다: %s", tt.description)
			} else {
				assert.NoError(t, err, "에러가 발생하지 않아야 합니다: %s", tt.description)
				assert.Equal(t, tt.expected, &req, "파싱된 구조체가 기대값과 일치해야 합니다")
			}
		})
	}
}
