package jdc

import (
	"fmt"
	"testing"

	"github.com/darkkaiser/notify-server/service/task"
	"github.com/stretchr/testify/require"
)

func TestJdcTask_RunWatchNewOnlineEducation_Integration(t *testing.T) {
	// 1. Mock 설정
	mockFetcher := task.NewMockHTTPFetcher()

	// 상세 페이지 URL (목록에서 추출될 값)
	detailPath := "detail_course"
	// 상세 페이지 실제 URL (Fetch할 URL)
	fullDetailURL := jdcBaseURL + "product/" + detailPath

	// 상세 페이지 내부의 링크 (최종 결과 URL)
	finalLinkPath := "final_link"
	expectedFinalURL := jdcBaseURL + "product/" + finalLinkPath

	// 목록 페이지 HTML (digital_edu)
	listHTML := fmt.Sprintf(`
		<html>
		<body>
			<div id="content">
				<ul class="prdt-list2">
					<li>
						<a class="link" href="%s">강의 상세</a>
					</li>
				</ul>
			</div>
		</body>
		</html>
	`, detailPath)

	// 목록 페이지 HTML (untact_edu) - 데이터 없음
	emptyListHTML := `
		<html>
		<body>
			<div id="content">
				<div class="no-data2">데이터가 없습니다</div>
			</div>
		</body>
		</html>
	`

	// 상세 페이지 HTML
	title1 := "디지털 기초"
	title2 := "스마트폰 활용"
	period := "2025-01-01 ~ 2025-01-31"

	detailHTML := fmt.Sprintf(`
		<html>
		<body>
			<table class="prdt-tbl">
				<tbody>
					<tr>
						<td>
							<a href="%s">%s</a>
							<p>%s</p>
						</td>
						<td>%s</td>
						<td>접수중</td>
					</tr>
				</tbody>
			</table>
		</body>
		</html>
	`, finalLinkPath, title1, title2, period)

	// Mock 응답 설정
	mockFetcher.SetResponse(jdcBaseURL+"product/list?type=digital_edu", []byte(listHTML))
	mockFetcher.SetResponse(jdcBaseURL+"product/list?type=untact_edu", []byte(emptyListHTML))
	mockFetcher.SetResponse(fullDetailURL, []byte(detailHTML))

	// 2. Task 초기화
	tTask := &jdcTask{
		Task: task.NewBaseTask(TidJdc, TcidJdcWatchNewOnlineEducation, "test_instance", "test-notifier", task.RunByUnknown),
	}
	tTask.SetFetcher(mockFetcher)

	// 초기 결과 데이터 (비어있음)
	resultData := &jdcWatchNewOnlineEducationResultData{
		OnlineEducationCourses: make([]*jdcOnlineEducationCourse, 0),
	}

	// 3. 실행
	message, newResultData, err := tTask.executeWatchNewOnlineEducation(resultData, true)

	// 4. 검증
	require.NoError(t, err)
	require.NotNil(t, newResultData)

	// 결과 데이터 타입 변환
	typedResultData, ok := newResultData.(*jdcWatchNewOnlineEducationResultData)
	require.True(t, ok)
	require.Equal(t, 1, len(typedResultData.OnlineEducationCourses))

	course := typedResultData.OnlineEducationCourses[0]
	require.Equal(t, title1, course.Title1)
	require.Equal(t, title2, course.Title2)
	require.Equal(t, period, course.TrainingPeriod)
	require.Equal(t, expectedFinalURL, course.URL)

	// 메시지 검증
	require.Contains(t, message, "새로운 온라인교육 강의가 등록되었습니다")
	require.Contains(t, message, title1)
	require.Contains(t, message, "🆕")
}

func TestJdcTask_RunWatchNewOnlineEducation_NetworkError(t *testing.T) {
	// 1. Mock 설정
	mockFetcher := task.NewMockHTTPFetcher()
	url := jdcBaseURL + "product/list?type=digital_edu"
	mockFetcher.SetError(url, fmt.Errorf("network error"))

	// 2. Task 초기화
	tTask := &jdcTask{
		Task: task.NewBaseTask(TidJdc, TcidJdcWatchNewOnlineEducation, "test_instance", "test-notifier", task.RunByUnknown),
	}
	tTask.SetFetcher(mockFetcher)

	resultData := &jdcWatchNewOnlineEducationResultData{}

	// 3. 실행
	_, _, err := tTask.executeWatchNewOnlineEducation(resultData, true)

	// 4. 검증
	require.Error(t, err)
	require.Contains(t, err.Error(), "network error")
}

