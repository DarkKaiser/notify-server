package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/pkg/cronx"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/robfig/cron/v3"
)

// component Scheduler 서비스의 로깅용 컴포넌트 이름
const component = "scheduler.service"

// taskSubmitTimeout 작업 실행 요청 시 최대 대기 시간 (블로킹 방지)
const taskSubmitTimeout = 5 * time.Second

// Scheduler 애플리케이션 설정 파일(AppConfig)에 정의된 작업들을 Cron 스케줄에 맞춰 자동으로 실행하는 서비스입니다.
type Scheduler struct {
	taskConfigs []config.TaskConfig

	cron *cron.Cron

	// taskSubmitter 작업 실행을 요청하는 인터페이스입니다.
	taskSubmitter contract.TaskSubmitter

	// notificationSender 알림 전송을 담당하는 인터페이스입니다.
	notificationSender contract.NotificationSender

	running   bool
	runningMu sync.Mutex
}

// NewService 새로운 Scheduler 서비스 인스턴스를 생성합니다.
func NewService(taskConfigs []config.TaskConfig, submitter contract.TaskSubmitter, notificationSender contract.NotificationSender) *Scheduler {
	if submitter == nil {
		panic("TaskSubmitter는 필수입니다")
	}
	if notificationSender == nil {
		panic("NotificationSender는 필수입니다")
	}

	return &Scheduler{
		taskConfigs: taskConfigs,

		taskSubmitter: submitter,

		notificationSender: notificationSender,
	}
}

// Start 스케줄러를 시작하고 설정 파일에 정의된 작업들을 Cron 엔진에 등록합니다.
//
// 매개변수:
//   - serviceStopCtx: 서비스 종료 신호를 받기 위한 Context
//   - serviceStopWG: 서비스 종료 완료를 알리기 위한 WaitGroup
//
// 반환값:
//   - error: taskSubmitter 또는 notificationSender가 nil인 경우
func (s *Scheduler) Start(serviceStopCtx context.Context, serviceStopWG *sync.WaitGroup) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	applog.WithComponent(component).Info("서비스 시작 진입: Scheduler 서비스 초기화 프로세스를 시작합니다")

	if s.taskSubmitter == nil {
		serviceStopWG.Done()
		return ErrTaskSubmitterNotInitialized
	}
	if s.notificationSender == nil {
		serviceStopWG.Done()
		return ErrNotificationSenderNotInitialized
	}

	if s.running {
		serviceStopWG.Done()
		applog.WithComponent(component).Warn("Scheduler 서비스가 이미 실행 중입니다 (중복 호출)")
		return nil
	}

	// 1. Cron 엔진 초기화
	// - StandardParser: 초 단위 스케줄링 지원 (6개 필드: 초 분 시 일 월 요일)
	// - Recover: Panic 발생 시 복구하여 다른 작업에 영향을 주지 않음
	// - SkipIfStillRunning: 이전 실행이 끝나지 않았으면 다음 실행을 건너뜀
	s.cron = cron.New(
		cron.WithParser(cronx.StandardParser()),
		cron.WithLogger(cron.VerbosePrintfLogger(applog.StandardLogger())),
		cron.WithChain(
			cron.Recover(cron.VerbosePrintfLogger(applog.StandardLogger())),
			cron.SkipIfStillRunning(cron.VerbosePrintfLogger(applog.StandardLogger())),
		),
	)

	// 2. 작업 등록
	s.registerTasks(serviceStopCtx)

	// 3. 스케줄러 시작
	s.cron.Start()
	s.running = true

	// 등록된 스케줄 개수 로깅
	applog.WithComponentAndFields(component, applog.Fields{
		"registered_schedules": len(s.cron.Entries()),
		"total_defined_tasks":  len(s.taskConfigs),
	}).Info("서비스 시작 완료: Scheduler 서비스가 정상적으로 초기화되었습니다")

	// 4. 종료 신호 대기 (고루틴)
	// 서비스 생명주기 컨텍스트(serviceStopCtx)의 취소 이벤트를 비동기로 모니터링합니다.
	// 종료 시그널 수신 시 Stop() 메서드를 호출하여 리소스를 안전하게 해제하고 그 결과를 보장합니다.
	go func() {
		defer serviceStopWG.Done()

		<-serviceStopCtx.Done()

		s.Stop()
	}()

	return nil
}

