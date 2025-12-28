package handler

import (
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/api/v1/model/request"
	"github.com/stretchr/testify/assert"
)

func TestValidateRequest_Table(t *testing.T) {
	longMessage := strings.Repeat("a", 4097)
	maxMessage := strings.Repeat("a", 4096)

	tests := []struct {
		name      string
		req       interface{} // Using interface{} to allow TestStruct as well
		expectErr bool
		errMsg    []string
	}{
		{
			name: "Valid Request",
			req: &request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "테스트 메시지",
				ErrorOccurred: false,
			},
			expectErr: false,
		},
		{
			name: "Missing ApplicationID",
			req: &request.NotificationRequest{
				ApplicationID: "",
				Message:       "테스트 메시지",
			},
			expectErr: true,
			errMsg:    []string{"애플리케이션 ID", "필수"},
		},
		{
			name: "Missing Message",
			req: &request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "",
			},
			expectErr: true,
			errMsg:    []string{"메시지", "필수"},
		},
		{
			name: "Message Too Long",
			req: &request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       longMessage,
			},
			expectErr: true,
			errMsg:    []string{"메시지", "최대", "4096"},
		},
		{
			name: "Message Max Length",
			req: &request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       maxMessage,
			},
			expectErr: false,
		},
		{
			name: "Message Min Length",
			req: &request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "a",
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequest(tt.req)
			if tt.expectErr {
				assert.Error(t, err)
				if len(tt.errMsg) > 0 {
					msg := FormatValidationError(err)
					for _, expect := range tt.errMsg {
						assert.Contains(t, msg, expect)
					}
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestStruct for validation tags coverage
type TestStruct struct {
	Name     string `validate:"required,min=2" korean:"이름"`
	Age      int    `validate:"min=18" korean:"나이"`
	Email    string `validate:"email" korean:"이메일"`
	Homepage string `validate:"url" korean:"홈페이지"`
	Bio      string `validate:"max=10" korean:"자기소개"`
}

func TestFormatValidationError_Tags_Table(t *testing.T) {
	tests := []struct {
		name   string
		req    *TestStruct
		errMsg []string
	}{
		{
			name:   "Min Len (String)",
			req:    &TestStruct{Name: "a", Age: 20, Email: "a@b.com", Homepage: "http://a.com"},
			errMsg: []string{"이름", "최소 2자"},
		},
		{
			name:   "Min Value (Int)",
			req:    &TestStruct{Name: "Lee", Age: 10, Email: "a@b.com", Homepage: "http://a.com"},
			errMsg: []string{"나이", "최소 18 이상"},
		},
		{
			name:   "Email Invalid",
			req:    &TestStruct{Name: "Lee", Age: 20, Email: "invalid", Homepage: "http://a.com"},
			errMsg: []string{"이메일", "올바른 이메일 형식"},
		},
		{
			name:   "URL Invalid",
			req:    &TestStruct{Name: "Lee", Age: 20, Email: "a@b.com", Homepage: "invalid"},
			errMsg: []string{"홈페이지", "올바른 URL 형식"},
		},
		{
			name:   "Max Len",
			req:    &TestStruct{Name: "Lee", Age: 20, Email: "a@b.com", Homepage: "http://a.com", Bio: "Too long bio"},
			errMsg: []string{"자기소개", "최대 10자"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequest(tt.req)
			assert.Error(t, err)
			msg := FormatValidationError(err)
			for _, expect := range tt.errMsg {
				assert.Contains(t, msg, expect)
			}
		})
	}
}

func TestFormatValidationError_Nil_Table(t *testing.T) {
	result := FormatValidationError(nil)
	assert.Equal(t, "", result)
}
