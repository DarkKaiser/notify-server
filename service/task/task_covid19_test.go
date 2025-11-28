package task

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCovid19MedicalInstitution_String(t *testing.T) {
	t.Run("HTML 메시지 포맷", func(t *testing.T) {
		institution := &covid19MedicalInstitution{
			ID:              "12345",
			Name:            "테스트 병원",
			VaccineQuantity: "10개",
		}

		result := institution.String(true, "")

		assert.Contains(t, result, "테스트 병원", "병원 이름이 포함되어야 합니다")
		assert.Contains(t, result, "10개", "백신 수량이 포함되어야 합니다")
		assert.Contains(t, result, "<b>", "HTML 태그가 포함되어야 합니다")
	})

	t.Run("텍스트 메시지 포맷", func(t *testing.T) {
		institution := &covid19MedicalInstitution{
			ID:              "12345",
			Name:            "테스트 병원",
			VaccineQuantity: "10개",
		}

		result := institution.String(false, "")

		assert.Contains(t, result, "테스트 병원", "병원 이름이 포함되어야 합니다")
		assert.Contains(t, result, "10개", "백신 수량이 포함되어야 합니다")
		assert.NotContains(t, result, "<b>", "HTML 태그가 포함되지 않아야 합니다")
	})

	t.Run("마크 표시", func(t *testing.T) {
		institution := &covid19MedicalInstitution{
			ID:              "12345",
			Name:            "테스트 병원",
			VaccineQuantity: "5개",
		}

		result := institution.String(false, " 🆕")

		assert.Contains(t, result, "🆕", "마크가 포함되어야 합니다")
	})
}

func TestCovid19Task_JSONParsing(t *testing.T) {
	t.Run("복잡한 JSON 구조 파싱", func(t *testing.T) {
		// 간단한 테스트용 JSON (실제 구조는 매우 복잡함)
		jsonData := `[{
			"data": {
				"rests": {
					"businesses": {
						"total": 10,
						"vaccineLastSave": 1234567890,
						"isUpdateDelayed": false,
						"items": [
							{
								"id": "12345",
								"name": "테스트 병원",
								"vaccineQuantity": {
									"list": [
										{
											"quantity": 10,
											"vaccineType": "화이자"
										}
									]
								}
							}
						]
					}
				}
			}
		}]`

		var result covid19WatchResidualVaccineSearchResultData
		err := json.Unmarshal([]byte(jsonData), &result)

		assert.NoError(t, err, "JSON 파싱이 성공해야 합니다")
		assert.Equal(t, 1, len(result), "결과 배열 길이가 1이어야 합니다")
		assert.Equal(t, 10, result[0].Data.Rests.Businesses.Total, "Total 값이 일치해야 합니다")
		assert.Equal(t, 1, len(result[0].Data.Rests.Businesses.Items), "Items 개수가 일치해야 합니다")
	})

	t.Run("빈 응답 처리", func(t *testing.T) {
		jsonData := `[]`

		var result covid19WatchResidualVaccineSearchResultData
		err := json.Unmarshal([]byte(jsonData), &result)

		assert.NoError(t, err, "빈 배열도 파싱할 수 있어야 합니다")
		assert.Equal(t, 0, len(result), "결과 배열이 비어있어야 합니다")
	})
}
