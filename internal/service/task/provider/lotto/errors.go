package lotto

import (
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

var (
	// ErrAppPathMissing Task 설정 검증 시 로또 당첨번호 예측 프로그램의 경로(app_path)가 누락되었거나 공백일 때 반환됩니다.
	// 프로그램 실행을 위해 필수적인 설정값이므로 반드시 유효한 값이 설정되어야 합니다.
	ErrAppPathMissing = apperrors.New(apperrors.InvalidInput, "app_path는 필수 설정값입니다")
)

// newErrAppPathAbsFailed 로또 당첨번호 예측 프로그램의 경로(app_path)를 절대 경로로 변환하는 과정에서 발생한 에러를 래핑합니다.
//
// 매개변수:
//   - cause: filepath.Abs()에서 발생한 원본 에러
//
// 반환값: apperrors.System 타입으로 분류된 래핑된 에러
func newErrAppPathAbsFailed(cause error) error {
	return apperrors.Wrap(cause, apperrors.System, "app_path의 절대 경로 변환에 실패하였습니다")
}

// newErrAppPathDirValidationFailed 로또 당첨번호 예측 프로그램의 경로(app_path)에 대한 디렉터리 검증 실패 에러를 래핑합니다.
//
// 이 함수는 validation.ValidateDir() 호출이 실패했을 때 사용되며, 다음과 같은 경우에 발생합니다:
//   - 지정된 경로가 존재하지 않는 경우
//   - 지정된 경로가 디렉터리가 아닌 파일인 경우
//   - 디렉터리에 대한 읽기 권한이 없는 경우
//   - 디렉터리 정보 확인 중 시스템 오류가 발생한 경우
//
// 매개변수:
//   - cause: validation.ValidateDir()에서 발생한 원본 에러
//
// 반환값: apperrors.InvalidInput 타입으로 분류된 래핑된 에러
func newErrAppPathDirValidationFailed(cause error) error {
	return apperrors.Wrap(cause, apperrors.InvalidInput, "app_path로 지정된 디렉터리 검증에 실패하였습니다")
}

// newErrJarFileNotFound 로또 당첨번호 예측 프로그램의 JAR 파일 검증 실패 에러를 래핑합니다.
//
// 이 함수는 validation.ValidateFile() 호출이 실패했을 때 사용되며, 다음과 같은 경우에 발생합니다:
//   - JAR 파일이 존재하지 않는 경우
//   - 해당 경로가 일반 파일이 아닌 경우 (디렉터리, 소켓, 파이프 등)
//   - JAR 파일에 대한 읽기 권한이 없는 경우
//   - 파일 정보 확인 중 시스템 오류가 발생한 경우
//
// 매개변수:
//   - cause: validation.ValidateFile()에서 발생한 원본 에러
//   - predictionJarName: 검증에 실패한 JAR 파일명 (에러 메시지에 포함됨)
//
// 반환값: apperrors.InvalidInput 타입으로 분류된 래핑된 에러
func newErrJarFileNotFound(cause error, predictionJarName string) error {
	return apperrors.Wrapf(cause, apperrors.InvalidInput, "로또 당첨번호 예측 프로그램(%s)을 찾을 수 없습니다", predictionJarName)
}

// newErrJavaNotFound Java 런타임(JRE) 환경 감지 실패 에러를 래핑합니다.
//
// 이 함수는 exec.LookPath("java") 호출이 실패했을 때 사용되며, 다음과 같은 경우에 발생합니다:
//   - 시스템에 Java(JRE 또는 JDK)가 설치되지 않은 경우
//   - Java가 설치되어 있지만 PATH 환경변수에 등록되지 않은 경우
//   - PATH 환경변수 조회 중 시스템 오류가 발생한 경우
//
// JAR 파일 실행을 위한 필수 시스템 요구사항이 충족되지 않았음을 나타냅니다.
//
// 매개변수:
//   - cause: exec.LookPath()에서 발생한 원본 에러
//
// 반환값: apperrors.System 타입으로 분류된 래핑된 에러
func newErrJavaNotFound(cause error) error {
	return apperrors.Wrap(cause, apperrors.System, "Java 런타임(JRE) 환경을 찾을 수 없습니다")
}
