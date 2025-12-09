package jyiu

import (
	"fmt"
	"testing"

	"github.com/darkkaiser/notify-server/service/task"
	"github.com/stretchr/testify/require"
)

func TestJyiuTask_RunWatchNewNotice_Integration(t *testing.T) {
	// 1. Mock 설정
	mockFetcher := task.NewMockHTTPFetcher()

	// 테스트용 HTML 응답 생성
	noticeTitle := "테스트 공지사항"
	noticeDate := "2025-11-28"
	noticeID := "12345"

	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<div id="contents">
				<table class="bbsList">
					<tbody>
						<tr>
							<td>1</td>
							<td><a href="#" onclick="view(%s); return false;">%s</a></td>
							<td>관리자</td>
							<td>%s</td>
							<td>10</td>
						</tr>
					</tbody>
				</table>
			</div>
		</body>
		</html>
	`, noticeID, noticeTitle, noticeDate)

	url := "https://www.jyiu.or.kr/gms_005001/"
	mockFetcher.SetResponse(url, []byte(htmlContent))

	// 2. Task 초기화
	tTask := &jyiuTask{
		Task: task.Task{
			ID:         TidJyiu,
			CommandID:  TcidJyiuWatchNewNotice,
			NotifierID: "test-notifier",
			Fetcher:    mockFetcher,
			RunBy:      task.RunByScheduler,
		},
	}

	// 초기 결과 데이터 (비어있음)
	resultData := &jyiuWatchNewNoticeResultData{
		Notices: make([]*jyiuNotice, 0),
	}

	// 3. 실행
	message, newResultData, err := tTask.runWatchNewNotice(resultData, true)

	// 4. 검증
	require.NoError(t, err)
	require.NotNil(t, newResultData)

	// 결과 데이터 타입 변환
	typedResultData, ok := newResultData.(*jyiuWatchNewNoticeResultData)
	require.True(t, ok)
	require.Equal(t, 1, len(typedResultData.Notices))

	notice := typedResultData.Notices[0]
	require.Equal(t, noticeTitle, notice.Title)
	require.Equal(t, noticeDate, notice.Date)
	require.Equal(t, fmt.Sprintf("https://www.jyiu.or.kr/gms_005001/view?id=%s", noticeID), notice.URL)

	// 메시지 검증 (신규 공지사항 알림)
	require.Contains(t, message, "새로운 공지사항이 등록되었습니다")
	require.Contains(t, message, noticeTitle)
	require.Contains(t, message, "🆕")
}

func TestJyiuTask_RunWatchNewEducation_Integration(t *testing.T) {
	// 1. Mock 설정
	mockFetcher := task.NewMockHTTPFetcher()

	// 테스트용 HTML 응답 생성
	eduTitle := "테스트 교육"
	eduTrainingPeriod := "2025-12-01 ~ 2025-12-31"
	eduAcceptancePeriod := "2025-11-01 ~ 2025-11-30"
	eduURL := "/gms_003001/view?id=67890"

	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<div class="gms_003001">
				<table class="bbsList">
					<tbody>
						<tr onclick="location.href='%s'">
							<td>1</td>
							<td>교육</td>
							<td>%s</td>
							<td>모집중</td>
							<td>%s</td>
							<td>%s</td>
						</tr>
					</tbody>
				</table>
			</div>
		</body>
		</html>
	`, eduURL, eduTitle, eduTrainingPeriod, eduAcceptancePeriod)

	url := "https://www.jyiu.or.kr/gms_003001/experienceList"
	mockFetcher.SetResponse(url, []byte(htmlContent))

	// 2. Task 초기화
	tTask := &jyiuTask{
		Task: task.Task{
			ID:         TidJyiu,
			CommandID:  TcidJyiuWatchNewEducation,
			NotifierID: "test-notifier",
			Fetcher:    mockFetcher,
			RunBy:      task.RunByScheduler,
		},
	}

	// 초기 결과 데이터 (비어있음)
	resultData := &jyiuWatchNewEducationResultData{
		Educations: make([]*jyiuEducation, 0),
	}

	// 3. 실행
	message, newResultData, err := tTask.runWatchNewEducation(resultData, true)

	// 4. 검증
	require.NoError(t, err)
	require.NotNil(t, newResultData)

	// 결과 데이터 타입 변환
	typedResultData, ok := newResultData.(*jyiuWatchNewEducationResultData)
	require.True(t, ok)
	require.Equal(t, 1, len(typedResultData.Educations))

	edu := typedResultData.Educations[0]
	require.Equal(t, eduTitle, edu.Title)
	require.Equal(t, eduTrainingPeriod, edu.TrainingPeriod)
	require.Equal(t, eduAcceptancePeriod, edu.AcceptancePeriod)
	require.Equal(t, "https://www.jyiu.or.kr/"+eduURL, edu.URL)

	// 메시지 검증 (신규 교육프로그램 알림)
	require.Contains(t, message, "새로운 교육프로그램이 등록되었습니다")
	require.Contains(t, message, eduTitle)
	require.Contains(t, message, "🆕")
}

