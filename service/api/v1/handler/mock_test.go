package handler

// MockNotificationSender is a mock implementation of NotificationSender
type MockNotificationSender struct {
	NotifyCalled      bool
	LastNotifierID    string
	LastTitle         string
	LastMessage       string
	LastErrorOccurred bool
}

func (m *MockNotificationSender) Notify(notifierID string, title string, message string, errorOccurred bool) bool {
	m.NotifyCalled = true
	m.LastNotifierID = notifierID
	m.LastTitle = title
	m.LastMessage = message
	m.LastErrorOccurred = errorOccurred
	return true
}

func (m *MockNotificationSender) NotifyToDefault(message string) bool {
	return true
}

func (m *MockNotificationSender) NotifyWithErrorToDefault(message string) bool {
	return true
}
