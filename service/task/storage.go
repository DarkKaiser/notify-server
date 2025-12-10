package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/strutil"
)

// TaskResultStorage Task 실행 결과를 저장하고 불러오는 저장소 인터페이스
type TaskResultStorage interface {
	Load(taskID ID, commandID CommandID, v interface{}) error
	Save(taskID ID, commandID CommandID, v interface{}) error
}

// FileTaskResultStorage 파일 시스템 기반의 Task 결과 저장소 구현체
type FileTaskResultStorage struct {
	appName string
	mu      sync.Mutex // 파일 쓰기 동시성 제어를 위한 뮤텍스 (선택 사항, 파일별 락이 더 좋을 수 있음)
}

// NewFileTaskResultStorage 새로운 파일 기반 저장소를 생성합니다.
func NewFileTaskResultStorage(appName string) *FileTaskResultStorage {
	return &FileTaskResultStorage{
		appName: appName,
	}
}

func (s *FileTaskResultStorage) dataFileName(taskID ID, commandID CommandID) string {
	filename := fmt.Sprintf("%s-task-%s-%s.json", s.appName, strutil.ToSnakeCase(string(taskID)), strutil.ToSnakeCase(string(commandID)))
	return strings.ReplaceAll(filename, "_", "-")
}

// Load 저장된 Task 결과를 파일에서 읽어옵니다.
func (s *FileTaskResultStorage) Load(taskID ID, commandID CommandID, v interface{}) error {
	filename := s.dataFileName(taskID, commandID)
	data, err := os.ReadFile(filename)
	if err != nil {
		// 아직 데이터 파일이 생성되기 전이라면 nil을 반환한다.
		var pathError *os.PathError
		if errors.As(err, &pathError) {
			return nil
		}

		return apperrors.Wrap(err, apperrors.ErrInternal, "작업 결과 데이터 파일을 읽는데 실패했습니다")
	}

	return json.Unmarshal(data, v)
}

// Save Task 결과를 파일에 저장합니다.
func (s *FileTaskResultStorage) Save(taskID ID, commandID CommandID, v interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return apperrors.Wrap(err, apperrors.ErrInternal, "작업 결과 데이터 마샬링에 실패했습니다")
	}

	filename := s.dataFileName(taskID, commandID)
	// 파일 권한 0644: Owner(RW), Group(R), Others(R)
	if err := os.WriteFile(filename, data, os.FileMode(0644)); err != nil {
		return apperrors.Wrap(err, apperrors.ErrInternal, "작업 결과 데이터 파일 쓰기에 실패했습니다")
	}

	return nil
}