func TestJyiuTask_RunWatchNewNotice_NetworkError(t *testing.T) {
	// 1. Mock 설정
	mockFetcher := task.NewMockHTTPFetcher()
	url := "https://www.jyiu.or.kr/gms_005001/"
	mockFetcher.SetError(url, fmt.Errorf("network error"))

	// 2. Task 초기화
	tTask := &jyiuTask{
		Task: task.Task{
			ID:         TidJyiu,
			CommandID:  TcidJyiuWatchNewNotice,
			NotifierID: "test-notifier",
			Fetcher:    mockFetcher,
			RunBy:      task.RunByScheduler,
		},
	}

	resultData := &jyiuWatchNewNoticeResultData{}

	// 3. 실행
	_, _, err := tTask.runWatchNewNotice(resultData, true)

	// 4. 검증
	require.Error(t, err)
	require.Contains(t, err.Error(), "network error")
}

func TestJyiuTask_RunWatchNewEducation_ParsingError(t *testing.T) {
	// 1. Mock 설정
	mockFetcher := task.NewMockHTTPFetcher()
	url := "https://www.jyiu.or.kr/gms_003001/experienceList"
	// 필수 요소가 누락된 HTML
	mockFetcher.SetResponse(url, []byte(`<html><body><h1>No Education Info</h1></body></html>`))

	// 2. Task 초기화
	tTask := &jyiuTask{
		Task: task.Task{
			ID:         TidJyiu,
			CommandID:  TcidJyiuWatchNewEducation,
			NotifierID: "test-notifier",
			Fetcher:    mockFetcher,
			RunBy:      task.RunByScheduler,
		},
	}

	resultData := &jyiuWatchNewEducationResultData{}

	// 3. 실행
	_, _, err := tTask.runWatchNewEducation(resultData, true)

	// 4. 검증
	require.Error(t, err)
	// webScrape 함수에서 발생하는 에러 확인
	// "문서구조가 변경되었습니다" 메시지 예상
	require.Contains(t, err.Error(), "문서구조가 변경되었습니다")
}

