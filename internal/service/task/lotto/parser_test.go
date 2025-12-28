package lotto

import (
	"fmt"
	"strings"
	"testing"

	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestExtractLogFilePath(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedPath  string
		expectedError error
	}{
		{
			name: "Normal Output (Windows Path)",
			input: `
[INFO] Start Prediction
로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:C:\Users\test\result.log)
[INFO] End
`,
			expectedPath:  `C:\Users\test\result.log`,
			expectedError: nil,
		},
		{
			name: "Normal Output (Unix Path with Spaces)",
			input: `
로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:/home/user/my project/result.log)
`,
			expectedPath:  `/home/user/my project/result.log`,
			expectedError: nil,
		},
		{
			name: "Normal Output (Mixed Separators)",
			input: `
로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:D:/Data/Projects\Lotto/result_123.log)
`,
			expectedPath:  `D:/Data/Projects\Lotto/result_123.log`,
			expectedError: nil,
		},
		{
			name: "Missing End Message",
			input: `
[INFO] Start Prediction
Processing...
`,
			expectedPath:  "",
			expectedError: apperrors.New(apperrors.ExecutionFailed, "당첨번호 예측 작업이 정상적으로 완료되었는지 확인할 수 없습니다. 자세한 내용은 로그를 확인하여 주세요"),
		},
		{
			name: "End Message But No Path",
			input: `
로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로 없음)
`,
			expectedPath:  "",
			expectedError: apperrors.New(apperrors.ExecutionFailed, "당첨번호 예측 결과가 저장되어 있는 파일의 경로를 찾을 수 없습니다. 자세한 내용은 로그를 확인하여 주세요"),
		},
		{
			name:          "Empty Input",
			input:         "",
			expectedPath:  "",
			expectedError: apperrors.New(apperrors.ExecutionFailed, "당첨번호 예측 작업이 정상적으로 완료되었는지 확인할 수 없습니다. 자세한 내용은 로그를 확인하여 주세요"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := extractLogFilePath(tt.input)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPath, path)
			}
		})
	}
}

func Example_extractLogFilePath() {
	input := `
[INFO] Processing...
로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:/tmp/lotto/result.log)
[INFO] Done.
`
	path, err := extractLogFilePath(input)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(path)
	// Output: /tmp/lotto/result.log
}

func TestParseAnalysisResult(t *testing.T) {
	validResult := `
======================
- 분석결과
======================
당첨 확률이 높은 당첨번호 목록(5개)중에서 5개의 당첨번호가 추출되었습니다.

당첨번호1 [ 1, 2, 3, 4, 5, 6 ]
당첨번호2 [ 7, 8, 9, 10, 11, 12 ]
당첨번호3 [ 13, 14, 15, 16, 17, 18 ]
당첨번호4 [ 19, 20, 21, 22, 23, 24 ]
당첨번호5 [ 25, 26, 27, 28, 29, 30 ]
`

	tests := []struct {
		name          string
		input         string
		expectedError error
		contains      []string
	}{
		{
			name:          "Valid Content",
			input:         validResult,
			expectedError: nil,
			contains: []string{
				"당첨 확률이 높은 당첨번호 목록",
				"• 당첨번호1 [ 1, 2, 3, 4, 5, 6 ]",
				"• 당첨번호5 [ 25, 26, 27, 28, 29, 30 ]",
			},
		},
		{
			name: "Missing Analysis Header",
			input: `
이상한 로그 내용
헤더가 없습니다.
`,
			expectedError: apperrors.New(apperrors.ExecutionFailed, "당첨번호 예측 결과 파일의 내용이 유효하지 않습니다 (- 분석결과 없음). 자세한 내용은 로그를 확인하여 주세요"),
			contains:      nil,
		},
		{
			name: "Partial Content (Missing Numbers)",
			input: `
- 분석결과
당첨 확률이 높은 당첨번호 목록(5개)중에서 5개의 당첨번호가 추출되었습니다.
당첨번호1 [ 1, 2, 3, 4, 5, 6 ]
`,
			expectedError: nil,
			contains: []string{
				"• 당첨번호1 [ 1, 2, 3, 4, 5, 6 ]",
				// 나머지는 정규식 매칭 안되면 빈 문자열로 나올 것임 (parser 구현상 에러는 아님)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := parseAnalysisResult(tt.input)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				for _, c := range tt.contains {
					assert.Contains(t, msg, c)
				}
			}
		})
	}
}

