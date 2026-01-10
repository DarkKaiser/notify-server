package testutil

import (
	"fmt"
	"net"
	"time"
)

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
