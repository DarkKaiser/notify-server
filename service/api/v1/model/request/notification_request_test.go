package request

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotificationRequest_Table(t *testing.T) {
	longMessage := "This is a very long message. It contains multiple sentences and should be handled properly. The system should be able to process messages of various lengths."

	tests := []struct {
		name             string
		input            *NotificationRequest
		expectedAppID    string
		expectedMessage  string
		expectedErrOccur bool
		verify           func(*testing.T, *NotificationRequest)
	}{
		{
			name: "Standard Request",
			input: &NotificationRequest{
				ApplicationID: "app-123",
				Message:       "Test notification message",
				ErrorOccurred: false,
			},
			expectedAppID:    "app-123",
			expectedMessage:  "Test notification message",
			expectedErrOccur: false,
		},
		{
			name: "Error Occurred Request",
			input: &NotificationRequest{
				ApplicationID: "app-456",
				Message:       "Error occurred!",
				ErrorOccurred: true,
			},
			expectedAppID:    "app-456",
			expectedMessage:  "Error occurred!",
			expectedErrOccur: true,
		},
		{
			name:             "Empty Request",
			input:            &NotificationRequest{},
			expectedAppID:    "",
			expectedMessage:  "",
			expectedErrOccur: false,
		},
		{
			name: "Long Message Request",
			input: &NotificationRequest{
				Message: longMessage,
			},
			expectedMessage: longMessage,
			verify: func(t *testing.T, req *NotificationRequest) {
				assert.Greater(t, len(req.Message), 100, "Message length should be greater than 100")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Field assertions
			if tt.expectedAppID != "" || tt.input.ApplicationID == "" {
				assert.Equal(t, tt.expectedAppID, tt.input.ApplicationID)
			}
			if tt.expectedMessage != "" || tt.input.Message == "" {
				assert.Equal(t, tt.expectedMessage, tt.input.Message)
			}
			assert.Equal(t, tt.expectedErrOccur, tt.input.ErrorOccurred)

			if tt.verify != nil {
				tt.verify(t, tt.input)
			}
		})
	}
}