func Example_parseAnalysisResult() {
	input := `
- 분석결과
당첨번호1 [ 1, 2, 3, 4, 5, 6 ]
당첨번호2 [ 7, 8, 9, 10, 11, 12 ]
당첨번호3 [ 13, 14, 15, 16, 17, 18 ]
당첨번호4 [ 19, 20, 21, 22, 23, 24 ]
당첨번호5 [ 25, 26, 27, 28, 29, 30 ]
`
	// Note: 실제 함수는 헤더나 결과 형식이 조금 다를 수 있지만, 기본적으로 "- 분석결과" 이후를 파싱합니다.
	// 정확한 정규식 매칭을 위해선 전체 문맥이 필요할 수 있습니다.

	// 테스트를 위해 필요한 최소 입력을 구성합니다.
	fullInput := `
- 분석결과
당첨 확률이 높은 당첨번호 목록(5개)중에서 5개의 당첨번호가 추출되었습니다.
` + input

	msg, err := parseAnalysisResult(fullInput)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// 편의상 결과 내용 중 첫 줄만 출력하여 확인합니다.
	// 결과에는 "\r\n"이 포함되어 있습니다.
	lines := strings.Split(msg, "\r\n")
	if len(lines) > 0 {
		fmt.Println(lines[0])
	}
	// Output: 당첨 확률이 높은 당첨번호 목록(5개)중에서 5개의 당첨번호가 추출되었습니다.
}

// --- Fuzz Tests ---

func FuzzExtractLogFilePath(f *testing.F) {
	// 1. Seed Corpus 추가
	f.Add(`
[INFO] Start Prediction
로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:C:\Users\test\result.log)
[INFO] End
`)
	f.Add(`Invalid Output`)
	f.Add(``)

	// 2. Fuzzing 실행
	f.Fuzz(func(t *testing.T, input string) {
		path, err := extractLogFilePath(input)

		// Panic이 발생하지 않아야 함
		if err == nil {
			// 에러가 없다면 경로가 비어있지 않아야 함
			if path == "" {
				t.Errorf("Error is nil but path is empty for input: %q", input)
			}
		}
	})
}

func FuzzParseAnalysisResult(f *testing.F) {
	f.Add(`
- 분석결과
당첨 확률이 높은 당첨번호 목록(5개)중에서 5개의 당첨번호가 추출되었습니다.
당첨번호1 [ 1, 2, 3, 4, 5, 6 ]
`)
	f.Add(`Random Valid Like Content - 분석결과 당첨번호1 [ 1, 2 ]`)
	f.Add(`Empty`)

	f.Fuzz(func(t *testing.T, input string) {
		result, err := parseAnalysisResult(input)

		if err == nil {
			if result == "" {
				t.Errorf("Error is nil but result is empty for input: %q", input)
			}
		}
	})
}

// --- Benchmark Tests ---

func BenchmarkExtractLogFilePath(b *testing.B) {
	input := `
[INFO] Start Prediction
[INFO] Running...
[INFO] Running...
로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:C:\Users\test\log\my_result_12345.log)
[INFO] End
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = extractLogFilePath(input)
	}
}

func BenchmarkParseAnalysisResult(b *testing.B) {
	input := `
======================
- 분석결과
======================
당첨 확률이 높은 당첨번호 목록(5개)중에서 5개의 당첨번호가 추출되었습니다.

당첨번호1 [ 1, 2, 3, 4, 5, 6 ]
당첨번호2 [ 7, 8, 9, 10, 11, 12 ]
당첨번호3 [ 13, 14, 15, 16, 17, 18 ]
당첨번호4 [ 19, 20, 21, 22, 23, 24 ]
당첨번호5 [ 25, 26, 27, 28, 29, 30 ]
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parseAnalysisResult(input)
	}
}
