package kurly

import (
	"errors"
	"testing"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// 검증 헬퍼
// =============================================================================

// assertAppError AppError 타입 단언 및 타입/메시지를 검증하는 헬퍼입니다.
func assertAppError(t *testing.T, err error, wantType apperrors.ErrorType, wantMsgSubstr string) {
	t.Helper()
	require.Error(t, err)
	assert.True(t, apperrors.Is(err, wantType),
		"에러 타입이 일치해야 합니다. got=%v, want=%v", err, wantType)
	assert.Contains(t, err.Error(), wantMsgSubstr,
		"에러 메시지가 기대한 문자열을 포함해야 합니다")
}

// assertWrappedAppError Wrap된 에러의 타입 및 원인 에러(cause) 보존 여부를 검증하는 헬퍼입니다.
func assertWrappedAppError(t *testing.T, err error, wantType apperrors.ErrorType, wantMsgSubstr string, cause error) {
	t.Helper()
	assertAppError(t, err, wantType, wantMsgSubstr)
	// errors.Is를 사용하여 원인 에러가 체인 안에 보존되었는지 검증합니다.
	assert.True(t, errors.Is(err, cause),
		"원인 에러(cause)가 에러 체인에 보존되어야 합니다")
}

// =============================================================================
// 1. Sentinel 에러 변수 검증
// =============================================================================

// TestSentinelErrors 패키지 레벨에 선언된 sentinel 에러 변수들의
// ErrorType 및 메시지 내용을 검증합니다.
func TestSentinelErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		err           error
		wantType      apperrors.ErrorType
		wantMsgSubstr string
	}{
		{
			name:          "ErrWatchListFileEmpty — InvalidInput, 공백 메시지",
			err:           ErrWatchListFileEmpty,
			wantType:      apperrors.InvalidInput,
			wantMsgSubstr: "watch_list_file이 설정되지 않았거나 공백입니다",
		},
		{
			name:          "ErrWatchListFileNotCSV — InvalidInput, CSV 확장자 메시지",
			err:           ErrWatchListFileNotCSV,
			wantType:      apperrors.InvalidInput,
			wantMsgSubstr: "watch_list_file은 .csv 파일 경로여야 합니다",
		},
		{
			name:          "ErrCSVStreamReadFailed — InvalidInput, 스트림 읽기 실패 메시지",
			err:           ErrCSVStreamReadFailed,
			wantType:      apperrors.InvalidInput,
			wantMsgSubstr: "CSV 스트림의 시작 부분을 읽는 중 오류가 발생했습니다",
		},
		{
			name:          "ErrBOMRecovery — Internal, BOM 복구 실패 메시지",
			err:           ErrBOMRecovery,
			wantType:      apperrors.Internal,
			wantMsgSubstr: "BOM(Byte Order Mark) 검사 후 버퍼 상태를 복구하는 중 오류가 발생했습니다",
		},
		{
			name:          "ErrCSVParse — InvalidInput, CSV 파싱 에러 메시지",
			err:           ErrCSVParse,
			wantType:      apperrors.InvalidInput,
			wantMsgSubstr: "CSV 데이터 파싱 중 오류가 발생했습니다",
		},
		{
			name:          "ErrCSVEmpty — InvalidInput, 빈 CSV 메시지",
			err:           ErrCSVEmpty,
			wantType:      apperrors.InvalidInput,
			wantMsgSubstr: "CSV 데이터가 비어있습니다",
		},
		{
			name:          "ErrCSVInvalidHeader — InvalidInput, 헤더 형식 오류 메시지",
			err:           ErrCSVInvalidHeader,
			wantType:      apperrors.InvalidInput,
			wantMsgSubstr: "CSV 헤더 형식이 올바르지 않습니다",
		},
		{
			name:          "ErrCSVAllRecordsFiltered — InvalidInput, 유효 레코드 없음 메시지",
			err:           ErrCSVAllRecordsFiltered,
			wantType:      apperrors.InvalidInput,
			wantMsgSubstr: "처리할 수 있는 유효한 상품 레코드가 없습니다",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertAppError(t, tt.err, tt.wantType, tt.wantMsgSubstr)
		})
	}
}

