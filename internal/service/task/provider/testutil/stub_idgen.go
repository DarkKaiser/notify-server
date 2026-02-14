package testutil

import (
	"fmt"
	"sync/atomic"

	"github.com/darkkaiser/notify-server/internal/service/contract"
)

// StubIDGenerator 테스트용 단순 ID 생성기 (매번 고유 ID 반환)
type StubIDGenerator struct {
	counter int64

	// Prefix 생성된 ID 앞에 붙을 접두사입니다. (기본값: "stub-id-")
	Prefix string

	// FixedID 설정된 경우 항상 이 ID를 반환합니다.
	FixedID contract.TaskInstanceID
}

func (s *StubIDGenerator) New() contract.TaskInstanceID {
	if s.FixedID != "" {
		return s.FixedID
	}

	prefix := s.Prefix
	if prefix == "" {
		prefix = "stub-id-"
	}

	id := atomic.AddInt64(&s.counter, 1)
	return contract.TaskInstanceID(fmt.Sprintf("%s%d", prefix, id))
}
