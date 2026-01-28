package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/pkg/concurrency"
	applog "github.com/darkkaiser/notify-server/pkg/log"
)

// component Task 서비스의 Storage 로깅용 컴포넌트 이름
const component = "task.storage"

// defaultDataDirectory 작업 결과를 저장할 기본 디렉토리 이름입니다.
const defaultDataDirectory = "data"

// tempFilePattern 임시 파일 저장 시 사용되는 임시 파일의 이름 패턴입니다.
const tempFilePattern = "task-result-*.tmp"

// fileTaskResultStore 파일 시스템을 기반으로 작업 결과를 저장하는 저장소 구현체입니다.
//
// [파일 구조]
//   - task-{taskID}-{commandID}-{hash}.json: 작업 결과가 JSON 형식으로 저장됩니다.
//   - task-result-*.tmp: 파일 저장 중 생성되는 임시 파일입니다.
type fileTaskResultStore struct {
	baseDir string

	// locks 동일한 파일에 대한 동시 쓰기를 방지하기 위한 파일별 뮤텍스입니다.
	// 파일 경로를 키로 사용하여 각 파일마다 독립적인 락을 관리합니다.
	locks *concurrency.KeyedMutex[string]
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ contract.TaskResultStore = (*fileTaskResultStore)(nil)

// NewFileTaskResultStore 파일 시스템 기반의 작업 결과 저장소를 생성합니다.
//
// 이 함수는 작업 결과를 JSON 파일로 저장하고 관리하는 저장소를 초기화합니다.
// 초기화 과정에서 저장 디렉토리를 생성하고, 이전 실행에서 남은 임시 파일을 정리합니다.
//
// 매개변수:
//   - baseDir: 작업 결과 파일을 저장할 디렉토리 경로
//     빈 문자열("")을 전달하면 기본 디렉토리("data")를 사용합니다.
//     상대 경로를 전달하면 절대 경로로 자동 변환됩니다.
//
// 반환값:
//   - contract.TaskResultStore: 생성된 저장소 인터페이스
//   - error: 디렉토리 생성 실패 또는 권한 문제 발생 시 에러 반환
func NewFileTaskResultStore(dir string) (contract.TaskResultStore, error) {
	if dir == "" {
		dir = defaultDataDirectory
	}

	// 상대 경로를 절대 경로로 변환하여 경로 일관성을 보장합니다.
	// 이후 모든 파일 작업은 이 절대 경로를 기준으로 수행됩니다.
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, NewErrAbsPathConversionFailed(err)
	}

	// 저장소 초기화 시점에 디렉토리 생성 및 접근 권한을 미리 확인합니다.
	// 이를 통해 나중에 Save 작업 시 발생할 수 있는 에러를 조기에 발견할 수 있습니다.
	if err := os.MkdirAll(absDir, 0755); err != nil {
		return nil, NewErrDirectoryAccessFailed(err, absDir)
	}

	s := &fileTaskResultStore{
		baseDir: absDir,

		locks: concurrency.NewKeyedMutex[string](),
	}

	// 백그라운드에서 이전 실행 시 남은 오래된 임시 파일을 정리합니다.
	// 서버 시작 속도에 영향을 주지 않도록 비동기로 수행하며,
	// 정리 작업 실패가 서버 전체에 영향을 주지 않도록 패닉을 복구합니다.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				applog.WithComponentAndFields(component, applog.Fields{
					"baseDir": s.baseDir,
					"panic":   r,
				}).Error("임시 파일 정리 중단: 백그라운드 작업 패닉 발생")
			}
		}()

		s.cleanupStaleTempFiles()
	}()

	return s, nil
}