// TestSentinelErrors_Identity 동일한 sentinel 에러 변수는 항상 같은 인스턴스임을 검증합니다.
// (errors.Is로 비교 가능한 sentinel 패턴 보장)
func TestSentinelErrors_Identity(t *testing.T) {
	t.Parallel()

	t.Run("ErrWatchListFileEmpty는 자기 자신과 동일", func(t *testing.T) {
		t.Parallel()
		// errros.Is가 아닌 표준 errors.Is로 sentinel 동일성 확인
		assert.True(t, errors.Is(ErrWatchListFileEmpty, ErrWatchListFileEmpty))
	})
}

// =============================================================================
// 2. 에러 생성 함수 검증 — WatchList 로딩 관련
// =============================================================================

// TestNewErrWatchListFileNotFound OS 에러를 래핑하여 파일 경로를 메시지에 포함시키는지 검증합니다.
func TestNewErrWatchListFileNotFound(t *testing.T) {
	t.Parallel()

	const path = "/data/watch_list.csv"
	cause := errors.New("no such file or directory")

	err := newErrWatchListFileNotFound(cause, path)

	assertWrappedAppError(t, err, apperrors.InvalidInput, "/data/watch_list.csv", cause)
	assert.Contains(t, err.Error(), "존재하지 않습니다")
}

// TestNewErrWatchListFileOpenFailed 파일 열기 실패 에러가 Internal 타입으로 분류되는지 검증합니다.
func TestNewErrWatchListFileOpenFailed(t *testing.T) {
	t.Parallel()

	const path = "/data/watch_list.csv"
	cause := errors.New("permission denied")

	err := newErrWatchListFileOpenFailed(cause, path)

	assertWrappedAppError(t, err, apperrors.Internal, "/data/watch_list.csv", cause)
	assert.Contains(t, err.Error(), "예기치 않은 오류")
}

// =============================================================================
// 3. 에러 생성 함수 검증 — 상품 상세 페이지 파싱 관련
// =============================================================================

// TestNewErrNextDataNotFound __NEXT_DATA__ 태그 미발견 에러를 검증합니다.
func TestNewErrNextDataNotFound(t *testing.T) {
	t.Parallel()

	const url = "https://www.kurly.com/goods/12345"

	err := newErrNextDataNotFound(url)

	assertAppError(t, err, apperrors.ExecutionFailed, "__NEXT_DATA__ JSON 태그를 찾을 수 없습니다")
	assert.Contains(t, err.Error(), url, "에러 메시지에 URL이 포함되어야 합니다")
}

// TestNewErrNextDataStructureInvalid JSON 스키마 변경 감지 에러를 검증합니다.
func TestNewErrNextDataStructureInvalid(t *testing.T) {
	t.Parallel()

	const url = "https://www.kurly.com/goods/12345"

	err := newErrNextDataStructureInvalid(url)

	assertAppError(t, err, apperrors.ExecutionFailed, "props.pageProps")
	assert.Contains(t, err.Error(), url)
}

// TestNewErrProductSectionExtractionFailed 상품 정보 섹션 미발견 에러를 검증합니다.
func TestNewErrProductSectionExtractionFailed(t *testing.T) {
	t.Parallel()

	const url = "https://www.kurly.com/goods/12345"

	err := newErrProductSectionExtractionFailed(url)

	assertAppError(t, err, apperrors.ExecutionFailed, "#product-atf")
	assert.Contains(t, err.Error(), url)
}

// TestNewErrProductNameExtractionFailed 상품명 요소 미발견 에러를 검증합니다.
func TestNewErrProductNameExtractionFailed(t *testing.T) {
	t.Parallel()

	const url = "https://www.kurly.com/goods/12345"

	err := newErrProductNameExtractionFailed(url)

	assertAppError(t, err, apperrors.ExecutionFailed, "상품 이름 요소")
	assert.Contains(t, err.Error(), url)
}

// =============================================================================
// 4. 에러 생성 함수 검증 — 가격 파싱 및 변환 관련
// =============================================================================

