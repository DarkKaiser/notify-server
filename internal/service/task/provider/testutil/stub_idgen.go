package testutil

import (
	"fmt"
	"sync"

	"github.com/darkkaiser/notify-server/internal/service/contract"
)

// StubIDGenerator 테스트용 단순 ID 생성기 (매번 고유 ID 반환)
type StubIDGenerator struct {
	counter int64
	mu      sync.Mutex
}

func (s *StubIDGenerator) New() contract.TaskInstanceID {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counter++
	return contract.TaskInstanceID(fmt.Sprintf("stub-id-%d", s.counter))
}
