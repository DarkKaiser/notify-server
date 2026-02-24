package kurly

import (
	"bufio"
	"encoding/csv"
	"io"
	"os"
	"strings"
)

// columnIndex 파싱된 레코드 내 각 필드 위치를 나타내는 인덱스 타입입니다.
// int가 아닌 별도 타입으로 정의하여 일반 정수와의 혼용을 컴파일 타임에 방지합니다.
type columnIndex int

const (
	// columnID 상품 코드 컬럼의 인덱스 (0번째)입니다.
	// 마켓컬리에서 부여한 숫자형 ID를 문자열 그대로 담습니다. (예: "12345")
	columnID columnIndex = iota

	// columnName 상품 이름 컬럼의 인덱스 (1번째)입니다.
	columnName

	// columnStatus 감시 활성화 여부 컬럼의 인덱스 (2번째)입니다.
	// 값이 statusEnabled("1")이면 활성, 그 외 모든 값은 비활성으로 처리합니다.
	columnStatus
)

// statusEnabled status 컬럼에서 '감시 활성화' 상태를 나타내는 문자열 값입니다.
const statusEnabled = "1"

// WatchListLoader 감시 대상 상품 목록을 로드하는 인터페이스입니다.
//
// 데이터 소스(CSV 파일, 데이터베이스, 원격 API 등)를 추상화하여 실행 로직이 데이터 출처에 의존하지 않도록 분리합니다.
// 테스트 시에는 이 인터페이스를 구현한 Mock을 주입하여 파일 I/O 없이 검증할 수 있습니다.
type WatchListLoader interface {
	// Load 데이터 소스로부터 감시 대상 상품 레코드 목록을 읽어 반환합니다.
	//
	// 반환되는 [][]string의 각 원소는 하나의 상품 레코드이며,
	// columnID, columnName, columnStatus 인덱스로 각 필드에 접근합니다.
	Load() ([][]string, error)
}

// csvWatchListLoader 로컬 파일 시스템의 CSV 파일을 데이터 소스로 사용하는 WatchListLoader 구현체입니다.
type csvWatchListLoader struct {
	// filePath 읽어올 CSV 파일의 절대 경로 또는 상대 경로입니다.
	filePath string
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ WatchListLoader = (*csvWatchListLoader)(nil)

// NewCSVWatchListLoader 주어진 filePath의 CSV 파일을 읽는 WatchListLoader를 생성합니다.
func NewCSVWatchListLoader(filePath string) WatchListLoader {
	return &csvWatchListLoader{
		filePath: filePath,
	}
}

// Load filePath에 지정된 CSV 파일을 열고, 파싱·정제하여 감시 대상 상품 레코드 목록을 반환합니다.
//
// 파일 열기 단계에서 발생하는 오류는 원인에 따라 두 가지로 분류합니다.
//   - 파일 미존재(os.IsNotExist): 경로 설정 오류로 사용자가 즉시 조치할 수 있는 입력 오류
//   - 그 외(권한 부족, 잠금 등): 예측하기 어려운 시스템 내부 오류
func (l *csvWatchListLoader) Load() ([][]string, error) {
	file, err := os.Open(l.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, newErrWatchListFileNotFound(err, l.filePath)
		}

		return nil, newErrWatchListFileOpenFailed(err, l.filePath)
	}
	defer file.Close()

	return parseWatchListRecords(file)
}

// parseWatchListRecords CSV 스트림을 파싱하고, 비즈니스 로직에서 안전하게 사용할 수 있도록
// 불완전한 데이터를 정제한 레코드 목록을 반환합니다.
//
// [처리 단계]
//  1. BOM 제거: Windows 메모장 등에서 저장 시 삽입되는 UTF-8 BOM(\uFEFF)을 감지하고 건너뜁니다.
//  2. CSV 파싱: 공백·따옴표·주석 등을 관대하게 처리하는 옵션으로 전체 레코드를 읽습니다.
//  3. 헤더 검증: 첫 행이 최소 3개 컬럼(no, name, status)을 갖추고 있는지 확인합니다.
//  4. 데이터 정제: 헤더를 제외한 본문 레코드만 순회하며 각 필드의 공백을 제거하고,
//     필수 컬럼(상품번호·상품명)이 없거나 비어있는 행은 조용히 걸러냅니다.
//
// 반환값:
//   - [][]string: 정제 완료된 유효 레코드 목록 (헤더 행 미포함)
//   - error: I/O 오류, 형식 오류, 또는 유효 레코드가 한 건도 없는 경우 반환
func parseWatchListRecords(r io.Reader) ([][]string, error) {
	// [단계 1] BOM 제거
	// Windows 메모장·Excel 등에서 UTF-8로 저장 시, 파일 앞에 보이지 않는 BOM(\uFEFF)이 붙습니다.
	// csv.Reader가 BOM을 헤더 컬럼명의 일부로 오인하지 않도록 사전에 제거합니다.
	bomReader := bufio.NewReader(r)
	bom, _, err := bomReader.ReadRune()
	if err != nil {
		return nil, ErrCSVStreamReadFailed
	}
	if bom != '\uFEFF' {
		// BOM이 아닌 일반 문자이면 버퍼에 되돌려 정상적으로 읽히도록 합니다.
		if err := bomReader.UnreadRune(); err != nil {
			return nil, ErrBOMRecovery
		}
	}

	// [단계 2] CSV 파싱 (관대한 옵션 적용)
	csvReader := csv.NewReader(bomReader)
	csvReader.TrimLeadingSpace = true // 쉼표 직후 공백 자동 제거 (예: "a, b" → "a", "b")
	csvReader.FieldsPerRecord = -1    // 행마다 컬럼 수가 달라도 허용 (status 컬럼 누락 방어)
	csvReader.LazyQuotes = true       // 따옴표가 불완전하게 사용된 필드도 너그럽게 파싱
	csvReader.Comment = '#'           // '#'으로 시작하는 행은 주석으로 간주하여 읽지 않음

	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, ErrCSVParse
	}

	if len(records) == 0 {
		return nil, ErrCSVEmpty
	}

	// [단계 3] 헤더 검증
	// 첫 번째 행은 헤더로 간주하고 실제 데이터에서 제외합니다.
	// 최소한 no, name, status 3개 컬럼이 있어야 본문 레코드를 올바르게 처리할 수 있습니다.
	if len(records[0]) < 3 {
		return nil, ErrCSVInvalidHeader
	}

	// [단계 4] 데이터 정제
	// 헤더를 제외한 본문 레코드 수만큼 미리 용량을 확보하여 재할당 비용을 방지합니다.
	sanitizedRecords := make([][]string, 0, max(len(records)-1, 0))

	for _, record := range records[1:] {
		// csv.Reader의 TrimLeadingSpace는 선행 공백만 제거합니다.
		// 후행 공백까지 확실히 제거하기 위해 모든 필드를 명시적으로 Trim 합니다.
		for i := range record {
			record[i] = strings.TrimSpace(record[i])
		}

		// 상품번호(ID)와 상품명(Name) 컬럼이 모두 존재하고 값이 있어야 유효한 레코드로 처리합니다.
		// 둘 중 하나라도 없거나 비어있으면 비즈니스 로직에서 처리할 수 없으므로 조용히 건너뜁니다.
		if len(record) <= int(columnName) {
			continue
		}
		if record[columnID] == "" || record[columnName] == "" {
			continue
		}

		sanitizedRecords = append(sanitizedRecords, record)
	}

	if len(sanitizedRecords) == 0 {
		return nil, ErrCSVAllRecordsFiltered
	}

	return sanitizedRecords, nil
}
