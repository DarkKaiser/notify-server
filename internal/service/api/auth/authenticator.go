package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/httputil"
	"github.com/darkkaiser/notify-server/internal/service/api/model/domain"
	applog "github.com/darkkaiser/notify-server/pkg/log"
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
// 보안:
//   - App Key는 SHA-256 해시로 저장되어 메모리 덤프 공격을 방어합니다.
//   - Constant-Time 비교를 사용하여 타이밍 공격을 방어합니다.
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
	mu           sync.RWMutex                   // 동시성 제어
	applications map[string]*domain.Application // 애플리케이션 정보
	appKeyHashes map[string]string              // App Key SHA-256 해시 (보안)
}

// NewAuthenticator 설정 파일에서 애플리케이션 정보를 로드하여 인증자를 생성합니다.
//
// 이 함수는 설정된 모든 애플리케이션의 ID, 제목, 설명, 기본 Notifier ID를 메모리에 로드하고,
// App Key는 SHA-256 해시로 변환하여 별도로 저장합니다.
//
// 보안:
//   - App Key는 SHA-256으로 해시되어 저장됩니다.
//   - 원본 App Key는 메모리에 저장되지 않아 메모리 덤프 공격을 방어합니다.
func NewAuthenticator(appConfig *config.AppConfig) *Authenticator {
	applications := make(map[string]*domain.Application)
	appKeyHashes := make(map[string]string)

	for _, application := range appConfig.NotifyAPI.Applications {
		applications[application.ID] = &domain.Application{
			ID:                application.ID,
			Title:             application.Title,
			Description:       application.Description,
			DefaultNotifierID: application.DefaultNotifierID,
		}

		// App Key를 SHA-256으로 해시하여 저장 (보안)
		hash := sha256.Sum256([]byte(application.AppKey))
		appKeyHashes[application.ID] = hex.EncodeToString(hash[:])
	}

	return &Authenticator{
		applications: applications,
		appKeyHashes: appKeyHashes,
	}
}

// Authenticate 애플리케이션 ID와 App Key를 검증하여 인증을 수행합니다.
//
// 인증 과정:
//  1. Application ID로 등록된 애플리케이션 조회
//  2. 입력받은 App Key를 SHA-256으로 해시 변환
//  3. 저장된 해시와 Constant-Time 비교
//
// 반환값:
//   - 성공: 인증된 Application 객체
//   - 실패: 401 Unauthorized 에러 (ID 없음 또는 Key 불일치)
//
// 보안:
//   - Constant-Time 비교를 사용하여 타이밍 공격을 방어합니다.
//   - 입력받은 App Key를 SHA-256으로 해시하여 저장된 해시와 비교합니다.
//
// 이 메서드는 동시성 안전하며, 여러 고루틴에서 동시에 호출 가능합니다.
func (a *Authenticator) Authenticate(applicationID, appKey string) (*domain.Application, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	app, ok := a.applications[applicationID]
	if !ok {
		return nil, httputil.NewUnauthorizedError(fmt.Sprintf(constants.ErrMsgUnauthorizedNotFoundApplicationID, applicationID))
	}

	// 입력받은 App Key를 SHA-256으로 해시
	inputHash := sha256.Sum256([]byte(appKey))
	inputHashStr := hex.EncodeToString(inputHash[:])

	// Constant-Time 비교 (타이밍 공격 방어)
	storedHash := a.appKeyHashes[applicationID]
	if subtle.ConstantTimeCompare([]byte(storedHash), []byte(inputHashStr)) != 1 {
		applog.WithComponentAndFields(constants.ComponentHandler, applog.Fields{
			"application_id": applicationID,
			"app_title":      app.Title,
		}).Warn("인증 실패: App Key 불일치")

		return nil, httputil.NewUnauthorizedError(fmt.Sprintf(constants.ErrMsgUnauthorizedInvalidAppKey, applicationID))
	}

	return app, nil
}
