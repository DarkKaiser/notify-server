package handler

// MockNotificationService is a mock implementation of NotificationService
type MockNotificationService struct {
	NotifyCalled      bool
	LastNotifierID    string
	LastTitle         string
	LastMessage       string
	LastErrorOccurred bool
}

func (m *MockNotificationService) Notify(notifierID string, title string, message string, errorOccurred bool) bool {
	m.NotifyCalled = true
	m.LastNotifierID = notifierID
	m.LastTitle = title
	m.LastMessage = message
	m.LastErrorOccurred = errorOccurred
	return true
}

func (m *MockNotificationService) NotifyToDefault(message string) bool {
	return true
}

func (m *MockNotificationService) NotifyWithErrorToDefault(message string) bool {
	return true
}
