package kurly

import (
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

var (
	// ErrWatchListFileEmpty watch_list_file 설정이 존재하지 않거나 공백일 때 반환합니다.
	ErrWatchListFileEmpty = apperrors.New(apperrors.InvalidInput, "watch_list_file이 설정되지 않았거나 공백입니다")

	// ErrWatchListFileNotCSV watch_list_file 경로가 .csv 확장자로 끝나지 않을 때 반환합니다.
	ErrWatchListFileNotCSV = apperrors.New(apperrors.InvalidInput, "watch_list_file은 .csv 파일 경로여야 합니다")

	// ErrCSVStreamReadFailed CSV 스트림의 첫 바이트를 읽는 중 오류가 발생했을 때 반환되는 에러입니다.
	ErrCSVStreamReadFailed = apperrors.New(apperrors.InvalidInput, "CSV 스트림의 시작 부분을 읽는 중 오류가 발생했습니다. 데이터가 비어있거나 올바르지 않은 인코딩일 수 있습니다")

	// ErrBOMRecovery BOM 검사 후 버퍼를 원래 상태로 복구하지 못했을 때 반환되는 에러입니다.
	ErrBOMRecovery = apperrors.New(apperrors.Internal, "BOM(Byte Order Mark) 검사 후 버퍼 상태를 복구하는 중 오류가 발생했습니다")

	// ErrCSVParse CSV 데이터 파싱 중 오류가 발생했을 때 반환되는 에러입니다.
	ErrCSVParse = apperrors.New(apperrors.InvalidInput, "CSV 데이터 파싱 중 오류가 발생했습니다. 파일 인코딩이나 형식을 확인해 주세요")

	// ErrCSVEmpty CSV 파일을 성공적으로 읽었지만 실제 행이 한 건도 없을 때 반환되는 에러입니다.
	ErrCSVEmpty = apperrors.New(apperrors.InvalidInput, "CSV 데이터가 비어있습니다. 파일 내용을 확인해 주세요")

	// ErrCSVInvalidHeader CSV 파일의 첫 번째 행(헤더)의 컬럼 수가 부족할 때 반환되는 에러입니다.
	ErrCSVInvalidHeader = apperrors.New(apperrors.InvalidInput, "CSV 헤더 형식이 올바르지 않습니다. 필수 컬럼(no, name, status)이 포함되어 있는지 확인해 주세요")

	// ErrCSVAllRecordsFiltered CSV 파일의 모든 데이터 행이 필터링되어 유효한 상품 레코드가 한 건도 남지 않았을 때 반환되는 에러입니다.
	ErrCSVAllRecordsFiltered = apperrors.New(apperrors.InvalidInput, "처리할 수 있는 유효한 상품 레코드가 없습니다. 모든 행이 필수 데이터(상품번호, 상품명) 누락으로 인해 필터링되었습니다")
)

// newErrWatchListFileNotFound 원본 에러를 래핑하여 감시 대상 상품 목록 파일이 존재하지 않을 때의 에러를 생성합니다.
//
// os.IsNotExist()가 true인 경우에만 호출합니다.
// 파일 경로가 설정 파일에 잘못 기재된 사용자 입력 오류이므로 InvalidInput으로 분류합니다.
//
// 매개변수:
//   - cause: os.Open()에서 반환된 원본 에러
//   - path: 존재하지 않는 파일의 절대 경로 또는 상대 경로
//
// 반환값: 파일 경로와 조치 안내 메시지를 포함한 InvalidInput 에러
func newErrWatchListFileNotFound(cause error, path string) error {
	return apperrors.Wrap(cause, apperrors.InvalidInput, "감시 대상 상품 목록 파일("+path+")이 존재하지 않습니다. 경로 설정을 확인해 주세요")
}

// newErrWatchListFileOpenFailed 원본 에러를 래핑하여 감시 대상 상품 목록 파일 열기 실패 에러를 생성합니다.
//
// 파일이 존재하지만 열 수 없는 경우(권한 부족, 파일 잠금 등)에 호출합니다.
// 사용자 입력 오류가 아닌 시스템 레벨의 예측 불가 오류이므로 Internal로 분류합니다.
//
// 매개변수:
//   - cause: os.Open()에서 반환된 원본 에러
//   - path: 열기에 실패한 파일의 절대 경로 또는 상대 경로
//
// 반환값: 파일 경로와 조치 안내 메시지를 포함한 Internal 에러
func newErrWatchListFileOpenFailed(cause error, path string) error {
	return apperrors.Wrap(cause, apperrors.Internal, "감시 대상 상품 목록 파일("+path+")을 여는 중 예기치 않은 오류가 발생했습니다. 파일 권한이나 잠금 상태를 확인해 주세요")
}