// Stop 실행 중인 스케줄러를 안전하게 중지합니다.
func (s *Scheduler) Stop() {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if !s.running {
		return
	}

	applog.WithComponent(component).Info("종료 절차 진입: Scheduler 서비스 중지 시그널을 수신했습니다")

	// Cron 엔진 중지 및 실행 중인 작업 완료 대기
	if s.cron != nil {
		ctx := s.cron.Stop()
		<-ctx.Done()
	}

	s.cron = nil
	s.running = false

	applog.WithComponent(component).Info("Scheduler 서비스 종료 완료: 모든 리소스가 정리되었습니다")
}

// registerTasks 설정 파일에 정의된 모든 작업을 하나씩 살펴보며, "실행 가능(Runnable)" 플래그가 켜진 작업들만
// Cron 스케줄러에 등록합니다. 등록되지 않은 작업은 건너뛰므로, 필요에 따라 작업을 활성화/비활성화할 수 있습니다.
func (s *Scheduler) registerTasks(serviceStopCtx context.Context) {
	for _, t := range s.taskConfigs {
		for _, c := range t.Commands {
			if !c.Scheduler.Runnable {
				continue
			}

			// 클로저 캡처 문제 방지를 위해 로컬 변수에 재할당 (중요!)
			// Go의 클로저는 변수의 참조를 캡처하므로, 루프 변수를 직접 사용하면 모든 클로저가 마지막 값을 참조하게 됩니다.
			taskID := contract.TaskID(t.ID)
			commandID := contract.TaskCommandID(c.ID)
			defaultNotifierID := contract.NotifierID(c.DefaultNotifierID)
			timeSpec := c.Scheduler.TimeSpec

			// Cron 스케줄 등록
			_, err := s.cron.AddFunc(timeSpec, func() {
				// ========================================
				// 작업 실행 요청 (Task Submission)
				// ========================================
				//
				// [컨텍스트 설계 배경]
				//
				// 1. context.Background() 사용 이유:
				//    - 작업 실행 요청의 생명주기를 스케줄러 서비스의 종료 시그널(serviceStopCtx)과 분리합니다.
				//    - Graceful Shutdown 시 cron.Stop()이 실행 중인 모든 작업의 완료를 대기하므로,
				//      작업 도중 컨텍스트 취소로 인한 강제 중단을 방지하고 데이터 정합성을 보장합니다.
				//
				// 2. WithTimeout 적용 이유:
				//    - 작업 큐(Task Queue)가 가득 찼을 때 무한 대기(Hang)를 방지합니다.
				//    - taskSubmitTimeout(5초) 내에 큐에 자리가 나지 않으면 에러를 반환하여,
				//      스케줄러가 블로킹되지 않고 다음 스케줄을 정상적으로 처리할 수 있도록 합니다.
				ctx, cancel := context.WithTimeout(context.Background(), taskSubmitTimeout)
				defer cancel()

				if err := s.taskSubmitter.Submit(ctx, &contract.TaskSubmitRequest{
					TaskID:        taskID,
					CommandID:     commandID,
					NotifierID:    defaultNotifierID,
					NotifyOnStart: false,
					RunBy:         contract.TaskRunByScheduler,
				}); err != nil {
					message := "작업 요청 실패: TaskSubmitter 실행 중 오류가 발생했습니다"
					s.logAndNotifyError(serviceStopCtx, defaultNotifierID, taskID, commandID, message, err)
				}
			})

			if err != nil {
				// 스케줄 파싱 실패 시 해당 작업만 건너뛰고 계속 진행
				message := fmt.Sprintf("스케줄 등록 실패: 잘못된 Cron 표현식입니다 (TimeSpec: %s)", timeSpec)
				s.logAndNotifyError(serviceStopCtx, defaultNotifierID, taskID, commandID, message, err)
				continue
			}
		}
	}
}

// logAndNotifyError 스케줄러 실행 중 발생한 오류를 로깅하고 관리자에게 알림을 전송합니다.
func (s *Scheduler) logAndNotifyError(serviceStopCtx context.Context, notifierID contract.NotifierID, taskID contract.TaskID, commandID contract.TaskCommandID, message string, err error) {
	fields := applog.Fields{
		"notifier_id": notifierID,
		"task_id":     taskID,
		"command_id":  commandID,
		"run_by":      contract.TaskRunByScheduler,
	}
	if err != nil {
		fields["error"] = err

		// 에러 객체가 있으면 메시지에 상세 내용 추가
		message = fmt.Sprintf("%s: %v", message, err)
	}

	applog.WithComponentAndFields(component, fields).Error(message)

	s.notificationSender.Notify(serviceStopCtx, contract.Notification{
		NotifierID:    notifierID,
		TaskID:        taskID,
		CommandID:     commandID,
		Title:         "",
		Message:       message,
		ErrorOccurred: true,
	})
}