// TestNewErrPriceExtractionFailed 가격 요소 미발견 에러에 셀렉터와 URL이 모두 포함되는지 검증합니다.
func TestNewErrPriceExtractionFailed(t *testing.T) {
	t.Parallel()

	const (
		url      = "https://www.kurly.com/goods/12345"
		selector = "h2.css-xrp7wx > div.css-o2nlqt > span"
	)

	err := newErrPriceExtractionFailed(url, selector)

	assertAppError(t, err, apperrors.ExecutionFailed, "상품 가격 요소")
	assert.Contains(t, err.Error(), selector, "에러 메시지에 CSS 셀렉터가 포함되어야 합니다")
	assert.Contains(t, err.Error(), url)
}

// TestNewErrPriceConversionFailed 정가 텍스트 변환 실패 에러가 원인 에러를 보존하는지 검증합니다.
func TestNewErrPriceConversionFailed(t *testing.T) {
	t.Parallel()

	const text = "N/A"
	cause := errors.New(`strconv.Atoi: parsing "N/A": invalid syntax`)

	err := newErrPriceConversionFailed(cause, text)

	assertWrappedAppError(t, err, apperrors.ExecutionFailed, "정가 텍스트", cause)
	assert.Contains(t, err.Error(), text, "에러 메시지에 원본 텍스트가 포함되어야 합니다")
}

// TestNewErrDiscountRateConversionFailed 할인율 텍스트 변환 실패 에러를 검증합니다.
func TestNewErrDiscountRateConversionFailed(t *testing.T) {
	t.Parallel()

	const text = "abc%"
	cause := errors.New(`strconv.Atoi: parsing "abc": invalid syntax`)

	err := newErrDiscountRateConversionFailed(cause, text)

	assertWrappedAppError(t, err, apperrors.ExecutionFailed, "할인율 텍스트", cause)
	assert.Contains(t, err.Error(), text)
}

// TestNewErrDiscountedPriceConversionFailed 할인가 텍스트 변환 실패 에러를 검증합니다.
func TestNewErrDiscountedPriceConversionFailed(t *testing.T) {
	t.Parallel()

	const text = "INVALID"
	cause := errors.New(`strconv.Atoi: parsing "INVALID": invalid syntax`)

	err := newErrDiscountedPriceConversionFailed(cause, text)

	assertWrappedAppError(t, err, apperrors.ExecutionFailed, "할인가 텍스트", cause)
	assert.Contains(t, err.Error(), text)
}

// TestNewErrPriceStructureInvalid 가격 DOM 구조 이상 에러를 검증합니다.
func TestNewErrPriceStructureInvalid(t *testing.T) {
	t.Parallel()

	const url = "https://www.kurly.com/goods/12345"

	err := newErrPriceStructureInvalid(url)

	assertAppError(t, err, apperrors.ExecutionFailed, "span.css-8h3us8")
	assert.Contains(t, err.Error(), "2개 이상")
	assert.Contains(t, err.Error(), url)
}

// =============================================================================
// 5. 에러 생성 함수 — 경계값 검증
// =============================================================================

// TestErrorFunctions_EmptyArgs 빈 문자열 인자를 전달해도 패닉 없이 에러를 반환하는지 검증합니다.
func TestErrorFunctions_EmptyArgs(t *testing.T) {
	t.Parallel()

	t.Run("빈 URL — newErrNextDataNotFound", func(t *testing.T) {
		t.Parallel()
		err := newErrNextDataNotFound("")
		require.Error(t, err)
		assert.True(t, apperrors.Is(err, apperrors.ExecutionFailed))
	})

	t.Run("빈 URL — newErrNextDataStructureInvalid", func(t *testing.T) {
		t.Parallel()
		err := newErrNextDataStructureInvalid("")
		require.Error(t, err)
	})

	t.Run("빈 path — newErrWatchListFileNotFound", func(t *testing.T) {
		t.Parallel()
		cause := errors.New("no such file")
		err := newErrWatchListFileNotFound(cause, "")
		require.Error(t, err)
		assert.True(t, errors.Is(err, cause))
	})

	t.Run("빈 text — newErrPriceConversionFailed", func(t *testing.T) {
		t.Parallel()
		cause := errors.New("atoi error")
		err := newErrPriceConversionFailed(cause, "")
		require.Error(t, err)
	})
}
