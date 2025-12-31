package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/concurrency"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	log "github.com/sirupsen/logrus"
)

// defaultDataDirectory 기본 데이터 저장 디렉토리 이름
const defaultDataDirectory = "data"

// TaskResultStorage Task 실행 결과를 저장하고 불러오는 저장소 인터페이스
type TaskResultStorage interface {
	Load(taskID ID, commandID CommandID, v interface{}) error
	Save(taskID ID, commandID CommandID, v interface{}) error
}

// FileTaskResultStorage 파일 시스템 기반의 Task 결과 저장소 구현체
type FileTaskResultStorage struct {
	appName string

	baseDir string // 데이터 저장 디렉토리

	locks *concurrency.KeyedMutex[string] // 파일별 락킹을 위한 KeyedMutex
}

// NewFileTaskResultStorage 새로운 파일 기반 저장소를 생성합니다.
// 기본 저장 디렉토리는 "data" 입니다.
func NewFileTaskResultStorage(appName string) *FileTaskResultStorage {
	s := &FileTaskResultStorage{
		appName: appName,

		baseDir: defaultDataDirectory,

		locks: concurrency.NewKeyedMutex[string](),
	}

	// 시작 시 오래된 임시 파일 정리 (Best Effort)
	s.cleanupTempFiles()

	return s
}

// SetBaseDir 데이터 저장 디렉토리를 변경합니다.
func (s *FileTaskResultStorage) SetBaseDir(dir string) {
	s.baseDir = dir
}

// cleanupTempFiles 작업 도중 비정상 종료 등으로 남겨진 임시 파일(*.tmp)을 정리합니다.
func (s *FileTaskResultStorage) cleanupTempFiles() {
	pattern := filepath.Join(s.baseDir, "task-result-*.tmp")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		applog.WithComponentAndFields("storage", log.Fields{
			"pattern": pattern,
			"error":   err,
		}).Warn("임시 파일 정리 중 패턴 매칭 실패")
		return
	}

	for _, match := range matches {
		if err := os.Remove(match); err != nil {
			applog.WithComponentAndFields("storage", log.Fields{
				"file":  match,
				"error": err,
			}).Warn("남겨진 임시 파일 삭제 실패")
		} else {
			applog.WithComponentAndFields("storage", log.Fields{
				"file": match,
			}).Info("남겨진 임시 파일을 삭제했습니다")
		}
	}
}

func (s *FileTaskResultStorage) resolvePath(taskID ID, commandID CommandID) (string, error) {
	// 입력값 보안 검증: Path Traversal 문자가 포함되어 있는지 확인
	// strutil.ToSnakeCase 변환 전에 검증해야 함 (변환 과정에서 위험 문자가 사라질 수 있음)
	if strings.Contains(string(taskID), "..") || strings.Contains(string(taskID), "/") || strings.Contains(string(taskID), "\\") {
		return "", apperrors.New(apperrors.Internal, "TaskID에 유효하지 않은 문자가 포함되어 있습니다 (Path Traversal Detected)")
	}
	if strings.Contains(string(commandID), "..") || strings.Contains(string(commandID), "/") || strings.Contains(string(commandID), "\\") {
		return "", apperrors.New(apperrors.Internal, "CommandID에 유효하지 않은 문자가 포함되어 있습니다 (Path Traversal Detected)")
	}

	filename := fmt.Sprintf("%s-task-%s-%s.json", s.appName, strutil.ToSnakeCase(string(taskID)), strutil.ToSnakeCase(string(commandID)))
	filename = strings.ReplaceAll(filename, "_", "-")

	// Base 디렉토리의 절대 경로
	basePath, err := filepath.Abs(s.baseDir)
	if err != nil {
		return "", apperrors.Wrap(err, apperrors.Internal, "데이터 디렉토리 절대 경로 확인 실패")
	}

	// 타겟 파일의 절대 경로
	fullPath := filepath.Join(basePath, filename)
	cleanPath := filepath.Clean(fullPath)

	// Path Traversal 검사: 생성된 경로가 반드시 Base 디렉토리로 시작해야 함
	if !strings.HasPrefix(cleanPath, basePath) {
		applog.WithComponentAndFields("storage", log.Fields{
			"task_id":    taskID,
			"command_id": commandID,
			"path":       cleanPath,
		}).Error("비정상적인 파일 경로 접근 시도가 감지되었습니다")
		return "", apperrors.New(apperrors.Internal, "유효하지 않은 파일 경로입니다 (Path Traversal Detected)")
	}

	return cleanPath, nil
}

