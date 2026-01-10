package auth

import (
	"fmt"
	"sync"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/httputil"
	"github.com/darkkaiser/notify-server/internal/service/api/model/domain"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutil"
)

// Authenticator 애플리케이션 인증을 담당하는 인증자입니다.
//
// 이 구조체는 다음과 같은 역할을 수행합니다:
//   - 설정 파일에서 등록된 애플리케이션 정보를 메모리에 로드
//   - Application ID와 App Key를 통한 인증 처리
//   - 인증 실패 시 적절한 HTTP 에러 반환
//
// Authenticator는 API 버전(v1, v2 등)과 무관하게 모든 핸들러에서 재사용 가능하며,
// 애플리케이션 인증이 필요한 모든 엔드포인트에서 공통으로 사용됩니다.
//
// 동시성 안전성:
//   - sync.RWMutex를 사용하여 동시성 안전을 보장합니다.
//   - 여러 고루틴에서 동시에 Authenticate를 호출해도 안전합니다.
//   - 현재는 초기화 후 읽기 전용이지만, 향후 동적 추가/삭제 기능 확장 가능합니다.
//
// 사용 예시:
//
//	authenticator := auth.NewAuthenticator(appConfig)
//	app, err := authenticator.Authenticate(applicationID, appKey)
//	if err != nil {
//	    return err // 401 Unauthorized
//	}
//	// app 사용
type Authenticator struct {
	mu           sync.RWMutex
	applications map[string]*domain.Application
}

// NewAuthenticator 설정에서 애플리케이션을 로드하여 Authenticator를 생성합니다.
func NewAuthenticator(appConfig *config.AppConfig) *Authenticator {
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

	return &Authenticator{
		applications: applications,
	}
}

// Authenticate 애플리케이션을 찾고 인증을 수행합니다.
// 성공 시 Application 객체를 반환하고, 실패 시 적절한 HTTP 에러를 반환합니다.
//
// 이 메서드는 동시성 안전하며, 여러 고루틴에서 동시에 호출 가능합니다.
func (a *Authenticator) Authenticate(applicationID, appKey string) (*domain.Application, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	app, ok := a.applications[applicationID]
	if !ok {
		return nil, httputil.NewUnauthorizedError(fmt.Sprintf("접근이 허용되지 않은 application_id(%s)입니다", applicationID))
	}

	if app.AppKey != appKey {
		applog.WithComponentAndFields(constants.ComponentHandler, applog.Fields{
			"application_id":   applicationID,
			"received_app_key": strutil.Mask(appKey),
		}).Warn("APP_KEY 불일치")

		return nil, httputil.NewUnauthorizedError(fmt.Sprintf("app_key가 유효하지 않습니다.(application_id:%s)", applicationID))
	}

	return app, nil
}
