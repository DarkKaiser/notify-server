package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	baseDir string     // 데이터 저장 디렉토리
	mu      sync.Mutex // 파일 쓰기 동시성 제어를 위한 뮤텍스
}

// NewFileTaskResultStorage 새로운 파일 기반 저장소를 생성합니다.
// 기본 저장 디렉토리는 "data" 입니다.
func NewFileTaskResultStorage(appName string) *FileTaskResultStorage {
	return &FileTaskResultStorage{
		appName: appName,
		baseDir: "data",
	}
}

// SetBaseDir 데이터 저장 디렉토리를 변경합니다. (주로 테스트용)
func (s *FileTaskResultStorage) SetBaseDir(dir string) {
	s.baseDir = dir
}

func (s *FileTaskResultStorage) dataFileName(taskID ID, commandID CommandID) string {
	filename := fmt.Sprintf("%s-task-%s-%s.json", s.appName, strutil.ToSnakeCase(string(taskID)), strutil.ToSnakeCase(string(commandID)))
	filename = strings.ReplaceAll(filename, "_", "-")
	return filepath.Join(s.baseDir, filename)
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

// Save Task 결과를 파일에 저장합니다. (Atomic Write 적용)
func (s *FileTaskResultStorage) Save(taskID ID, commandID CommandID, v interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return apperrors.Wrap(err, apperrors.ErrInternal, "작업 결과 데이터 마샬링에 실패했습니다")
	}

	filename := s.dataFileName(taskID, commandID)
	dir := filepath.Dir(filename)

	// 디렉토리가 없으면 생성
	if err := os.MkdirAll(dir, 0755); err != nil {
		return apperrors.Wrap(err, apperrors.ErrInternal, "데이터 디렉토리 생성에 실패했습니다")
	}

	// 임시 파일 생성 (같은 디렉토리 내에 생성해야 Rename이 안전함)
	tmpFile, err := os.CreateTemp(dir, "task-result-*.tmp")
	if err != nil {
		return apperrors.Wrap(err, apperrors.ErrInternal, "임시 파일 생성에 실패했습니다")
	}
	tmpName := tmpFile.Name()

	// 확실한 cleanup을 위해 defer로 삭제 시도 (Rename 성공 시에는 에러 무시됨)
	defer os.Remove(tmpName)

	// 데이터 쓰기
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return apperrors.Wrap(err, apperrors.ErrInternal, "임시 파일 쓰기에 실패했습니다")
	}

	// 파일 닫기 (Windows에서는 닫지 않으면 Rename 불가)
	if err := tmpFile.Close(); err != nil {
		return apperrors.Wrap(err, apperrors.ErrInternal, "임시 파일 닫기에 실패했습니다")
	}

	// Windows 호환성을 위한 원본 파일 삭제
	// (Linux 등에서는 Rename이 Atomic하게 덮어쓰지만, Windows는 타겟이 있으면 실패할 수 있음)
	if _, err := os.Stat(filename); err == nil {
		if err := os.Remove(filename); err != nil {
			return apperrors.Wrap(err, apperrors.ErrInternal, "기존 파일 삭제에 실패했습니다")
		}
	}

	// 임시 파일을 원본 파일명으로 변경
	if err := os.Rename(tmpName, filename); err != nil {
		return apperrors.Wrap(err, apperrors.ErrInternal, "파일 이름 변경(저장)에 실패했습니다")
	}

	return nil
}