// Load 저장된 Task 결과를 파일에서 읽어옵니다.
func (s *FileTaskResultStorage) Load(taskID ID, commandID CommandID, v interface{}) error {
	filename, err := s.resolvePath(taskID, commandID)
	if err != nil {
		return err
	}

	// 읽기 시에도 락을 걸어서 쓰기 중인 파일에 접근하는 것을 방지합니다.
	return s.locks.WithLock(filename, func() error {
		data, err := os.ReadFile(filename)
		if err != nil {
			// 아직 데이터 파일이 생성되기 전이라면 nil을 반환한다.
			var pathError *os.PathError
			if errors.As(err, &pathError) {
				return nil
			}

			return apperrors.Wrap(err, apperrors.Internal, "작업 결과 데이터 파일을 읽는데 실패했습니다")
		}

		return json.Unmarshal(data, v)
	})
}

// Save Task 결과를 파일에 저장합니다. (Atomic Write 적용)
func (s *FileTaskResultStorage) Save(taskID ID, commandID CommandID, v interface{}) error {
	filename, err := s.resolvePath(taskID, commandID)
	if err != nil {
		return err
	}

	return s.locks.WithLock(filename, func() error {
		data, err := json.MarshalIndent(v, "", "\t")
		if err != nil {
			return apperrors.Wrap(err, apperrors.Internal, "작업 결과 데이터 마샬링에 실패했습니다")
		}

		// filename은 상단에서 이미 획득함
		dir := filepath.Dir(filename)

		// 디렉토리가 없으면 생성
		if err := os.MkdirAll(dir, 0755); err != nil {
			return apperrors.Wrap(err, apperrors.Internal, "데이터 디렉토리 생성에 실패했습니다")
		}

		// 임시 파일 생성 (같은 디렉토리 내에 생성해야 Rename이 안전함)
		tmpFile, err := os.CreateTemp(dir, "task-result-*.tmp")
		if err != nil {
			return apperrors.Wrap(err, apperrors.Internal, "임시 파일 생성에 실패했습니다")
		}
		tmpName := tmpFile.Name()

		// 확실한 cleanup을 위해 defer로 삭제 시도 (Rename 성공 시에는 에러 무시됨)
		defer os.Remove(tmpName)

		// 데이터 쓰기
		if _, err := tmpFile.Write(data); err != nil {
			tmpFile.Close()
			return apperrors.Wrap(err, apperrors.Internal, "임시 파일 쓰기에 실패했습니다")
		}

		// 파일 닫기 (Windows에서는 닫지 않으면 Rename 불가)
		if err := tmpFile.Close(); err != nil {
			return apperrors.Wrap(err, apperrors.Internal, "임시 파일 닫기에 실패했습니다")
		}

		// Windows 호환성을 위한 원본 파일 삭제
		// (Linux 등에서는 Rename이 Atomic하게 덮어쓰지만, Windows는 타겟이 있으면 실패할 수 있음)
		if _, err := os.Stat(filename); err == nil {
			if err := os.Remove(filename); err != nil {
				return apperrors.Wrap(err, apperrors.Internal, "기존 파일 삭제에 실패했습니다")
			}
		}

		// 임시 파일을 원본 파일명으로 변경
		if err := os.Rename(tmpName, filename); err != nil {
			return apperrors.Wrap(err, apperrors.Internal, "파일 이름 변경(저장)에 실패했습니다")
		}

		return nil
	})
}