// cleanupStaleTempFiles 이전 실행에서 남겨진 오래된 임시 파일을 정리합니다.
//
// 이 함수는 저장소 초기화 시 백그라운드 고루틴에서 비동기로 실행되며,
// 비정상 종료(크래시, 강제 종료 등)로 인해 남겨진 임시 파일들을 정리합니다.
func (s *fileTaskResultStore) cleanupStaleTempFiles() {
	// 저장소 디렉토리의 모든 파일 목록을 읽어옵니다.
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		applog.WithComponentAndFields(component, applog.Fields{
			"dir":   s.baseDir,
			"error": err,
		}).Warn("임시 파일 정리 중단: 디렉토리 조회 실패")

		return
	}

	// 삭제 기준 시간: 현재 시간으로부터 1시간 이전
	// 이 시간보다 오래된 파일만 삭제하여 현재 사용 중인 파일을 보호합니다.
	threshold := time.Now().Add(-1 * time.Hour)

	for _, entry := range entries {
		// 디렉토리는 건너뜁니다 (파일만 처리)
		if entry.IsDir() {
			continue
		}

		// 임시 파일 패턴과 일치하는지 확인
		name := entry.Name()
		matched, _ := filepath.Match(tempFilePattern, name)
		if !matched {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			// 파일 정보 조회 실패 시 해당 파일은 건너뜁니다
			continue
		}

		// 최근 1시간 이내에 수정된 파일은 건너뜁니다.
		// 다른 프로세스가 현재 사용 중일 수 있으므로 삭제하지 않습니다.
		if info.ModTime().After(threshold) {
			continue
		}

		// 오래된 임시 파일 삭제 시도
		fullPath := filepath.Join(s.baseDir, name)
		if err := os.Remove(fullPath); err != nil {
			applog.WithComponentAndFields(component, applog.Fields{
				"file":  fullPath,
				"error": err,
			}).Warn("임시 파일 삭제 실패: 파일 제거 오류")
		} else {
			applog.WithComponentAndFields(component, applog.Fields{
				"file": fullPath,
			}).Info("임시 파일 삭제 완료: 이전 실행 잔존 파일 정리")
		}
	}
}

