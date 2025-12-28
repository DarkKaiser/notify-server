package kurly

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

// csvColumnIndex CSV 파일에서 상품 정보를 파싱할 때 사용되는 컬럼 인덱스를 정의하는 타입입니다.
type csvColumnIndex int

const (
	// CSV 파일의 헤더 순서에 따른 컬럼 인덱스 상수입니다.
	//
	// [주의]
	// 이 상수의 순서는 실제 CSV 파일의 헤더 순서와 **엄격하게 일치**해야 합니다.
	// 파일 포맷이 변경될 경우, 이 상수의 정의도 반드시 함께 수정되어야 합니다.
	csvColumnID     csvColumnIndex = iota // [0] 상품 코드
	csvColumnName                         // [1] 상품 이름
	csvColumnStatus                       // [2] 감시 활성화 여부

	// CSV 파일의 '감시 활성화 여부' 컬럼에 사용되는 상태값 상수입니다.
	//
	// [설명]
	// CSV 파일에서 읽어온 데이터는 문자열(string) 타입이므로, 비교의 정확성을 위해
	// 정수형(1) 대신 문자열 상수("1")를 정의하여 사용합니다. ('1'이 아닌 모든 값은 비활성 상태로 간주합니다)
	csvStatusEnabled = "1" // 감시 활성화
)

// WatchListLoader 감시 대상 상품 목록을 외부 데이터 소스로부터 로드(적재)하는 추상 인터페이스입니다.
type WatchListLoader interface {
	Load() ([][]string, error)
}

// CSVWatchListLoader 로컬 파일 시스템에 저장된 CSV 파일로부터 감시 대상 상품 목록을 로드하는 구현체입니다.
type CSVWatchListLoader struct {
	FilePath string
}

// Load 지정된 경로의 CSV 파일을 열고, 파싱하여 감시 대상 상품 목록을 로드합니다.
func (l *CSVWatchListLoader) Load() ([][]string, error) {
	file, err := os.Open(l.FilePath)
	if err != nil {
		// 오류 발생 시 파일 경로와 상황을 명확히게 전달하여 사용자가 즉시 조치할 수 있도록 돕습니다.
		if os.IsNotExist(err) {
			return nil, apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("감시 대상 상품 목록 파일(%s)이 존재하지 않습니다. 경로 설정을 확인해 주세요", l.FilePath))
		}
		return nil, apperrors.Wrap(err, apperrors.Internal, fmt.Sprintf("감시 대상 상품 목록 파일(%s)을 여는 중 예기치 않은 오류가 발생했습니다. 파일 권한이나 잠금 상태를 확인해 주세요", l.FilePath))
	}
	defer file.Close()

	return readWatchListRecords(file)
}

// readWatchListRecords 원본 CSV 스트림을 읽어 파싱하고, 비즈니스 로직에 적합한 형태로 정제(Sanitize)하여 반환합니다.
//
// [매개변수]
//   - r: CSV 데이터를 스트리밍할 io.Reader 인터페이스
//
// [반환값]
//   - [][]string: 정제된 유효 감시 대상 상품 레코드 목록 (헤더 제외)
//   - error: I/O 오류 또는 치명적인 포맷 오류 시 반환
func readWatchListRecords(r io.Reader) ([][]string, error) {
	// Windows 메모장 등으로 저장 시 발생하는 UTF-8 BOM 제거
	buf := bufio.NewReader(r)
	runeChar, _, err := buf.ReadRune()
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.InvalidInput, "CSV 스트림의 시작 부분을 읽는 중 오류가 발생했습니다. 데이터가 비어있거나 올바르지 않은 인코딩일 수 있습니다")
	}
	if runeChar != '\uFEFF' {
		// BOM이 아니면 다시 되돌린다 (Unread)
		if err := buf.UnreadRune(); err != nil {
			return nil, apperrors.Wrap(err, apperrors.Internal, "BOM(Byte Order Mark) 검사 후 버퍼 상태를 복구(Unread)하는 중 치명적인 오류가 발생했습니다")
		}
	}

	csvReader := csv.NewReader(buf)
	csvReader.TrimLeadingSpace = true // 쉼표 뒤 공백 자동 제거
	csvReader.FieldsPerRecord = -1    // 행마다 컬럼 개수가 달라도 에러 없이 읽음 (유연성)
	csvReader.LazyQuotes = true       // 따옴표 규칙 완화 (손상된 CSV 처리)
	csvReader.Comment = '#'           // '#'으로 시작하는 행은 주석으로 처리하여 무시 (설정 파일 주석 지원)

	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.InvalidInput, "CSV 데이터 파싱 중 치명적인 오류가 발생했습니다. 파일 인코딩이나 형식을 확인해 주세요")
	}

	if len(records) == 0 {
		return nil, apperrors.New(apperrors.InvalidInput, "CSV 데이터가 비어있습니다. 파일 내용을 확인해 주세요")
	}

	header := records[0]
	if len(header) < 3 { // 최소 3개 컬럼(no, name, status) 필요
		return nil, apperrors.New(apperrors.InvalidInput, "CSV 헤더 형식이 올바르지 않습니다. 필수 컬럼(no, name, status)이 포함되어 있는지 확인해 주세요")
	}

	// 원본 레코드 수만큼 미리 용량을 확보하여 append 시 재할당을 방지합니다.
	expectedSize := len(records) - 1
	if expectedSize < 0 {
		expectedSize = 0
	}
	sanitizedRecords := make([][]string, 0, expectedSize)

	// 파싱 단계에서 불완전한 데이터(필수 컬럼 누락)를 미리 필터링하여 데이터 정합성 확보
	for _, record := range records[1:] {
		// CSV 파서의 TrimLeadingSpace로는 후행 공백이 제거되지 않으므로, 명시적으로 모든 필드를 Trim 합니다.
		for i := range record {
			record[i] = strings.TrimSpace(record[i])
		}

		// 최소한 ID와 Name 컬럼이 존재해야 유효한 데이터로 취급한다.
		if len(record) <= int(csvColumnName) {
			continue
		}
		// ID나 Name이 공백인 경우도 무시한다.
		if record[csvColumnID] == "" || record[csvColumnName] == "" {
			continue
		}
		sanitizedRecords = append(sanitizedRecords, record)
	}

	if len(sanitizedRecords) == 0 {
		return nil, apperrors.New(apperrors.InvalidInput, "처리할 수 있는 유효한 상품 레코드가 없습니다. 모든 행이 필수 데이터(상품번호, 상품명) 누락으로 인해 필터링되었습니다")
	}

	return sanitizedRecords, nil
}
