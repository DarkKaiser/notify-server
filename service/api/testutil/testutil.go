package testutil

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// MockNotificationService NotificationService의 테스트용 Mock 구현체입니다.
// service/api 및 하위 패키지(v1/handler 등) 테스트에서 공통으로 사용됩니다.
type MockNotificationService struct {
	mu sync.Mutex

	NotifyCalled      bool
	LastNotifierID    string
	LastTitle         string
	LastMessage       string
	LastErrorOccurred bool
	ShouldFail        bool

	NotifyDefaultCalled bool
}

func (m *MockNotificationService) NotifyWithTitle(notifierID string, title string, message string, errorOccurred bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NotifyCalled = true
	m.LastNotifierID = notifierID
	m.LastTitle = title
	m.LastMessage = message
	m.LastErrorOccurred = errorOccurred
	return !m.ShouldFail
}

func (m *MockNotificationService) NotifyDefault(message string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NotifyDefaultCalled = true
	m.LastMessage = message
	return true
}

func (m *MockNotificationService) NotifyDefaultWithError(message string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NotifyDefaultCalled = true
	m.LastMessage = message
	m.LastErrorOccurred = true
	return true
}

// Reset 상태를 초기화합니다.
func (m *MockNotificationService) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NotifyCalled = false
	m.LastNotifierID = ""
	m.LastTitle = ""
	m.LastMessage = ""
	m.LastErrorOccurred = false
	m.ShouldFail = false
	m.NotifyDefaultCalled = false
}

// GetFreePort 테스트용으로 사용 가능한 임의의 포트를 반환합니다.
func GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// WaitForServer 서버가 해당 포트에서 리스닝할 때까지 대기합니다.
func WaitForServer(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("server did not start on port %d within %v", port, timeout)
}
