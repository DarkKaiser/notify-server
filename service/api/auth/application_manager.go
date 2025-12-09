package auth

import (
	"fmt"

	"github.com/darkkaiser/notify-server/config"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutils"
	"github.com/darkkaiser/notify-server/service/api/handler"
	"github.com/darkkaiser/notify-server/service/api/model/domain"
	log "github.com/sirupsen/logrus"
)

// ApplicationManager 애플리케이션 로딩 및 인증을 담당하는 매니저입니다.
//
// 이 구조체는 다음과 같은 역할을 수행합니다:
//   - 설정 파일에서 등록된 애플리케이션 정보를 메모리에 로드
//   - Application ID와 App Key를 통한 인증 처리
//   - 인증 실패 시 적절한 HTTP 에러 반환
//
// ApplicationManager는 API 버전(v1, v2 등)과 무관하게 모든 핸들러에서 재사용 가능하며,
// 애플리케이션 인증이 필요한 모든 엔드포인트에서 공통으로 사용됩니다.
//
// 사용 예시:
//
//	manager := auth.NewApplicationManager(appConfig)
//	app, err := manager.Authenticate(applicationID, appKey)
//	if err != nil {
//	    return err // 401 Unauthorized
//	}
//	// app 사용
type ApplicationManager struct {
	applications map[string]*domain.Application
}

// NewApplicationManager 설정에서 애플리케이션을 로드하여 ApplicationManager를 생성합니다.
func NewApplicationManager(appConfig *config.AppConfig) *ApplicationManager {
	applications := make(map[string]*domain.Application)
	for _, application := range appConfig.NotifyAPI.Applications {
		applications[application.ID] = &domain.Application{
			ID:                application.ID,
			Title:             application.Title,
			Description:       application.Description,
			DefaultNotifierID: application.DefaultNotifierID,
			AppKey:            application.AppKey,
		}
	}

	return &ApplicationManager{
		applications: applications,
	}
}

// Authenticate 애플리케이션을 찾고 인증을 수행합니다.
// 성공 시 Application 객체를 반환하고, 실패 시 적절한 HTTP 에러를 반환합니다.
func (m *ApplicationManager) Authenticate(applicationID, appKey string) (*domain.Application, error) {
	app, ok := m.applications[applicationID]
	if !ok {
		return nil, handler.NewUnauthorizedError(fmt.Sprintf("접근이 허용되지 않은 application_id(%s)입니다", applicationID))
	}

	if app.AppKey != appKey {
		applog.WithComponentAndFields("api.handler", log.Fields{
			"application_id":   applicationID,
			"received_app_key": strutils.MaskSensitiveData(appKey),
		}).Warn("APP_KEY 불일치")

		return nil, handler.NewUnauthorizedError(fmt.Sprintf("app_key가 유효하지 않습니다.(application_id:%s)", applicationID))
	}

	return app, nil
}