func TestJyiuTask_RunWatchNewNotice_NoChange(t *testing.T) {
	// 데이터 변화 없음 시나리오 (스케줄러 실행)
	mockFetcher := task.NewMockHTTPFetcher()
	noticeTitle := "테스트 공지사항"
	noticeDate := "2025-11-28"
	noticeID := "12345"

	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<div id="contents">
				<table class="bbsList">
					<tbody>
						<tr>
							<td>1</td>
							<td><a href="#" onclick="view(%s); return false;">%s</a></td>
							<td>관리자</td>
							<td>%s</td>
							<td>10</td>
						</tr>
					</tbody>
				</table>
			</div>
		</body>
		</html>
	`, noticeID, noticeTitle, noticeDate)

	url := "https://www.jyiu.or.kr/gms_005001/"
	mockFetcher.SetResponse(url, []byte(htmlContent))

	tTask := &jyiuTask{
		Task: task.Task{
			ID:         TidJyiu,
			CommandID:  TcidJyiuWatchNewNotice,
			NotifierID: "test-notifier",
			Fetcher:    mockFetcher,
			RunBy:      task.RunByScheduler,
		},
	}

	resultData := &jyiuWatchNewNoticeResultData{
		Notices: []*jyiuNotice{
			{
				Title: noticeTitle,
				Date:  noticeDate,
				URL:   fmt.Sprintf("https://www.jyiu.or.kr/gms_005001/view?id=%s", noticeID),
			},
		},
	}

	// 실행
	message, newResultData, err := tTask.runWatchNewNotice(resultData, true)

	// 검증
	require.NoError(t, err)
	require.Empty(t, message)     // 변화 없으면 메시지 없음
	require.Nil(t, newResultData) // 변화 없으면 nil 반환
}

func TestJyiuTask_RunWatchNewNotice_NewNotice(t *testing.T) {
	// 신규 공지사항 시나리오
	mockFetcher := task.NewMockHTTPFetcher()
	noticeTitle1 := "기존 공지사항"
	noticeDate1 := "2025-11-27"
	noticeID1 := "12345"
	noticeTitle2 := "신규 공지사항"
	noticeDate2 := "2025-11-28"
	noticeID2 := "12346"

	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<div id="contents">
				<table class="bbsList">
					<tbody>
						<tr>
							<td>2</td>
							<td><a href="#" onclick="view(%s); return false;">%s</a></td>
							<td>관리자</td>
							<td>%s</td>
							<td>10</td>
						</tr>
						<tr>
							<td>1</td>
							<td><a href="#" onclick="view(%s); return false;">%s</a></td>
							<td>관리자</td>
							<td>%s</td>
							<td>10</td>
						</tr>
					</tbody>
				</table>
			</div>
		</body>
		</html>
	`, noticeID2, noticeTitle2, noticeDate2, noticeID1, noticeTitle1, noticeDate1)

	url := "https://www.jyiu.or.kr/gms_005001/"
	mockFetcher.SetResponse(url, []byte(htmlContent))

	tTask := &jyiuTask{
		Task: task.Task{
			ID:         TidJyiu,
			CommandID:  TcidJyiuWatchNewNotice,
			NotifierID: "test-notifier",
			Fetcher:    mockFetcher,
			RunBy:      task.RunByScheduler,
		},
	}

	// 기존 결과 데이터 (기존 공지사항만 있음)
	resultData := &jyiuWatchNewNoticeResultData{
		Notices: []*jyiuNotice{
			{
				Title: noticeTitle1,
				Date:  noticeDate1,
				URL:   fmt.Sprintf("https://www.jyiu.or.kr/gms_005001/view?id=%s", noticeID1),
			},
		},
	}

	// 실행
	message, newResultData, err := tTask.runWatchNewNotice(resultData, true)

	// 검증
	require.NoError(t, err)
	require.NotEmpty(t, message)
	require.Contains(t, message, "새로운 공지사항이 등록되었습니다")
	require.Contains(t, message, noticeTitle2)
	require.Contains(t, message, "🆕")

	typedResultData, ok := newResultData.(*jyiuWatchNewNoticeResultData)
	require.True(t, ok)
	require.Equal(t, 2, len(typedResultData.Notices))
}

