package lotto

import (
	"fmt"
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
			name: "Normal Output",
			input: `
[INFO] Start Prediction
...
로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:C:\Users\test\result.log)
[INFO] End
`,
			expectedPath:  `C:\Users\test\result.log`,
			expectedError: nil,
		},
		{
			name: "Missing End Message",
			input: `
[INFO] Start Prediction
...
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := extractLogFilePath(tt.input)

			if tt.expectedError != nil {
				assert.Error(t, err)
				// 같은 에러 타입과 메시지를 포함하는지 확인
				assert.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPath, path)
			}
		})
	}
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
				fmt.Println("Parsed Message:\n", msg) // 확인용 출력
			}
		})
	}
}

func FuzzExtractLogFilePath(f *testing.F) {
	// 1. Seed Corpus 추가 (기존 테스트 케이스 활용)
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

		// Fuzzing의 목적은 Crash(Panic)가 나지 않는 것을 확인하는 것
		// 정상적인 입력이 아니면 에러가 나거나 빈 문자열이 나와야 함
		if err == nil {
			// 에러가 없다면 경로가 비어있지 않아야 함 (정상 추출)
			if path == "" {
				t.Errorf("Error is nil but path is empty for input: %q", input)
			}
		}
	})
}

func FuzzParseAnalysisResult(f *testing.F) {
	// 1. Seed Corpus 추가
	f.Add(`
- 분석결과
당첨 확률이 높은 당첨번호 목록(5개)중에서 5개의 당첨번호가 추출되었습니다.
당첨번호1 [ 1, 2, 3, 4, 5, 6 ]
`)
	f.Add(`Random Valid Like Content - 분석결과 당첨번호1 [ 1, 2 ]`)
	f.Add(`Empty`)

	// 2. Fuzzing 실행
	f.Fuzz(func(t *testing.T, input string) {
		result, err := parseAnalysisResult(input)

		if err == nil {
			// 에러가 없다면 결과 메시지가 비어있지 않아야 함
			if result == "" {
				t.Errorf("Error is nil but result is empty for input: %q", input)
			}
		}
	})
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
	// 벤치마크 루프
	for i := 0; i < b.N; i++ {
		_, _ = parseAnalysisResult(input)
	}
}