func TestJdcTask_RunWatchNewOnlineEducation_ParsingError(t *testing.T) {
	// 1. Mock 설정
	mockFetcher := task.NewMockHTTPFetcher()
	url := jdcBaseURL + "product/list?type=digital_edu"
	// 필수 요소가 누락된 HTML
	mockFetcher.SetResponse(url, []byte(`<html><body><h1>No Course Info</h1></body></html>`))

	// 2. Task 초기화
	tTask := &jdcTask{
		Task: task.NewBaseTask(TidJdc, TcidJdcWatchNewOnlineEducation, "test_instance", "test-notifier", task.RunByUnknown),
	}
	tTask.SetFetcher(mockFetcher)

	resultData := &jdcWatchNewOnlineEducationResultData{}

	// 3. 실행
	_, _, err := tTask.executeWatchNewOnlineEducation(resultData, true)

	// 4. 검증
	require.Error(t, err)
	// webScrape 함수에서 발생하는 에러 확인
	// "문서구조가 변경되었습니다" 메시지 예상
	require.Contains(t, err.Error(), "문서구조가 변경되었습니다")
}

func TestJdcTask_RunWatchNewOnlineEducation_NoChange(t *testing.T) {
	// 데이터 변화 없음 시나리오 (스케줄러 실행)
	mockFetcher := task.NewMockHTTPFetcher()

	// 상세 페이지 URL (목록에서 추출될 값)
	detailPath := "detail_course"
	// 상세 페이지 실제 URL (Fetch할 URL)
	fullDetailURL := jdcBaseURL + "product/" + detailPath

	// 상세 페이지 내부의 링크 (최종 결과 URL)
	finalLinkPath := "final_link"
	expectedFinalURL := jdcBaseURL + "product/" + finalLinkPath

	// 목록 페이지 HTML (digital_edu)
	listHTML := fmt.Sprintf(`
		<html>
		<body>
			<div id="content">
				<ul class="prdt-list2">
					<li>
						<a class="link" href="%s">강의 상세</a>
					</li>
				</ul>
			</div>
		</body>
		</html>
	`, detailPath)

	// 목록 페이지 HTML (untact_edu) - 데이터 없음
	emptyListHTML := `
		<html>
		<body>
			<div id="content">
				<div class="no-data2">데이터가 없습니다</div>
			</div>
		</body>
		</html>
	`

	// 상세 페이지 HTML
	title1 := "디지털 기초"
	title2 := "스마트폰 활용"
	period := "2025-01-01 ~ 2025-01-31"

	detailHTML := fmt.Sprintf(`
		<html>
		<body>
			<table class="prdt-tbl">
				<tbody>
					<tr>
						<td>
							<a href="%s">%s</a>
							<p>%s</p>
						</td>
						<td>%s</td>
						<td>접수중</td>
					</tr>
				</tbody>
			</table>
		</body>
		</html>
	`, finalLinkPath, title1, title2, period)

	// Mock 응답 설정
	mockFetcher.SetResponse(jdcBaseURL+"product/list?type=digital_edu", []byte(listHTML))
	mockFetcher.SetResponse(jdcBaseURL+"product/list?type=untact_edu", []byte(emptyListHTML))
	mockFetcher.SetResponse(fullDetailURL, []byte(detailHTML))

	tTask := &jdcTask{
		Task: task.NewBaseTask(TidJdc, TcidJdcWatchNewOnlineEducation, "test_instance", "test-notifier", task.RunByScheduler),
	}
	tTask.SetFetcher(mockFetcher)

	// 기존 결과 데이터 (동일한 데이터)
	resultData := &jdcWatchNewOnlineEducationResultData{
		OnlineEducationCourses: []*jdcOnlineEducationCourse{
			{
				Title1:         title1,
				Title2:         title2,
				TrainingPeriod: period,
				URL:            expectedFinalURL,
			},
		},
	}

	// 실행
	message, newResultData, err := tTask.executeWatchNewOnlineEducation(resultData, true)

	// 검증
	require.NoError(t, err)
	require.Empty(t, message)     // 변화 없으면 메시지 없음
	require.Nil(t, newResultData) // 변화 없으면 nil 반환
}