func TestJyiuTask_RunWatchNewEducation_NoChange(t *testing.T) {
	// 데이터 변화 없음 시나리오 (스케줄러 실행)
	mockFetcher := task.NewMockHTTPFetcher()
	eduTitle := "테스트 교육"
	eduTrainingPeriod := "2025-12-01 ~ 2025-12-31"
	eduAcceptancePeriod := "2025-11-01 ~ 2025-11-30"
	eduURL := "/gms_003001/view?id=67890"

	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<div class="gms_003001">
				<table class="bbsList">
					<tbody>
						<tr onclick="location.href='%s'">
							<td>1</td>
							<td>교육</td>
							<td>%s</td>
							<td>모집중</td>
							<td>%s</td>
							<td>%s</td>
						</tr>
					</tbody>
				</table>
			</div>
		</body>
		</html>
	`, eduURL, eduTitle, eduTrainingPeriod, eduAcceptancePeriod)

	url := "https://www.jyiu.or.kr/gms_003001/experienceList"
	mockFetcher.SetResponse(url, []byte(htmlContent))

	tTask := &jyiuTask{
		Task: task.Task{
			ID:         TidJyiu,
			CommandID:  TcidJyiuWatchNewEducation,
			NotifierID: "test-notifier",
			Fetcher:    mockFetcher,
			RunBy:      task.RunByScheduler,
		},
	}

	// 기존 결과 데이터 (동일한 데이터)
	resultData := &jyiuWatchNewEducationResultData{
		Educations: []*jyiuEducation{
			{
				Title:            eduTitle,
				TrainingPeriod:   eduTrainingPeriod,
				AcceptancePeriod: eduAcceptancePeriod,
				URL:              "https://www.jyiu.or.kr/" + eduURL,
			},
		},
	}

	// 실행
	message, newResultData, err := tTask.runWatchNewEducation(resultData, true)

	// 검증
	require.NoError(t, err)
	require.Empty(t, message)     // 변화 없으면 메시지 없음
	require.Nil(t, newResultData) // 변화 없으면 nil 반환
}

func TestJyiuTask_RunWatchNewEducation_NewEducation(t *testing.T) {
	// 신규 교육프로그램 시나리오
	mockFetcher := task.NewMockHTTPFetcher()
	eduTitle1 := "기존 교육"
	eduTrainingPeriod1 := "2025-12-01 ~ 2025-12-31"
	eduAcceptancePeriod1 := "2025-11-01 ~ 2025-11-30"
	eduURL1 := "/gms_003001/view?id=11111"
	eduTitle2 := "신규 교육"
	eduTrainingPeriod2 := "2026-01-01 ~ 2026-01-31"
	eduAcceptancePeriod2 := "2025-12-01 ~ 2025-12-31"
	eduURL2 := "/gms_003001/view?id=22222"

	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<div class="gms_003001">
				<table class="bbsList">
					<tbody>
						<tr onclick="location.href='%s'">
							<td>2</td>
							<td>교육</td>
							<td>%s</td>
							<td>모집중</td>
							<td>%s</td>
							<td>%s</td>
						</tr>
						<tr onclick="location.href='%s'">
							<td>1</td>
							<td>교육</td>
							<td>%s</td>
							<td>모집중</td>
							<td>%s</td>
							<td>%s</td>
						</tr>
					</tbody>
				</table>
			</div>
		</body>
		</html>
	`, eduURL2, eduTitle2, eduTrainingPeriod2, eduAcceptancePeriod2, eduURL1, eduTitle1, eduTrainingPeriod1, eduAcceptancePeriod1)

	url := "https://www.jyiu.or.kr/gms_003001/experienceList"
	mockFetcher.SetResponse(url, []byte(htmlContent))

	tTask := &jyiuTask{
		Task: task.Task{
			ID:         TidJyiu,
			CommandID:  TcidJyiuWatchNewEducation,
			NotifierID: "test-notifier",
			Fetcher:    mockFetcher,
			RunBy:      task.RunByScheduler,
		},
	}

	// 기존 결과 데이터 (기존 교육만 있음)
	resultData := &jyiuWatchNewEducationResultData{
		Educations: []*jyiuEducation{
			{
				Title:            eduTitle1,
				TrainingPeriod:   eduTrainingPeriod1,
				AcceptancePeriod: eduAcceptancePeriod1,
				URL:              "https://www.jyiu.or.kr/" + eduURL1,
			},
		},
	}

	// 실행
	message, newResultData, err := tTask.runWatchNewEducation(resultData, true)

	// 검증
	require.NoError(t, err)
	require.NotEmpty(t, message)
	require.Contains(t, message, "새로운 교육프로그램이 등록되었습니다")
	require.Contains(t, message, eduTitle2)
	require.Contains(t, message, "🆕")

	typedResultData, ok := newResultData.(*jyiuWatchNewEducationResultData)
	require.True(t, ok)
	require.Equal(t, 2, len(typedResultData.Educations))
}
