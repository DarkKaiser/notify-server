package kurly

import (
	"bufio"
	"encoding/csv"
	"io"
	"os"
	"strings"
)

// columnIndex 파싱된 레코드 내 각 필드 위치를 나타내는 인덱스 타입입니다.
type columnIndex int

const (
	// columnID 상품 번호 컬럼의 인덱스입니다. (0번째)
	// 마켓컬리에서 부여한 숫자형 ID를 문자열 그대로 담습니다. (예: "12345")
	columnID columnIndex = iota

	// columnName 상품 이름 컬럼의 인덱스입니다. (1번째)
	columnName

	// columnStatus 감시 활성화 여부 컬럼의 인덱스입니다. (2번째)
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

		// csv.Reader의 FieldsPerRecord = -1 옵션으로 인해, 행마다 컬럼 수가 다를 수 있습니다.
		// 예를 들어, 사용자가 status 컬럼을 쉼표 없이 생략하면 해당 레코드의 길이는 2가 됩니다.
		// 이 상태로 반환하면, 소비 단에서 record[columnStatus](인덱스 2)에 접근할 때
		// Index Out of Range 패닉이 발생합니다.
		// 따라서 모든 레코드의 길이가 최소 columnStatus + 1(인덱스 3)이 되도록
		// 누락된 자리를 빈 문자열("")로 패딩합니다. (빈 값 = statusEnabled가 아님 = 비활성 취급)
		for len(record) <= int(columnStatus) {
			record = append(record, "")
		}

		sanitizedRecords = append(sanitizedRecords, record)
	}

	if len(sanitizedRecords) == 0 {
		return nil, ErrCSVAllRecordsFiltered
	}

	return sanitizedRecords, nil
}

// separateDuplicateRecords 레코드 목록을 상품 번호(columnID) 기준으로 순회하면서,
// 최초로 등장한 레코드는 '고유 레코드 목록'으로, 이미 등장한 레코드는 '중복 레코드 목록'으로 분리합니다.
//
// 중복이 존재하더라도 오류로 처리하지 않고, 이 함수를 사용하는 곳에서 중복 여부를 별도로 판단(예: 경고 로그)할 수 있도록
// 두 슬라이스를 함께 반환하는 방식을 채택합니다.
//
// 파라미터:
//   - records: parseWatchListRecords가 정제한 레코드 목록. 각 레코드는 최소 columnID 인덱스를 포함해야 합니다.
//
// 반환값:
//   - distinctRecords: 상품 번호 기준으로 중복이 제거된 고유 레코드 목록
//     동일 상품 중 하나라도 활성 상태("1")인 레코드가 있다면 해당 고유 레코드의 상태도 활성 상태로 합산(승격)됩니다.
//   - duplicateRecords: 이미 distinctRecords에 포함된 상품 번호가 재등장한 중복 레코드 목록
func separateDuplicateRecords(records [][]string) ([][]string, [][]string) {
	distinctRecords := make([][]string, 0, len(records))
	duplicateRecords := make([][]string, 0, len(records)/2)

	// 중복 레코드 등장 시 기존 고유 레코드의 상태를 갱신할 수 있도록
	// distinctRecords 배열 내의 저장 위치(Index)를 추적하는 맵입니다.
	//  - Key: 상품 번호 (ID)
	//  - Value: distinctRecords 배열 내 해당 레코드의 인덱스
	distinctIdxByID := make(map[string]int, len(records))

	for _, record := range records {
		// 필수 컬럼(상품 번호 및 상태) 존재 여부 확인
		if len(record) <= int(columnStatus) {
			continue
		}

		productID := record[columnID]
		if distinctIdx, exists := distinctIdxByID[productID]; !exists {
			// 처음 발견된 상품: 향후 중복 레코드 출현 시 고유 레코드의 상태를 합산·갱신할 수 있도록,
			// 고유 레코드 목록에 추가될 위치(Index)를 맵에 미리 기록해 둡니다.
			distinctIdxByID[productID] = len(distinctRecords)

			// 고유 레코드 목록에 추가
			// 중복 레코드 처리 시 고유 레코드 요소의 상태(status)값을 변경하는 로직이 있습니다.
			// 원본 슬라이스(records)와의 메모리 공유로 인한 의도치 않은 데이터 오염을 막기 위해
			// 새로운 슬라이스를 할당하여 깊은 복사(Deep Copy)를 수행합니다.
			clonedRecord := make([]string, len(record))
			copy(clonedRecord, record)
			distinctRecords = append(distinctRecords, clonedRecord)
		} else {
			// 이미 발견된 상품: 중복 레코드 목록에 추가
			duplicateRecords = append(duplicateRecords, record)

			// 중복으로 등장한 레코드 중 하나라도 '감시 활성화(1)' 상태라면,
			// 대표로 수집을 수행할 고유 레코드의 상태도 '감시 활성화'로 강제 승격(합산)시킵니다.
			// 이를 통해 사용자가 여러 레코드 중 하나만 활성화해도 정상적으로 수집이 수행되도록 보장합니다.
			if record[columnStatus] == statusEnabled {
				distinctRecords[distinctIdx][columnStatus] = statusEnabled
			}
		}
	}

	return distinctRecords, duplicateRecords
}
