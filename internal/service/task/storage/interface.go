package storage

import (
	"github.com/darkkaiser/notify-server/internal/service/contract"
)

// TaskResultStorage Task 실행 결과를 저장하고 불러오는 저장소 인터페이스
type TaskResultStorage interface {
	Load(taskID contract.TaskID, commandID contract.TaskCommandID, v interface{}) error
	Save(taskID contract.TaskID, commandID contract.TaskCommandID, v interface{}) error
}
