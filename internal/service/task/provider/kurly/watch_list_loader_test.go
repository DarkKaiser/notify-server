package kurly

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// parseWatchListRecords 테스트
// =============================================================================

// TestParseWatchListRecords_TableDriven
// CSV 스트림을 받아 정제된 레코드로 변환하는 parseWatchListRecords 함수를
// 전방위적으로 검증합니다.
func TestParseWatchListRecords_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantRecords [][]string
		wantErr     bool
		errTarget   error // errors.Is 비교용 (sentinel error)
	}{
		// ── 정상 케이스 ──────────────────────────────────────────────────────
		{
			name: "성공: 정상적인 CSV 데이터 (헤더 + 데이터) — 반환값 전체 비교",
			input: "No,Name,Status\n" +
				"1001,사과,1\n" +
				"1002,바나나,0",
			wantRecords: [][]string{
				{"1001", "사과", "1"},
				{"1002", "바나나", "0"},
			},
			wantErr: false,
		},
		{
			name: "성공: BOM(\\uFEFF)이 포함된 데이터 — BOM 제거 후 정상 파싱",
			// Windows 메모장이 UTF-8로 저장 시 BOM을 삽입합니다.
			input: "\uFEFFNo,Name,Status\n1001,사과,1",
			wantRecords: [][]string{
				{"1001", "사과", "1"},
			},
			wantErr: false,
		},
		{
			name: "성공: 선행·후행 공백 자동 Trim 검증",
			input: "No, Name, Status\n" +
				" 1001 ,  사과  , 1 ",
			wantRecords: [][]string{
				{"1001", "사과", "1"},
			},
			wantErr: false,
		},
		{
			name: "성공: 주석(#) 행 건너뜀",
			input: "No,Name,Status\n" +
				"# 이 라인은 주석입니다\n" +
				"1001,사과,1\n" +
				"# 1002,바나나,0",
			wantRecords: [][]string{
				{"1001", "사과", "1"},
			},
			wantErr: false,
		},
		{
			name: "성공: 필수 컬럼 외 추가 컬럼 존재 — 유연하게 파싱",
			input: "No,Name,Status,Memo\n" +
				"1001,사과,1,청송 사과",
			wantRecords: [][]string{
				{"1001", "사과", "1", "청송 사과"},
			},
			wantErr: false,
		},
		{
			name: "성공: status 컬럼 누락 레코드 → 빈 문자열(\"\")로 자동 패딩",
			// status 없이 ID·Name만 있는 경우
			// FieldsPerRecord = -1 이므로 파싱은 성공하고, 이후 단계에서 ""로 패딩됩니다.
			input: "No,Name,Status\n" +
				"1001,사과",
			wantRecords: [][]string{
				{"1001", "사과", ""}, // status 위치에 "" 패딩
			},
			wantErr: false,
		},
		{
			name: "성공: 불완전한 따옴표(LazyQuotes) 포함 레코드 — 너그럽게 파싱",
			// LazyQuotes로 인해 Name 필드에 ",1"이 포함되므로 status 컬럼이 없어
			// 자동 패딩("") 로직에 의해 3번째 필드로 빈 문자열이 추가됩니다.
			input: "No,Name,Status\n" +
				`1001,"사과은 맛있어,1`,
			wantRecords: [][]string{
				{"1001", "사과은 맛있어,1", ""},
			},
			wantErr: false,
		},
		{
			name: "성공: ID가 같고 Name이 다른 복수 레코드 — 모두 반환",
			input: "No,Name,Status\n" +
				"1001,사과,1\n" +
				"1001,청송사과,0",
			wantRecords: [][]string{
				{"1001", "사과", "1"},
				{"1001", "청송사과", "0"},
			},
			wantErr: false,
		},

		// ── 필터링 케이스 (조용한 건너뜀) ────────────────────────────────────
		{
			name: "성공: 상품번호(ID) 비어있는 행 → 건너뜀, 나머지는 정상 수집",
			input: "No,Name,Status\n" +
				",사과,1\n" +
				"1002,바나나,1",
			wantRecords: [][]string{
				{"1002", "바나나", "1"},
			},
			wantErr: false,
		},
		{
			name: "성공: 상품명(Name) 비어있는 행 → 건너뜀, 나머지는 정상 수집",
			input: "No,Name,Status\n" +
				"1001,,1\n" +
				"1002,바나나,1",
			wantRecords: [][]string{
				{"1002", "바나나", "1"},
			},
			wantErr: false,
		},
		{
			name: "성공: 컬럼 수 부족 행(ID만 존재) → 건너뜀",
			// columnName 인덱스까지 필드가 없으면 건너뜁니다.
			input: "No,Name,Status\n" +
				"1001\n" +
				"1002,바나나,1",
			wantRecords: [][]string{
				{"1002", "바나나", "1"},
			},
			wantErr: false,
		},

		// ── 에러 케이스 ──────────────────────────────────────────────────────
		{
			name:      "실패: 완전히 빈 입력 스트림 → ErrCSVStreamReadFailed",
			input:     "",
			wantErr:   true,
			errTarget: ErrCSVStreamReadFailed,
		},
		{
			name: "실패: 헤더만 있고 데이터가 없음(빈 줄만) → ErrCSVAllRecordsFiltered",
			input: "No,Name,Status\n" +
				"   ",
			wantErr:   true,
			errTarget: ErrCSVAllRecordsFiltered,
		},
		{
			name: "실패: 헤더 컬럼 수 부족(2개) → ErrCSVInvalidHeader",
			input: "No,Name\n" +
				"1001,사과",
			wantErr:   true,
			errTarget: ErrCSVInvalidHeader,
		},
		{
			name: "실패: 모든 데이터 행이 필수값 누락으로 필터링 → ErrCSVAllRecordsFiltered",
			input: "No,Name,Status\n" +
				",사과,1\n" +
				"1002,,1\n" +
				",,",
			wantErr:   true,
			errTarget: ErrCSVAllRecordsFiltered,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := strings.NewReader(tt.input)
			got, err := parseWatchListRecords(r)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errTarget != nil {
					assert.ErrorIs(t, err, tt.errTarget)
				}
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantRecords, got)
			}
		})
	}
}

