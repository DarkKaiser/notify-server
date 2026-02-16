package lotto

import (
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

var (
	// ErrAppPathMissing Task 설정 검증 시 로또 당첨번호 예측 프로그램의 경로(app_path)가 누락되었거나 공백일 때 반환됩니다.
	// 프로그램 실행을 위해 필수적인 설정값이므로 반드시 유효한 값이 설정되어야 합니다.
	ErrAppPathMissing = apperrors.New(apperrors.InvalidInput, "app_path는 필수 설정값입니다")

	// ErrPredictionCompleteMsgNotFound 외부 명령어 실행 결과(표준 출력)에서 '예측 작업 완료 메시지'를 찾을 수 없을 때 반환됩니다.
	ErrPredictionCompleteMsgNotFound = apperrors.New(apperrors.ExecutionFailed, "당첨번호 예측 작업의 종료 상태를 확인할 수 없습니다. 상세 원인은 로그 파일을 참고해 주십시오")

	// ErrResultFilePathNotFound 예측 작업 완료 메시지 내에서 '예측 결과 파일의 경로'를 정규식으로 추출하는 데 실패했을 때 반환됩니다.
	ErrResultFilePathNotFound = apperrors.New(apperrors.ExecutionFailed, "당첨번호 예측 결과 파일의 경로 정보를 추출하는 데 실패했습니다. 상세 원인은 로그 파일을 참고해 주십시오")

	// ErrAnalysisResultInvalid 예측 결과 파일 내에서 '분석 결과 섹션'을 찾을 수 없거나 데이터 형식이 올바르지 않을 때 반환됩니다.
	ErrAnalysisResultInvalid = apperrors.New(apperrors.InvalidInput, "당첨번호 예측 결과 파일의 형식이 유효하지 않거나 내용을 식별할 수 없습니다. 상세 원인은 로그 파일을 참고해 주십시오")
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

// newErrPredictionFailed 당첨번호 예측 프로세스 실행 실패 에러를 래핑합니다.
//
// 이 함수는 외부 예측 프로그램(Java)이 비정상 종료되었거나, 정상 종료되었음에도
// 결과 파일 경로를 출력하지 않았을 때 사용됩니다. 보안 및 비공개 정보 유출 방지를 위해
// 상세한 에러 내용(Stderr)은 사용자 메시지에 직접 포함하지 않으며, 운영자는 시스템 로그를
// 통해 상세 원인을 파악해야 합니다.
//
// 매개변수:
//   - cause: 프로세스 실행 또는 종료 대기(Wait) 중 발생한 원본 에러
//
// 반환값: apperrors.ExecutionFailed 타입으로 분류된 래핑된 에러
func newErrPredictionFailed(cause error) error {
	return apperrors.Wrap(cause, apperrors.ExecutionFailed, "당첨번호 예측 프로세스 실행 중 오류가 발생하였습니다. 상세 원인은 로그 파일을 참고해 주십시오")
}

// newErrResultFileAbsFailed 당첨번호 예측 결과 파일의 절대 경로 변환 실패 에러를 래핑합니다.
//
// 이 함수는 대상 파일의 경로를 절대 경로로 정규화하는 과정에서 실패했을 때 사용됩니다.
// 주로 현재 작업 디렉터리(CWD) 정보를 가져올 수 없는 등의 시스템 환경 문제로 인해 발생합니다.
//
// 매개변수:
//   - cause: filepath.Abs()에서 발생한 원본 에러
//
// 반환값: apperrors.System 타입으로 분류된 래핑된 에러
func newErrResultFileAbsFailed(cause error) error {
	return apperrors.Wrap(cause, apperrors.System, "예측 결과 파일의 절대 경로를 확인(Resolve)하는 도중 시스템 오류가 발생했습니다")
}

// newErrResultFileRelFailed 예측 결과 파일의 상대 경로 계산 실패 에러를 래핑합니다.
//
// 이 함수는 보안 점검(Path Traversal 방지)을 위해 앱 실행 경로(appPath)와
// 예측 결과 파일(resultFilePath) 간의 상대 경로를 계산하려 했으나,
// 두 경로가 서로 다른 드라이브에 있거나 관계를 맺을 수 없는 경우에 사용됩니다.
//
// 매개변수:
//   - cause: filepath.Rel()에서 발생한 원본 에러
//
// 반환값: apperrors.ExecutionFailed 타입으로 분류된 래핑된 에러
func newErrResultFileRelFailed(cause error) error {
	return apperrors.Wrap(cause, apperrors.ExecutionFailed, "예측 결과 파일의 경로 유효성을 검증하는 도중 오류가 발생했습니다")
}

// newErrPathTraversalDetected 보안 위협(Path Traversal) 감지 에러를 생성합니다.
//
// 이 함수는 예측 결과 파일의 경로가 앱 실행 경로(appPath)를 벗어나는 경우(예: 상위 디렉터리 '..' 참조 등)에 호출됩니다.
// 이는 악의적인 파일 접근 시도로 간주되므로 즉시 작업을 중단하고 보안 위반 에러를 반환합니다.
//
// 반환값: apperrors.Forbidden 타입으로 분류된 에러
func newErrPathTraversalDetected() error {
	return apperrors.New(apperrors.Forbidden, "보안 정책 위반: 허용된 경로 범위를 벗어난 파일 접근이 감지되었습니다 (Path Traversal 시도 의심)")
}

// newErrReadResultFileFailed 예측 결과 파일을 읽는 도중 발생한 파일 입출력(I/O) 에러를 래핑합니다.
//
// 이 함수는 검증이 완료된 예측 결과 파일을 실제로 읽어들이는 os.ReadFile() 단계에서
// 파일이 삭제되었거나(Race Condition), 읽기 권한이 없는 등의 문제로 실패했을 때 사용됩니다.
//
// 매개변수:
//   - cause: os.ReadFile()에서 발생한 원본 에러
//   - filename: 읽기에 실패한 파일의 이름(또는 경로)
//
// 반환값: apperrors.System 타입으로 분류된 래핑된 에러
func newErrReadResultFileFailed(cause error, filename string) error {
	return apperrors.Wrapf(cause, apperrors.System, "예측 결과 파일(%s)을 읽는 도중 I/O 오류가 발생했습니다", filename)
}

// newErrParseResultFileFailed 예측 결과 파일의 데이터 파싱 실패 에러를 래핑합니다.
//
// 이 함수는 파일 읽기에는 성공했으나, 그 내용이 기대하는 포맷(예: 특정 헤더나 구조)과
// 일치하지 않아 분석할 수 없을 때 사용됩니다. 주로 예측 프로그램의 버전 불일치나
// 출력 형식 변경 등으로 인해 발생할 수 있습니다.
//
// 매개변수:
//   - cause: 파싱 중 발생한 원본 에러 (예: "분석 결과 섹션을 찾을 수 없음")
//   - filename: 파싱에 실패한 파일의 이름(또는 경로)
//
// 반환값: apperrors.ParsingFailed 타입으로 분류된 래핑된 에러
func newErrParseResultFileFailed(cause error, filename string) error {
	return apperrors.Wrapf(cause, apperrors.ParsingFailed, "예측 결과 파일(%s)의 내용을 파싱하는 도중 오류가 발생했습니다", filename)
}
