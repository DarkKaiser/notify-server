package storage

import (
	"fmt"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

var (
	// ErrPathTraversalDetected 파일 경로 생성 시 Path Traversal(경로 이탈) 시도가 감지되었을 때 반환하는 에러입니다.
	ErrPathTraversalDetected = apperrors.New(apperrors.Internal, "보안 정책 위반: 허용되지 않은 경로 접근 시도로 인해 요청이 차단되었습니다")

	// ErrLoadRequiresPointer Load 함수 호출 시 대상 객체가 올바른 포인터 타입이 아닐 때 반환하는 에러입니다.
	ErrLoadRequiresPointer = apperrors.New(apperrors.Internal, "내부 시스템 오류: 데이터 로드 대상 객체가 올바른 포인터 타입이 아닙니다")
)

// NewErrPathResolutionFailed 파일 경로 해석(절대 경로/상대 경로 변환 등)에 실패했을 때 반환하는 에러를 생성합니다.
func NewErrPathResolutionFailed(err error) error {
	return apperrors.Wrap(err, apperrors.Internal, "보안 검증 실패: 파일 경로를 해석할 수 없습니다")
}

// NewErrAbsPathConversionFailed 저장소 초기화 시 디렉토리 경로를 절대 경로로 변환하는 데 실패했을 때 반환하는 에러를 생성합니다.
func NewErrAbsPathConversionFailed(err error) error {
	return apperrors.Wrap(err, apperrors.Internal, "저장소 초기화 실패: 절대 경로 변환 불가")
}

// NewErrDirectoryAccessFailed 저장소 초기화 시 디렉토리 생성 또는 접근 권한 확인에 실패했을 때 반환하는 에러를 생성합니다.
func NewErrDirectoryAccessFailed(err error, dir string) error {
	return apperrors.Wrap(err, apperrors.Internal, fmt.Sprintf("저장소 초기화 실패: 디렉토리 접근 불가 (%s)", dir))
}

// NewErrJSONMarshalFailed 작업 결과 데이터를 JSON으로 직렬화하는 데 실패했을 때 반환하는 에러를 생성합니다.
func NewErrJSONMarshalFailed(err error) error {
	return apperrors.Wrap(err, apperrors.Internal, "데이터 처리 실패: 작업 결과 데이터 직렬화(JSON Marshal) 중 오류가 발생했습니다")
}

// NewErrJSONUnmarshalFailed 작업 결과 데이터를 JSON에서 역직렬화하는 데 실패했을 때 반환하는 에러를 생성합니다.
func NewErrJSONUnmarshalFailed(err error) error {
	return apperrors.Wrap(err, apperrors.Internal, "데이터 처리 실패: 작업 결과 데이터 역직렬화(JSON Unmarshal) 중 오류가 발생했습니다")
}

// NewErrTaskResultReadFailed 작업 결과 파일을 읽는 데 실패했을 때 반환하는 에러를 생성합니다.
func NewErrTaskResultReadFailed(err error) error {
	return apperrors.Wrap(err, apperrors.Internal, "작업 결과 조회 실패: 저장된 작업 결과 파일 읽기 처리 중 오류가 발생했습니다")
}

// NewErrDirectoryCreationFailed 작업 결과 저장 시 저장 디렉토리 생성에 실패했을 때 반환하는 에러를 생성합니다.
func NewErrDirectoryCreationFailed(err error) error {
	return apperrors.Wrap(err, apperrors.Internal, "작업 결과 저장 실패: 저장 디렉토리 생성 중 오류가 발생했습니다")
}

// NewErrTempFileCreationFailed 작업 결과 저장 시 임시 파일 생성에 실패했을 때 반환하는 에러를 생성합니다.
func NewErrTempFileCreationFailed(err error) error {
	return apperrors.Wrap(err, apperrors.Internal, "작업 결과 저장 실패: 임시 파일 생성 중 오류가 발생했습니다")
}

// NewErrFileWriteFailed 작업 결과 저장 시 파일 쓰기에 실패했을 때 반환하는 에러를 생성합니다.
func NewErrFileWriteFailed(err error) error {
	return apperrors.Wrap(err, apperrors.Internal, "작업 결과 저장 실패: 파일 쓰기 중 오류가 발생했습니다")
}

// NewErrFileSyncFailed 작업 결과 저장 시 디스크 동기화에 실패했을 때 반환하는 에러를 생성합니다.
func NewErrFileSyncFailed(err error) error {
	return apperrors.Wrap(err, apperrors.Internal, "작업 결과 저장 실패: 디스크 동기화 중 오류가 발생했습니다")
}

// NewErrFileCloseFailed 작업 결과 저장 시 파일 닫기에 실패했을 때 반환하는 에러를 생성합니다.
func NewErrFileCloseFailed(err error) error {
	return apperrors.Wrap(err, apperrors.Internal, "작업 결과 저장 실패: 파일 닫기 중 오류가 발생했습니다")
}

// NewErrFileRenameFailed 작업 결과 저장 시 파일 이름 변경에 실패했을 때 반환하는 에러를 생성합니다.
func NewErrFileRenameFailed(err error) error {
	return apperrors.Wrap(err, apperrors.Internal, "작업 결과 저장 실패: 파일 이름 변경 중 오류가 발생했습니다")
}