// =============================================================================
// csvWatchListLoader 통합 테스트
// =============================================================================

// TestCSVWatchListLoader_Load
// 실제 임시 파일을 생성하여 csvWatchListLoader의 파일 핸들링 및 전체 로딩 파이프라인을 검증합니다.
func TestCSVWatchListLoader_Load(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("성공: NewCSVWatchListLoader 생성자를 통한 정상 로드", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "valid_constructor.csv")
		require.NoError(t, os.WriteFile(filePath, []byte("No,Name,Status\n2001,딸기,1"), 0644))

		// 공개 생성자 경유 → 내부 구조 직접 접근 없이 인터페이스 검증
		loader := NewCSVWatchListLoader(filePath)
		records, err := loader.Load()

		require.NoError(t, err)
		require.Len(t, records, 1)
		assert.Equal(t, "2001", records[0][columnID])
		assert.Equal(t, "딸기", records[0][columnName])
		assert.Equal(t, "1", records[0][columnStatus])
	})

	t.Run("성공: 복수 레코드 정상 로드", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "multi.csv")
		content := "No,Name,Status\n1001,사과,1\n1002,바나나,0\n1003,포도,1"
		require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

		loader := &csvWatchListLoader{filePath: filePath}
		records, err := loader.Load()

		require.NoError(t, err)
		assert.Len(t, records, 3)
		assert.Equal(t, "1001", records[0][columnID])
		assert.Equal(t, "1003", records[2][columnID])
	})

	t.Run("성공: BOM이 포함된 CSV 파일 통합 로드", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "bom.csv")
		// BOM(\uFEFF) + CSV 내용
		bomContent := "\uFEFFNo,Name,Status\n3001,망고,1"
		require.NoError(t, os.WriteFile(filePath, []byte(bomContent), 0644))

		loader := &csvWatchListLoader{filePath: filePath}
		records, err := loader.Load()

		require.NoError(t, err)
		require.Len(t, records, 1)
		assert.Equal(t, "3001", records[0][columnID])
		assert.Equal(t, "망고", records[0][columnName])
	})

	t.Run("실패: 존재하지 않는 파일 → ErrWatchListFileNotFound", func(t *testing.T) {
		loader := &csvWatchListLoader{filePath: filepath.Join(tempDir, "ghost.csv")}
		records, err := loader.Load()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "존재하지 않습니다")
		assert.Nil(t, records)
	})

	t.Run("실패: 빈 파일 → CSV 스트림 읽기 실패", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "empty.csv")
		require.NoError(t, os.WriteFile(filePath, []byte(""), 0644))

		loader := &csvWatchListLoader{filePath: filePath}
		records, err := loader.Load()

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCSVStreamReadFailed)
		assert.Nil(t, records)
	})

	t.Run("실패: 헤더만 있는 CSV → 유효 레코드 없음", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "header_only.csv")
		require.NoError(t, os.WriteFile(filePath, []byte("No,Name,Status\n"), 0644))

		loader := &csvWatchListLoader{filePath: filePath}
		records, err := loader.Load()

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCSVAllRecordsFiltered)
		assert.Nil(t, records)
	})
}

// =============================================================================
// separateDuplicateRecords 테스트
// =============================================================================

