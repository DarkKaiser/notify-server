package jdc

import (
	"testing"

	"github.com/darkkaiser/notify-server/service/task"
	"github.com/stretchr/testify/assert"
)

func TestJdcOnlineEducationCourse_String(t *testing.T) {
	t.Run("HTML 메시지 포맷", func(t *testing.T) {
		course := &jdcOnlineEducationCourse{
			Title1:         "테스트 교육",
			Title2:         "상세 제목",
			TrainingPeriod: "2025-01-01 ~ 2025-01-31",
			URL:            "https://example.com/course/1",
		}

		result := course.String(true, "")

		assert.Contains(t, result, "테스트 교육", "교육 제목이 포함되어야 합니다")
		assert.Contains(t, result, "2025-01-01", "교육 기간이 포함되어야 합니다")
		assert.Contains(t, result, "<a href", "HTML 링크 태그가 포함되어야 합니다")
	})

	t.Run("텍스트 메시지 포맷", func(t *testing.T) {
		course := &jdcOnlineEducationCourse{
			Title1:         "테스트 교육",
			Title2:         "상세 제목",
			TrainingPeriod: "2025-01-01 ~ 2025-01-31",
			URL:            "https://example.com/course/1",
		}

		result := course.String(false, "")

		assert.Contains(t, result, "테스트 교육", "교육 제목이 포함되어야 합니다")
		assert.NotContains(t, result, "<a href", "HTML 태그가 포함되지 않아야 합니다")
	})
}

func TestJdcTask_RunWatchNewOnlineEducation(t *testing.T) {
	t.Run("정상적인 강의 정보 파싱", func(t *testing.T) {
		mockFetcher := task.NewMockHTTPFetcher()

		// 1. Digital Education List Page
		digitalListURL := "http://전남디지털역량.com/product/list?type=digital_edu"
		digitalListHTML := `
			<div id="content">
				<ul class="prdt-list2">
					<li><a href="detail_digital_1" class="link">강의1</a></li>
				</ul>
			</div>`
		mockFetcher.SetResponse(digitalListURL, []byte(digitalListHTML))

		// 2. Untact Education List Page
		untactListURL := "http://전남디지털역량.com/product/list?type=untact_edu"
		untactListHTML := `
			<div id="content">
				<ul class="prdt-list2">
					<li><a href="detail_untact_1" class="link">강의2</a></li>
				</ul>
			</div>`
		mockFetcher.SetResponse(untactListURL, []byte(untactListHTML))

		// 3. Course Detail Page 1 (Digital)
		detailDigitalURL := "http://전남디지털역량.com/product/detail_digital_1"
		detailDigitalHTML := `
			<table class="prdt-tbl">
				<tbody>
					<tr>
						<td>
							<a href="real_detail_1">디지털 기초</a>
							<p>스마트폰 활용</p>
						</td>
						<td>2025-01-01 ~ 2025-01-31</td>
						<td>접수중</td>
					</tr>
				</tbody>
			</table>`
		mockFetcher.SetResponse(detailDigitalURL, []byte(detailDigitalHTML))

		// 4. Course Detail Page 2 (Untact)
		detailUntactURL := "http://전남디지털역량.com/product/detail_untact_1"
		detailUntactHTML := `
			<table class="prdt-tbl">
				<tbody>
					<tr>
						<td>
							<a href="real_detail_2">비대면 교육</a>
							<p>줌 활용법</p>
						</td>
						<td>2025-02-01 ~ 2025-02-28</td>
						<td>마감</td>
					</tr>
				</tbody>
			</table>`
		mockFetcher.SetResponse(detailUntactURL, []byte(detailUntactHTML))

		// Task Setup
		tTask := &jdcTask{
			Task: task.Task{
				ID:        TidJdc,
				CommandID: TcidJdcWatchNewOnlineEducation,
				Fetcher:   mockFetcher,
			},
		}

		// Initial Run
		taskResultData := &jdcWatchNewOnlineEducationResultData{}
		message, changedData, err := tTask.executeWatchNewOnlineEducation(taskResultData, false)

		assert.NoError(t, err)
		assert.Contains(t, message, "디지털 기초", "디지털 교육 제목이 포함되어야 합니다")
		assert.Contains(t, message, "스마트폰 활용", "디지털 교육 상세 제목이 포함되어야 합니다")
		assert.Contains(t, message, "비대면 교육", "비대면 교육 제목이 포함되어야 합니다")
		assert.Contains(t, message, "줌 활용법", "비대면 교육 상세 제목이 포함되어야 합니다")

		assert.NotNil(t, changedData)
		resultData := changedData.(*jdcWatchNewOnlineEducationResultData)
		assert.Equal(t, 2, len(resultData.OnlineEducationCourses), "2개의 강의가 추출되어야 합니다")
	})

	t.Run("데이터가 없는 경우", func(t *testing.T) {
		mockFetcher := task.NewMockHTTPFetcher()

		// Empty Lists
		digitalListURL := "http://전남디지털역량.com/product/list?type=digital_edu"
		mockFetcher.SetResponse(digitalListURL, []byte(`<div id="content"><div class="no-data2">데이터가 없습니다</div></div>`))

		untactListURL := "http://전남디지털역량.com/product/list?type=untact_edu"
		mockFetcher.SetResponse(untactListURL, []byte(`<div id="content"><div class="no-data2">데이터가 없습니다</div></div>`))

		// Task Setup
		tTask := &jdcTask{
			Task: task.Task{
				ID:         TidJdc,
				CommandID:  TcidJdcWatchNewOnlineEducation,
				NotifierID: "test-notifier",
				Fetcher:    mockFetcher,
				RunBy:      task.RunByScheduler,
			},
		}

		// Initial Run (Data empty)
		taskResultData := &jdcWatchNewOnlineEducationResultData{}
		message, changedData, err := tTask.executeWatchNewOnlineEducation(taskResultData, false)

		assert.NoError(t, err)
		assert.Empty(t, message, "데이터가 없으면 메시지도 없어야 합니다 (초기 실행 아님)")
		assert.Nil(t, changedData, "변경된 데이터가 없어야 합니다")
	})
}
