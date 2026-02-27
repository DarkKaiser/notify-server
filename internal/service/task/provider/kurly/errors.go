package kurly

import (
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

// =====================================================================
// 1. 감시 대상 상품 목록 (WatchList) 로딩 관련 에러
// =====================================================================

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
	return apperrors.Wrapf(cause, apperrors.InvalidInput, "감시 대상 상품 목록 파일(%s)이 존재하지 않습니다. 경로 설정을 확인해 주세요", path)
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
	return apperrors.Wrapf(cause, apperrors.Internal, "감시 대상 상품 목록 파일(%s)을 여는 중 예기치 않은 오류가 발생했습니다. 파일 권한이나 잠금 상태를 확인해 주세요", path)
}

// =====================================================================
// 2. 상품 상세 페이지 & 기본 구조 파싱 관련 에러
// =====================================================================

// newErrNextDataNotFound HTML에서 __NEXT_DATA__ JSON 스크립트 태그를 찾지 못했을 때 에러를 생성합니다.
//
// 정규식(reExtractNextData)으로 HTML을 탐색했으나 매칭 결과가 없을 때 호출됩니다.
// 이는 마켓컬리가 Next.js를 통한 SSR 데이터 주입 방식을 변경했을 가능성이 높습니다.
//
// 매개변수:
//   - url: 처리 중이던 상품 상세 페이지 URL (에러 메시지에 포함하여 원인 추적을 돕습니다)
//
// 반환값: ExecutionFailed 에러
func newErrNextDataNotFound(url string) error {
	return apperrors.Newf(apperrors.ExecutionFailed, "불러온 페이지(%s)에서 __NEXT_DATA__ JSON 태그를 찾을 수 없습니다. 페이지 구조가 변경되었을 가능성이 높습니다", url)
}

// newErrNextDataStructureInvalid 추출된 __NEXT_DATA__ JSON 내에 필수 최상위 노드(props.pageProps)가 존재하지 않을 때 에러를 생성합니다.
//
// 매개변수:
//   - url: 파싱을 시도한 상품 상세 페이지 URL (에러 메시지에 포함하여 원인 추적을 돕습니다)
//
// 반환값: ExecutionFailed 에러
func newErrNextDataStructureInvalid(url string) error {
	return apperrors.Newf(apperrors.ExecutionFailed, "추출된 __NEXT_DATA__ JSON 내에 기대하는 데이터 구조(props.pageProps)가 존재하지 않습니다. 스키마가 변경되었을 가능성이 높습니다(%s)", url)
}

// newErrProductSectionExtractionFailed 상품 정보 최상위 섹션 요소를 찾지 못했을 때 에러를 생성합니다.
//
// CSS 셀렉터(#product-atf > section.css-1ua1wyk)의 탐색 결과가 정확히 1개가 아닐 때 호출됩니다.
// 결과가 0개이거나 2개 이상인 경우 모두 이 에러로 처리합니다.
//
// 매개변수:
//   - url: 처리 중이던 상품 상세 페이지 URL (에러 메시지에 포함하여 원인 추적을 돕습니다)
//
// 반환값: ExecutionFailed 에러
func newErrProductSectionExtractionFailed(url string) error {
	return apperrors.Newf(apperrors.ExecutionFailed, "상품 정보 섹션(#product-atf > section.css-1ua1wyk)을 찾을 수 없습니다. 페이지 레이아웃이 변경되었을 가능성이 높습니다(%s)", url)
}

// newErrProductNameExtractionFailed 상품 이름 요소(h1)를 찾지 못했을 때 에러를 생성합니다.
//
// CSS 셀렉터(div.css-84rb3h > div.css-6zfm8o > div.css-o3fjh7 > h1)의 탐색 결과가 정확히 1개가 아닐 때 호출됩니다.
//
// 매개변수:
//   - url: 처리 중이던 상품 상세 페이지 URL (에러 메시지에 포함하여 원인 추적을 돕습니다)
//
// 반환값: ExecutionFailed 에러
func newErrProductNameExtractionFailed(url string) error {
	return apperrors.Newf(apperrors.ExecutionFailed, "상품 이름 요소(div.css-84rb3h > div.css-6zfm8o > div.css-o3fjh7 > h1)를 찾을 수 없습니다. 페이지 레이아웃이 변경되었을 가능성이 높습니다(%s)", url)
}

