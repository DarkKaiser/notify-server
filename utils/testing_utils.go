package utils

// MockErrorHandler는 테스트용 에러 핸들러입니다.
// 다른 패키지의 테스트에서도 사용할 수 있도록 export합니다.
type MockErrorHandler struct {
	HandledError error
	Called       bool
}

// Handle은 에러를 기록하고 호출 여부를 표시합니다.
func (m *MockErrorHandler) Handle(err error) {
	m.Called = true
	m.HandledError = err
}

// Reset은 MockErrorHandler의 상태를 초기화합니다.
func (m *MockErrorHandler) Reset() {
	m.Called = false
	m.HandledError = nil
}
