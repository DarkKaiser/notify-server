package jyiu

import (
	"fmt"
	"testing"

	"github.com/darkkaiser/notify-server/service/task"
	"github.com/darkkaiser/notify-server/service/task/testutil"
)

func BenchmarkJyiuTask_RunWatchNewNotice(b *testing.B) {
	// 1. Mock 설정
	mockFetcher := testutil.NewMockHTTPFetcher()

	// 공지사항 목록 HTML 생성
	noticeHTML := `<html><body><div id="contents"><table class="bbsList"><tbody>`
	for i := 0; i < 20; i++ {
		noticeHTML += fmt.Sprintf(`
			<tr>
				<td>%d</td>
				<td><a onclick="view(%d)">Notice Title %d</a></td>
				<td>Writer</td>
				<td>2023-01-01</td>
				<td>100</td>
			</tr>`, i, i, i)
	}
	noticeHTML += `</tbody></table></div></body></html>`

	mockFetcher.SetResponse(fmt.Sprintf("%sgms_005001/", jyiuBaseURL), []byte(noticeHTML))

	// Task Setup
	// noinspection GoBoolExpressions
	tTask := &jyiuTask{
		Task: task.NewBaseTask(ID, WatchNewNoticeCommand, "test_instance", "test-notifier", task.RunByScheduler),
	}
	tTask.SetFetcher(mockFetcher)

	// 3. 테스트 데이터 준비
	resultData := &jyiuWatchNewNoticeResultData{
		Notices: make([]*jyiuNotice, 0),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 벤치마크 실행
		_, _, err := tTask.executeWatchNewNotice(resultData, true)
		if err != nil {
			b.Fatalf("Task run failed: %v", err)
		}
	}
}

func BenchmarkJyiuTask_RunWatchNewEducation(b *testing.B) {
	// 1. Mock 설정
	mockFetcher := testutil.NewMockHTTPFetcher()

	// 교육 프로그램 목록 HTML 생성
	eduHTML := `<html><body><div class="gms_003001"><table class="bbsList"><tbody>`
	for i := 0; i < 20; i++ {
		eduHTML += fmt.Sprintf(`
			<tr onclick="view('%d')">
				<td>%d</td>
				<td>Category</td>
				<td>Education Title %d</td>
				<td>Target</td>
				<td>2023-01-01 ~ 2023-12-31</td>
				<td>2023-01-01 ~ 2023-01-31</td>
			</tr>`, i, i, i)
	}
	eduHTML += `</tbody></table></div></body></html>`

	mockFetcher.SetResponse(fmt.Sprintf("%sgms_003001/experienceList", jyiuBaseURL), []byte(eduHTML))

	// Task Setup
	// noinspection GoBoolExpressions
	tTask := &jyiuTask{
		Task: task.NewBaseTask(ID, WatchNewEducationCommand, "test_instance", "test-notifier", task.RunByScheduler),
	}
	tTask.SetFetcher(mockFetcher)

	// 3. 테스트 데이터 준비
	resultData := &jyiuWatchNewEducationResultData{
		Educations: make([]*jyiuEducation, 0),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 벤치마크 실행
		_, _, err := tTask.executeWatchNewEducation(resultData, true)
		if err != nil {
			b.Fatalf("Task run failed: %v", err)
		}
	}
}