func TestJdcTask_RunWatchNewOnlineEducation_NewEducation(t *testing.T) {
	// 신규 강의 시나리오
	mockFetcher := task.NewMockHTTPFetcher()

	// 상세 페이지 URL (목록에서 추출될 값)
	detailPath1 := "detail_course_1"
	detailPath2 := "detail_course_2"

	// 목록 페이지 HTML (digital_edu)
	listHTML := fmt.Sprintf(`
		<html>
		<body>
			<div id="content">
				<ul class="prdt-list2">
					<li><a class="link" href="%s">강의 상세 1</a></li>
					<li><a class="link" href="%s">강의 상세 2</a></li>
				</ul>
			</div>
		</body>
		</html>
	`, detailPath1, detailPath2)

	// 목록 페이지 HTML (untact_edu) - 데이터 없음
	emptyListHTML := `
		<html>
		<body>
			<div id="content">
				<div class="no-data2">데이터가 없습니다</div>
			</div>
		</body>
		</html>
	`

	// 상세 페이지 1 HTML (기존 강의)
	title1_1 := "기존 강의"
	title1_2 := "기존 강의 상세"
	period1 := "2025-01-01 ~ 2025-01-31"
	finalLinkPath1 := "final_link_1"
	detailHTML1 := fmt.Sprintf(`
		<html>
		<body>
			<table class="prdt-tbl">
				<tbody>
					<tr>
						<td>
							<a href="%s">%s</a>
							<p>%s</p>
						</td>
						<td>%s</td>
						<td>접수중</td>
					</tr>
				</tbody>
			</table>
		</body>
		</html>
	`, finalLinkPath1, title1_1, title1_2, period1)

	// 상세 페이지 2 HTML (신규 강의)
	title2_1 := "신규 강의"
	title2_2 := "신규 강의 상세"
	period2 := "2025-02-01 ~ 2025-02-28"
	finalLinkPath2 := "final_link_2"
	detailHTML2 := fmt.Sprintf(`
		<html>
		<body>
			<table class="prdt-tbl">
				<tbody>
					<tr>
						<td>
							<a href="%s">%s</a>
							<p>%s</p>
						</td>
						<td>%s</td>
						<td>접수중</td>
					</tr>
				</tbody>
			</table>
		</body>
		</html>
	`, finalLinkPath2, title2_1, title2_2, period2)

	// Mock 응답 설정
	mockFetcher.SetResponse(jdcBaseURL+"product/list?type=digital_edu", []byte(listHTML))
	mockFetcher.SetResponse(jdcBaseURL+"product/list?type=untact_edu", []byte(emptyListHTML))
	mockFetcher.SetResponse(jdcBaseURL+"product/"+detailPath1, []byte(detailHTML1))
	mockFetcher.SetResponse(jdcBaseURL+"product/"+detailPath2, []byte(detailHTML2))

	tTask := &jdcTask{
		Task: task.NewBaseTask(TidJdc, TcidJdcWatchNewOnlineEducation, "test_instance", "test-notifier", task.RunByUnknown),
	}
	tTask.SetFetcher(mockFetcher)

	// 기존 결과 데이터 (기존 강의만 있음)
	resultData := &jdcWatchNewOnlineEducationResultData{
		OnlineEducationCourses: []*jdcOnlineEducationCourse{
			{
				Title1:         title1_1,
				Title2:         title1_2,
				TrainingPeriod: period1,
				URL:            jdcBaseURL + "product/" + finalLinkPath1,
			},
		},
	}

	// 실행
	message, newResultData, err := tTask.executeWatchNewOnlineEducation(resultData, true)

	// 검증
	require.NoError(t, err)
	require.NotEmpty(t, message)
	require.Contains(t, message, "새로운 온라인교육 강의가 등록되었습니다")
	require.Contains(t, message, title2_1)
	require.Contains(t, message, "🆕")

	typedResultData, ok := newResultData.(*jdcWatchNewOnlineEducationResultData)
	require.True(t, ok)
	require.Equal(t, 2, len(typedResultData.OnlineEducationCourses))
}
