package jdc

import (
	"fmt"
	"testing"

	"github.com/darkkaiser/notify-server/service/task"
	"github.com/darkkaiser/notify-server/service/task/testutil"
)

func BenchmarkJdcTask_RunWatchNewOnlineEducation(b *testing.B) {
	// 1. Mock 설정
	mockFetcher := testutil.NewMockHTTPFetcher()

	// 목록 페이지 HTML 생성 (여러 개의 강의 포함)
	listHTML := `<html><body><div id="content"><ul class="prdt-list2">`
	for i := 0; i < 20; i++ {
		listHTML += fmt.Sprintf(`<li><a class="link" href="detail?id=%d">Course %d</a></li>`, i, i)
	}
	listHTML += `</ul></div></body></html>`

	// 목록 페이지 응답 설정 (두 가지 타입 모두)
	mockFetcher.SetResponse(fmt.Sprintf("%sproduct/list?type=digital_edu", jdcBaseURL), []byte(listHTML))
	mockFetcher.SetResponse(fmt.Sprintf("%sproduct/list?type=untact_edu", jdcBaseURL), []byte(listHTML))

	// 상세 페이지 HTML 생성 및 응답 설정
	detailHTML := `
		<html><body>
			<table class="prdt-tbl"><tbody>
				<tr>
					<td>
						<a href="detail_view?id=123"><h3>Course Title 1</h3></a>
						<p>Course Title 2</p>
					</td>
					<td>2023-01-01 ~ 2023-12-31</td>
					<td>Wait</td>
				</tr>
			</tbody></table>
		</body></html>
	`
	// 목록에 있는 모든 상세 페이지에 대해 응답 설정
	// 목록에 있는 모든 상세 페이지에 대해 응답 설정
	for i := 0; i < 20; i++ {
		mockFetcher.SetResponse(fmt.Sprintf("%sproduct/detail?id=%d", jdcBaseURL, i), []byte(detailHTML))
	}

	// 2. Task 초기화
	// Task Setup
	// noinspection GoBoolExpressions
	tTask := &jdcTask{
		Task: task.NewBaseTask(ID, WatchNewOnlineEducationCommand, "test_instance", "test-notifier", task.RunByScheduler),
	}
	tTask.SetFetcher(mockFetcher)

	// 3. 테스트 데이터 준비
	resultData := &jdcWatchNewOnlineEducationResultData{
		OnlineEducationCourses: make([]*jdcOnlineEducationCourse, 0),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 벤치마크 실행
		_, _, err := tTask.executeWatchNewOnlineEducation(resultData, true)
		if err != nil {
			b.Fatalf("Task run failed: %v", err)
		}
	}
}