// =====================================================================
// 3. 상품 가격 상세 정보 파싱 및 변환 관련 에러
// =====================================================================

// newErrPriceExtractionFailed 상품 가격 요소를 찾지 못했을 때 에러를 생성합니다.
//
// 매개변수:
//   - url: 처리 중이던 상품 상세 페이지 URL (에러 메시지에 포함하여 원인 추적을 돕습니다)
//   - selector: 탐색에 실패한 CSS 셀렉터 문자열
//
// 반환값: ExecutionFailed 에러
func newErrPriceExtractionFailed(url, selector string) error {
	return apperrors.Newf(apperrors.ExecutionFailed, "상품 가격 요소(%s)를 찾을 수 없습니다. 페이지 레이아웃이 변경되었을 가능성이 높습니다(%s)", selector, url)
}

// newErrPriceConversionFailed 정가(할인 미적용 시 표시되는 가격) 텍스트를 정수(int)로 변환 실패 시 에러를 생성합니다.
//
// 매개변수:
//   - cause: strconv.Atoi에서 반환된 원본 에러
//   - text: 변환에 실패한 원본 가격 텍스트 (에러 메시지에 포함하여 원인 추적을 돕습니다)
//
// 반환값: 원인 에러를 래핑한 ExecutionFailed 에러
func newErrPriceConversionFailed(cause error, text string) error {
	return apperrors.Wrapf(cause, apperrors.ExecutionFailed, "정가 텍스트(%s)를 정수로 변환하는 중 실패하였습니다", text)
}

// newErrDiscountRateConversionFailed 할인율 텍스트를 정수(int)로 변환 실패 시 에러를 생성합니다.
//
// 매개변수:
//   - cause: strconv.Atoi에서 반환된 원본 에러
//   - text: 변환에 실패한 원본 할인율 텍스트 (에러 메시지에 포함하여 원인 추적을 돕습니다)
//
// 반환값: 원인 에러를 래핑한 ExecutionFailed 에러
func newErrDiscountRateConversionFailed(cause error, text string) error {
	return apperrors.Wrapf(cause, apperrors.ExecutionFailed, "할인율 텍스트(%s)를 정수로 변환하는 중 실패하였습니다", text)
}

// newErrDiscountedPriceConversionFailed 할인가(실구매가) 텍스트를 정수(int)로 변환 실패 시 에러를 생성합니다.
//
// 매개변수:
//   - cause: strconv.Atoi에서 반환된 원본 에러
//   - text: 변환에 실패한 원본 할인가 텍스트 (에러 메시지에 포함하여 원인 추적을 돕습니다)
//
// 반환값: 원인 에러를 래핑한 ExecutionFailed 에러
func newErrDiscountedPriceConversionFailed(cause error, text string) error {
	return apperrors.Wrapf(cause, apperrors.ExecutionFailed, "할인가 텍스트(%s)를 정수로 변환하는 중 실패하였습니다", text)
}

// newErrPriceStructureInvalid 예상치 못한 가격 DOM 구조(할인율 요소 2개 이상 등) 감지 시 에러를 생성합니다.
//
// 할인율 요소(span.css-8h3us8)가 정상 범위(0개 또는 1개)를 초과하여 2개 이상 감지될 때 호출됩니다.
// 이는 마켓컬리의 페이지 레이아웃이 전혀 예상하지 못한 방식으로 변경되었음을 의미합니다.
//
// 매개변수:
//   - url: 처리 중이던 상품 상세 페이지 URL (에러 메시지에 포함하여 원인 추적을 돕습니다)
//
// 반환값: ExecutionFailed 에러
func newErrPriceStructureInvalid(url string) error {
	return apperrors.Newf(apperrors.ExecutionFailed, "할인율 요소(span.css-8h3us8)가 2개 이상 감지되었습니다. 페이지 레이아웃이 변경되었을 가능성이 높습니다(%s)", url)
}
