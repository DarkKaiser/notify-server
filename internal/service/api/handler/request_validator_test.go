package handler

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/api/v1/model/request"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

func TestGetValidator_Concurrency(t *testing.T) {
	var wg sync.WaitGroup
	const routines = 100
	validators := make([]*validator.Validate, routines)

	wg.Add(routines)
	for i := 0; i < routines; i++ {
		go func(index int) {
			defer wg.Done()
			validators[index] = getValidator()
		}(i)
	}
	wg.Wait()

	// Verify all returned instances are the same (singleton)
	first := validators[0]
	for i := 1; i < routines; i++ {
		assert.Same(t, first, validators[i], "All validator instances should be the same")
	}
}

func TestValidateRequest_Table(t *testing.T) {
	longMessage := strings.Repeat("a", 4097)
	maxMessage := strings.Repeat("a", 4096)

	tests := []struct {
		name      string
		req       interface{}
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
	Name       string `validate:"required,min=2" korean:"이름"`
	Age        int    `validate:"min=18,max=100" korean:"나이"` // test max int
	Email      string `validate:"email" korean:"이메일"`
	Homepage   string `validate:"url" korean:"홈페이지"`
	Bio        string `validate:"max=10" korean:"자기소개"`
	NoTagField string `validate:"required"`        // test fallback to field name
	UnknownTag string `validate:"alpha,omitempty"` // test default case
}

func TestFormatValidationError_Tags_Table(t *testing.T) {
	tests := []struct {
		name   string
		req    *TestStruct
		errMsg []string
	}{
		{
			name:   "Min Len (String)",
			req:    &TestStruct{Name: "a", Age: 20, Email: "a@b.com", Homepage: "http://a.com", NoTagField: "exist"},
			errMsg: []string{"이름", "최소 2자"},
		},
		{
			name:   "Min Value (Int)",
			req:    &TestStruct{Name: "Lee", Age: 10, Email: "a@b.com", Homepage: "http://a.com", NoTagField: "exist"},
			errMsg: []string{"나이", "최소 18 이상"},
		},
		{
			name:   "Max Value (Int)",
			req:    &TestStruct{Name: "Lee", Age: 101, Email: "a@b.com", Homepage: "http://a.com", NoTagField: "exist"},
			errMsg: []string{"나이", "최대 100까지"},
		},
		{
			name:   "Email Invalid",
			req:    &TestStruct{Name: "Lee", Age: 20, Email: "invalid", Homepage: "http://a.com", NoTagField: "exist"},
			errMsg: []string{"이메일", "올바른 이메일 형식"},
		},
		{
			name:   "URL Invalid",
			req:    &TestStruct{Name: "Lee", Age: 20, Email: "a@b.com", Homepage: "invalid", NoTagField: "exist"},
			errMsg: []string{"홈페이지", "올바른 URL 형식"},
		},
		{
			name:   "Max Len",
			req:    &TestStruct{Name: "Lee", Age: 20, Email: "a@b.com", Homepage: "http://a.com", Bio: "Too long bio", NoTagField: "exist"},
			errMsg: []string{"자기소개", "최대 10자"},
		},
		{
			name:   "Missing Korean Tag (Fallback to Field Name)",
			req:    &TestStruct{Name: "Lee", Age: 20, Email: "a@b.com", Homepage: "http://a.com", NoTagField: ""},
			errMsg: []string{"NoTagField", "필수"},
		},
		{
			name:   "Unknown Tag (Default Case)",
			req:    &TestStruct{Name: "Lee", Age: 20, Email: "a@b.com", Homepage: "http://a.com", NoTagField: "exist", UnknownTag: "123"}, // alpha fails for numbers
			errMsg: []string{"UnknownTag", "검증 실패", "alpha"},
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

func TestFormatValidationError_EdgeCases(t *testing.T) {
	t.Run("Nil Error", func(t *testing.T) {
		assert.Equal(t, "", FormatValidationError(nil))
	})

	t.Run("Non-Validation Error", func(t *testing.T) {
		err := errors.New("standard error")
		assert.Equal(t, "standard error", FormatValidationError(err))
	})

	t.Run("Empty Validation Errors", func(t *testing.T) {
		// Manually create an empty ValidationErrors slice
		emptyValErr := validator.ValidationErrors{}
		// It should behave like a non-validation error or return empty depending on implementation?
		// implementation: if len(validationErrors) == 0 { return err.Error() }
		// ValidationErrors.Error() returns a string representation.
		assert.Equal(t, emptyValErr.Error(), FormatValidationError(emptyValErr))
	})
}