// TestSeparateDuplicateRecords
// 레코드를 고유/중복으로 분리하는 separateDuplicateRecords 함수를 검증합니다.
// 특히 '상태 승격' 로직과 'Deep Copy'에 의한 원본 슬라이스 보호를 집중 검증합니다.
func TestSeparateDuplicateRecords(t *testing.T) {
	t.Parallel()

	t.Run("중복 없는 대상 — 반환값 전체 내용 비교", func(t *testing.T) {
		t.Parallel()
		input := [][]string{
			{"1001", "A", "1"},
			{"1002", "B", "0"},
		}
		distinct, duplicate := separateDuplicateRecords(input)

		assert.Len(t, distinct, 2)
		assert.Len(t, duplicate, 0)
		// 실제 값 비교
		assert.Equal(t, "1001", distinct[0][columnID])
		assert.Equal(t, "1002", distinct[1][columnID])
	})

	t.Run("단일 중복 발생 — 개수 및 ID 검증", func(t *testing.T) {
		t.Parallel()
		input := [][]string{
			{"1001", "A", "1"},
			{"1001", "A", "1"},
		}
		distinct, duplicate := separateDuplicateRecords(input)

		assert.Len(t, distinct, 1)
		assert.Len(t, duplicate, 1)
		assert.Equal(t, "1001", distinct[0][columnID])
		assert.Equal(t, "1001", duplicate[0][columnID])
	})

	t.Run("다수 중복 발생 — 개수 검증", func(t *testing.T) {
		t.Parallel()
		input := [][]string{
			{"1001", "A", "1"},
			{"1002", "B", "1"},
			{"1001", "A", "1"},
			{"1002", "B", "1"},
			{"1003", "C", "1"},
		}
		distinct, duplicate := separateDuplicateRecords(input)

		assert.Len(t, distinct, 3)
		assert.Len(t, duplicate, 2)
	})

	t.Run("핵심 로직: 비활성 고유 레코드가 활성 중복 레코드로 인해 상태 승격", func(t *testing.T) {
		t.Parallel()
		// 최초 등록된 ID=1001 은 비활성("0"), 이후 동일 ID가 활성("1")으로 재등장
		// → 고유 레코드의 status가 "1"로 승격되어야 합니다.
		input := [][]string{
			{"1001", "사과", "0"}, // 최초 등록: 비활성
			{"1001", "사과", "1"}, // 중복 등록: 활성 → 승격 트리거
		}
		distinct, duplicate := separateDuplicateRecords(input)

		require.Len(t, distinct, 1)
		require.Len(t, duplicate, 1)
		assert.Equal(t, statusEnabled, distinct[0][columnStatus], "중복 활성 레코드로 인해 고유 레코드 status가 '1'로 승격되어야 합니다")
	})

	t.Run("핵심 로직: 활성 고유 레코드가 비활성 중복 레코드로 강등되지 않아야 함", func(t *testing.T) {
		t.Parallel()
		// 최초 등록된 ID=1001 은 활성("1"), 이후 비활성("0") 재등장
		// → 고유 레코드의 status는 "1"을 유지해야 합니다.
		input := [][]string{
			{"1001", "사과", "1"}, // 최초 등록: 활성
			{"1001", "사과", "0"}, // 중복 등록: 비활성 → 강등 시도 (막아야 함)
		}
		distinct, _ := separateDuplicateRecords(input)

		require.Len(t, distinct, 1)
		assert.Equal(t, statusEnabled, distinct[0][columnStatus], "비활성 중복 레코드로 인해 기존 활성 고유 레코드가 강등되어서는 안 됩니다")
	})

	t.Run("핵심 로직: Deep Copy 검증 — 원본 슬라이스 수정이 고유 레코드에 영향 없어야 함", func(t *testing.T) {
		t.Parallel()
		original := []string{"1001", "사과", "1"}
		input := [][]string{original}

		distinct, _ := separateDuplicateRecords(input)
		require.Len(t, distinct, 1)

		// 원본 슬라이스를 수정해도 distinct의 값은 변하지 않아야 합니다.
		original[columnName] = "변경된사과"

		assert.Equal(t, "사과", distinct[0][columnName], "Deep Copy로 인해 원본 슬라이스 수정이 distinct에 반영되어서는 안 됩니다")
	})

	t.Run("컬럼 수 부족 레코드(len <= columnStatus) 자동 무시", func(t *testing.T) {
		t.Parallel()
		// columnStatus 인덱스(2)까지 필드 수가 부족한 레코드 → 상태 접근 불가로 무시
		input := [][]string{
			{"1001", "A"}, // len=2, columnStatus(2)에 접근 불가
			{"1002"},      // len=1
		}
		distinct, duplicate := separateDuplicateRecords(input)

		assert.Len(t, distinct, 0)
		assert.Len(t, duplicate, 0)
	})

	t.Run("빈 입력 슬라이스 — 빈 결과 반환", func(t *testing.T) {
		t.Parallel()
		distinct, duplicate := separateDuplicateRecords([][]string{})

		assert.Len(t, distinct, 0)
		assert.Len(t, duplicate, 0)
	})

	t.Run("빈 행({}) 포함 입력 — 빈 행은 무시하고 나머지 처리", func(t *testing.T) {
		t.Parallel()
		input := [][]string{
			{"1001", "A", "1"},
			{},
			{"1002", "B", "1"},
		}
		distinct, duplicate := separateDuplicateRecords(input)

		assert.Len(t, distinct, 2)
		assert.Len(t, duplicate, 0)
	})

	t.Run("단일 레코드 입력 — distinct 1개, duplicate 0개", func(t *testing.T) {
		t.Parallel()
		input := [][]string{{"1001", "A", "1"}}
		distinct, duplicate := separateDuplicateRecords(input)

		assert.Len(t, distinct, 1)
		assert.Len(t, duplicate, 0)
		assert.Equal(t, "1001", distinct[0][columnID])
	})
}
