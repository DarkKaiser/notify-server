package kurly

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadWatchListRecords_TableDriven
// readWatchListRecords 함수의 다양한 입력 시나리오(정상, 에러, 엣지 케이스)를 검증합니다.
func TestReadWatchListRecords_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantRecords [][]string
		wantErr     bool
		errContains string
	}{
		{
			name: "성공: 정상적인 CSV 데이터 (헤더 + 데이터)",
			input: `No,Name,Status
1001,사과,1
1002,바나나,1`,
			wantRecords: [][]string{
				{"1001", "사과", "1"},
				{"1002", "바나나", "1"},
			},
			wantErr: false,
		},
		{
			name: "성공: 공백이 포함된 데이터 (Trim 지원)",
			input: `No, Name, Status
 1001 ,  사과  , 1 `,
			wantRecords: [][]string{
				{"1001", "사과", "1"},
			},
			wantErr: false,
		},
		{
			name: "성공: 주석(#)이 포함된 데이터",
			input: `No,Name,Status
# 이 라인은 주석입니다
1001,사과,1
# 1002,바나나,0 (주석 처리된 상품)`,
			wantRecords: [][]string{
				{"1001", "사과", "1"},
			},
			wantErr: false,
		},
		{
			name: "성공: 필수 컬럼 외 추가 컬럼 존재 (유연한 파싱)",
			input: `No,Name,Status,Description
1001,사과,1,맛있는 청송 사과`,
			wantRecords: [][]string{
				{"1001", "사과", "1", "맛있는 청송 사과"},
			},
			wantErr: false,
		},
		{
			name:  "성공: BOM(Byte Order Mark)이 포함된 데이터",
			input: "\uFEFFChecking,BOM,Removal\n1001,사과,1", // BOM 문자 포함
			wantRecords: [][]string{
				{"1001", "사과", "1"},
			},
			wantErr: false,
		},
		{
			name:        "실패: 빈 입력 스트림",
			input:       "",
			wantErr:     true,
			errContains: "비어있거나 올바르지 않은 인코딩",
		},
		{
			name: "실패: 헤더만 있고 데이터가 없음",
			input: `No,Name,Status
`,
			wantErr:     true,
			errContains: "유효한 상품 레코드가 없습니다",
		},
		{
			name: "실패: 필수 컬럼 누락 (헤더 부족)",
			input: `No,Name
1001,사과`,
			wantErr:     true,
			errContains: "헤더 형식이 올바르지 않습니다",
		},
		{
			name: "실패: 모든 데이터가 필터링됨 (필수값 누락)",
			input: `No,Name,Status
,사과,1
1002,,1
,,`,
			wantErr:     true,
			errContains: "모든 행이 필수 데이터(상품번호, 상품명) 누락으로 인해 필터링되었습니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			got, err := readWatchListRecords(r)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantRecords, got)
			}
		})
	}
}

// TestCSVWatchListLoader_Load_Integration
// 실제 임시 파일을 생성하여 CSVWatchListLoader의 파일 핸들링 및 전체 로딩 과정을 검증합니다.
func TestCSVWatchListLoader_Load_Integration(t *testing.T) {
	// 1. 임시 디렉토리 생성
	tempDir := t.TempDir()

	t.Run("성공: 정상적인 파일 로드", func(t *testing.T) {
		// 임시 CSV 파일 생성
		filePath := filepath.Join(tempDir, "valid.csv")
		content := `No,Name,Status
1001,TestItem,1`
		err := os.WriteFile(filePath, []byte(content), 0644)
		require.NoError(t, err)

		// Loader 생성 및 실행
		loader := &CSVWatchListLoader{FilePath: filePath}
		records, err := loader.Load()

		// 검증
		assert.NoError(t, err)
		assert.Len(t, records, 1)
		assert.Equal(t, "1001", records[0][0])
		assert.Equal(t, "TestItem", records[0][1])
	})

	t.Run("실패: 존재하지 않는 파일", func(t *testing.T) {
		nonExistentPath := filepath.Join(tempDir, "ghost.csv")
		loader := &CSVWatchListLoader{FilePath: nonExistentPath}

		records, err := loader.Load()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "존재하지 않습니다") // 에러 메시지 검증
		assert.Nil(t, records)
	})

	t.Run("실패: 내용이 없는 빈 파일", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "empty.csv")
		err := os.WriteFile(filePath, []byte(""), 0644)
		require.NoError(t, err)

		loader := &CSVWatchListLoader{FilePath: filePath}
		records, err := loader.Load()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "비어있거나")
		assert.Nil(t, records)
	})
}