// Load 저장된 작업 결과를 파일에서 읽어옵니다.
//
// [동시성 제어]
// 읽기 작업에도 Lock을 적용하여 쓰기 중인 파일을 읽는 것을 방지합니다.
// 이를 통해 부분적으로 쓰여진 데이터를 읽는 문제를 예방합니다.
//
// [성능 최적화]
// Lock 보유 시간을 최소화하기 위해 I/O와 CPU 작업을 분리합니다:
// - Lock 내부: 파일 읽기 (I/O)
// - Lock 외부: JSON 역직렬화 (CPU)
func (s *fileTaskResultStore) Load(taskID contract.TaskID, commandID contract.TaskCommandID, v any) error {
	// 1단계: 입력 검증
	// v가 nil이 아닌 포인터인지 검증하여 잘못된 호출을 즉시 차단합니다.
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return ErrLoadRequiresPointer
	}

	// 2단계: 안전한 파일 경로 생성 (보안 검증 포함)
	filename, err := s.resolveSafePath(taskID, commandID)
	if err != nil {
		return err
	}

	// 3단계: Lock 획득 후 파일 읽기
	// 쓰기 작업과의 경합을 방지하기 위해 읽기에도 Lock을 적용합니다.
	// Windows 등 대소문자를 구분하지 않는 파일 시스템을 위해 Lock 키는 소문자로 정규화합니다.
	var data []byte
	err = s.locks.WithLock(strings.ToLower(filename), func() error {
		var readErr error
		data, readErr = os.ReadFile(filename)
		if readErr != nil {
			// 파일이 아직 생성되지 않은 경우 (첫 실행 등)
			if os.IsNotExist(readErr) {
				return contract.ErrTaskResultNotFound
			}

			return NewErrTaskResultReadFailed(readErr)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// 4단계: JSON 역직렬화
	if err := json.Unmarshal(data, v); err != nil {
		return NewErrJSONUnmarshalFailed(err)
	}

	return nil
}

// Save 작업 결과를 파일에 저장합니다.
//
// [저장 전략: 원자적 쓰기]
// 파일 저장 중 시스템 장애(전원 차단, 프로세스 종료 등)가 발생해도 데이터 무결성을 보장하기 위해 원자적 쓰기 방식을 사용합니다:
// 1. 임시 파일에 먼저 쓰기
// 2. 디스크 동기화(fsync)로 물리적 저장 보장
// 3. 원자적 이름 변경(rename)으로 최종 파일 생성
//
// [동시성 제어]
// 같은 파일에 대한 동시 쓰기를 방지하기 위해 파일별 뮤텍스(KeyedMutex)를 사용합니다.
// 이를 통해 여러 고루틴이 동시에 Save를 호출해도 안전하게 처리됩니다.
func (s *fileTaskResultStore) Save(taskID contract.TaskID, commandID contract.TaskCommandID, v any) error {
	// 1단계: 안전한 파일 경로 생성 (보안 검증 포함)
	filename, err := s.resolveSafePath(taskID, commandID)
	if err != nil {
		return err
	}

	// 2단계: JSON 직렬화 (Lock 획득 전 수행)
	data, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return NewErrJSONMarshalFailed(err)
	}

	// 3단계: 파일별 Lock 획득 후 원자적 쓰기
	// Windows 등 대소문자를 구분하지 않는 파일 시스템을 위해 Lock 키는 소문자로 정규화합니다.
	return s.locks.WithLock(strings.ToLower(filename), func() error {
		return s.writeAtomic(filename, data)
	})
}

// resolveSafePath TaskID, CommandID를 사용하여 안전하게 검증된 파일 경로를 생성합니다.
//
// 이 함수는 단순히 경로를 조합하는 것을 넘어, 생성된 경로가 허용된 기본 디렉토리를
// 벗어나지 않는지 엄격하게 검증하여 Path Traversal 공격을 방어합니다.
//
// 반환값:
//   - string: 검증이 완료된 안전한 절대 경로
//   - error: 보안 정책 위반 또는 경로 생성 실패 시 에러
func (s *fileTaskResultStore) resolveSafePath(taskID contract.TaskID, commandID contract.TaskCommandID) (string, error) {
	filename := generateFilename(taskID, commandID)

	// 1. 기본 디렉토리 준비
	// 생성자에서 이미 절대 경로로 변환되었으므로 신뢰할 수 있는 기준 경로입니다.
	basePath := s.baseDir

	// 2. 절대 경로 조립 및 정규화
	// filepath.Join과 Clean을 통해 경로 구분자를 통일하고 불필요한 요소(..)를 정리합니다.
	fullPath := filepath.Join(basePath, filename)
	cleanPath := filepath.Clean(fullPath)

	// 3. 보안 검증
	// 생성된 최종 경로가 BaseDir의 하위 경로인지 확인합니다.
	//
	// [보안 검증 전략]
	// filepath.Rel을 사용하여 BaseDir 기준의 상대 경로를 계산하여 검증합니다.
	//
	// [이점]
	// 1. 경로 이탈 차단: 상대 경로가 ".."으로 시작하면 상위 디렉토리 접근으로 간주하여 차단합니다.
	// 2. 정교한 경로 비교: 단순 접두사(Prefix) 비교 취약점(Sibling Directory Attack)을 근본적으로 해결합니다.
	// 3. 호환성: 루트 디렉토리("C:\")나 경로 구분자 차이와 관계없이 일관된 검증을 보장합니다.
	rel, err := filepath.Rel(basePath, cleanPath)
	if err != nil {
		// 경로 계산 실패면 안전하지 않은 것으로 간주
		return "", NewErrPathResolutionFailed(err)
	}

	// 상대 경로가 ".."으로 시작하면 상위 디렉토리로 이탈한 것입니다.
	if strings.HasPrefix(rel, "..") {
		applog.WithComponentAndFields(component, applog.Fields{
			"task_id":    taskID,
			"command_id": commandID,
			"filename":   filename,
			"base_dir":   s.baseDir,
			"path":       cleanPath,
			"rel_path":   rel,
		}).Error("파일 경로 생성 차단: 경로 이탈 시도 감지")

		return "", ErrPathTraversalDetected
	}

	return cleanPath, nil
}

// writeAtomic 데이터를 파일에 원자적으로 저장합니다.
//
// [원자적 쓰기 전략]
// 파일 저장 중 시스템 장애(전원 차단, 프로세스 종료)가 발생해도 데이터 무결성을 보장하기 위해
// "임시 파일 쓰기 → 동기화 → 원자적 이름 변경" 3단계 전략을 사용합니다:
//
// 1. 임시 파일 생성 및 쓰기
//   - 같은 디렉토리 내에 임시 파일을 생성 (크로스 파일시스템 rename 방지)
//   - 데이터를 임시 파일에 완전히 기록
//
// 2. 디스크 동기화 (fsync)
//   - 파일 내용을 물리적 디스크에 강제 기록
//   - 운영체제 버퍼 캐시에만 있는 상태에서 전원이 차단되는 것을 방지
//
// 3. 원자적 이름 변경 (Atomic Rename)
//   - os.Rename은 POSIX 및 현대 Windows(Go 1.5+)에서 원자적 덮어쓰기를 보장
//   - 기존 파일이 있어도 중간 상태 없이 완전히 교체됨
func (s *fileTaskResultStore) writeAtomic(filename string, data []byte) error {
	dir := filepath.Dir(filename)

	// 1단계: 디렉토리 준비
	if err := os.MkdirAll(dir, 0755); err != nil {
		return NewErrDirectoryCreationFailed(err)
	}

	// 2단계: 임시 파일 생성
	// 같은 디렉토리 내에 생성해야 rename이 원자적으로 동작합니다.
	tmpFile, err := os.CreateTemp(dir, tempFilePattern)
	if err != nil {
		return NewErrTempFileCreationFailed(err)
	}
	tmpPath := tmpFile.Name()

	// [임시 파일 안전 정리: Windows 호환성]
	// 이 코드는 함수 종료 시 임시 파일을 확실하게 정리하기 위한 안전 장치입니다.
	//
	// 특별히 주의할 점은 Windows 운영체제의 파일 잠금 정책입니다.
	// Windows에서는 파일이 열려있는 상태에서는 삭제(os.Remove)가 불가능합니다.
	// 따라서 반드시 '파일 닫기(Close)'가 '파일 삭제(Remove)'보다 먼저 실행되어야 합니다.
	defer os.Remove(tmpPath)
	defer tmpFile.Close()

	// 3단계: 데이터 쓰기
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return NewErrFileWriteFailed(err)
	}

	// 4단계: 파일 내용 동기화 (fsync)
	// 운영체제 버퍼 캐시에 있는 데이터를 물리적 디스크에 강제로 기록합니다.
	// 이 단계를 생략하면 전원 차단 시 데이터가 유실될 수 있습니다.
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return NewErrFileSyncFailed(err)
	}

	// 5단계: 파일 닫기
	// Windows에서는 파일이 열려 있으면 rename이 실패하므로 반드시 닫아야 합니다.
	if err := tmpFile.Close(); err != nil {
		return NewErrFileCloseFailed(err)
	}

	// 6단계: 이름 변경
	// 임시 파일을 최종 파일명으로 변경합니다.
	if err := s.renameWithRetry(tmpPath, filename); err != nil {
		return NewErrFileRenameFailed(err)
	}

	// 7단계: 디렉토리 엔트리 동기화 (Directory fsync)
	// 파일명 변경 사항을 디스크에 확실히 기록하기 위해 부모 디렉토리를 fsync합니다.
	// 이를 수행하지 않으면 전원 유실 시 파일이 사라질 수 있습니다.
	// 실패해도 치명적이지 않으므로 에러를 무시합니다.
	if dirFile, err := os.Open(dir); err == nil {
		_ = dirFile.Sync()
		dirFile.Close()
	}

	return nil
}

