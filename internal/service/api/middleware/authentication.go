package middleware

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/darkkaiser/notify-server/internal/service/api/auth"
	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/httputil"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
)

// RequireAuthentication 애플리케이션 인증을 수행하는 미들웨어를 반환합니다.
//
// 처리 과정:
//  1. App Key 추출 (X-App-Key 헤더 우선, app_key 쿼리 파라미터 폴백)
//  2. Application ID 추출 (X-Application-Id 헤더 우선, Body 폴백)
//  3. Authenticator를 통한 인증 처리
//  4. 인증된 Application 객체를 Context에 저장
//
// App Key 추출 우선순위:
//  1. X-App-Key 헤더 (권장)
//  2. app_key 쿼리 파라미터 (레거시, deprecated)
//
// Application ID 추출 우선순위:
//  1. X-Application-Id 헤더 (권장)
//     - HTTP Body는 스트림(Stream)이므로, 미들웨어에서 읽으면 소모되어 사라집니다.
//     - 이를 복구하려면 전체 데이터를 메모리에 복사하고 다시 채워넣는 고비용 작업(I/O + 메모리 할당)이 필요합니다.
//     - 헤더를 사용하면 이러한 "Double Parsing"과 "Stream Restoration" 비용을 "0"으로 만들 수 있습니다.
//  2. Request Body (레거시, 호환성 유지)
//     - 헤더가 없는 경우에만 불가피하게 Body를 파싱합니다.
//
// 인증 성공 시:
//   - Application 객체를 Context에 저장 (키: ContextKeyApplication)
//   - 다음 핸들러로 제어 전달
//
// 인증 실패 시:
//   - 400 Bad Request: App Key/Application ID 누락, 빈 Body, 잘못된 JSON
//   - 401 Unauthorized: 미등록 Application ID 또는 잘못된 App Key
//
// 사용 예시:
//
//	authMiddleware := middleware.RequireAuthentication(authenticator)
//	e.POST("/api/v1/notifications", handler, authMiddleware)
//
// Panics:
//   - authenticator가 nil인 경우
func RequireAuthentication(authenticator *auth.Authenticator) echo.MiddlewareFunc {
	if authenticator == nil {
		panic("RequireAuthentication: Authenticator가 nil입니다")
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 1. App Key 추출 (헤더 우선, 쿼리 파라미터 폴백)
			appKey := c.Request().Header.Get(constants.HeaderXAppKey)
			if appKey == "" {
				appKey = c.QueryParam(constants.QueryParamAppKey)

				// 레거시 방식 사용 시 경고 로그
				if appKey != "" {
					applog.WithComponent(constants.ComponentMiddleware).Warn("레거시 방식으로 App Key 전달됨 (쿼리 파라미터)")
				}
			}

			if appKey == "" {
				return httputil.NewBadRequestError(constants.ErrMsgAppKeyRequired)
			}

			// 2. Application ID 추출
			// 우선순위 1: X-Application-Id 헤더 (권장, 성능 최적화, GET 지원)
			// 우선순위 2: Request Body (레거시, 호환성 유지)
			applicationID := c.Request().Header.Get(constants.HeaderXApplicationID)

			if applicationID == "" {
				// 헤더가 없으면 Body 파싱 (레거시 호환)
				bodyBytes, err := io.ReadAll(c.Request().Body)
				if err != nil {
					return httputil.NewBadRequestError(constants.ErrMsgBodyReadFailed)
				}
				c.Request().Body.Close()

				if len(bodyBytes) == 0 {
					return httputil.NewBadRequestError(constants.ErrMsgEmptyBody)
				}

				// Body 복원 (다음 핸들러에서 사용 가능하도록)
				c.Request().Body = io.NopCloser(bytes.NewReader(bodyBytes))

				var authRequest struct {
					ApplicationID string `json:"application_id"`
				}
				if err := json.Unmarshal(bodyBytes, &authRequest); err != nil {
					return httputil.NewBadRequestError(constants.ErrMsgInvalidJSON)
				}

				applicationID = authRequest.ApplicationID
			}

			if applicationID == "" {
				return httputil.NewBadRequestError(constants.ErrMsgApplicationIDRequired)
			}

			// 3. 인증 처리
			app, err := authenticator.Authenticate(applicationID, appKey)
			if err != nil {
				return err
			}

			// 5. Context에 인증된 Application 저장
			c.Set(constants.ContextKeyApplication, app)

			// 6. 다음 핸들러로 전달
			return next(c)
		}
	}
}