// renameWithRetry 파일 이름 변경을 재시도 로직과 함께 수행합니다.
//
// [필요성: 개발 환경(Windows) 호환성]
// Windows 개발 환경에서는 다음과 같은 프로세스들이 파일을 일시적으로 잠글 수 있습니다:
// - 바이러스 백신 소프트웨어 (실시간 스캔)
// - Windows Search Indexer (파일 인덱싱)
// - 백업 프로그램
//
// 이러한 프로세스들이 파일을 점유하고 있을 때 os.Rename이 실패할 수 있으므로,
// 짧은 대기 후 재시도하여 일시적인 잠금 문제를 우회합니다.
//
// [운영 환경(Linux) 영향]
// Linux에서는 이러한 파일 잠금 문제가 거의 발생하지 않으므로 재시도 로직이 불필요하지만,
// 개발 환경과의 일관성을 위해 유지합니다. Linux에서도 해가 되지 않으며 (최대 50ms 지연),
// 만약 실패하더라도 즉시 에러를 반환하여 문제를 감지할 수 있습니다.
func (s *fileTaskResultStore) renameWithRetry(oldPath, newPath string) error {
	const maxRetries = 5
	const retryDelay = 10 * time.Millisecond

	var lastErr error
	for range maxRetries {
		err := os.Rename(oldPath, newPath)
		if err == nil {
			return nil
		}

		// 실패: 마지막 에러 저장 후 재시도
		lastErr = err
		time.Sleep(retryDelay)
	}

	return lastErr
}
