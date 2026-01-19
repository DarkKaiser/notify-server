package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

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
//   - 413 Request Entity Too Large: 요청 크기가 제한을 초과함 (BodyLimit)
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
		panic(constants.PanicMsgAuthenticatorRequired)
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 1. App Key 추출
			appKey := extractAppKey(c)
			if appKey == "" {
				return httputil.NewBadRequestError(constants.ErrMsgAuthAppKeyRequired)
			}

			// 2. Application ID 추출
			applicationID, err := extractApplicationID(c)
			if err != nil {
				return err
			}
			if applicationID == "" {
				return httputil.NewBadRequestError(constants.ErrMsgAuthApplicationIDRequired)
			}

			// 3. 인증 처리
			app, err := authenticator.Authenticate(applicationID, appKey)
			if err != nil {
				return err
			}

			// 4. Context에 인증된 Application 저장
			auth.SetApplication(c, app)

			// 5. 다음 핸들러로 전달
			return next(c)
		}
	}
}

// extractAppKey App Key를 추출합니다.
//
// 우선순위:
//  1. X-App-Key 헤더 (권장)
//  2. app_key 쿼리 파라미터 (레거시) - 사용 시 경고 로그 출력
func extractAppKey(c echo.Context) string {
	appKey := c.Request().Header.Get(constants.XAppKey)
	if appKey == "" {
		appKey = c.QueryParam(constants.AppKeyQuery)

		// 레거시 방식 사용 시 경고 로그
		if appKey != "" {
			applog.WithComponentAndFields(constants.MiddlewareAuth, applog.Fields{
				"method":    c.Request().Method,
				"path":      c.Path(),
				"remote_ip": c.RealIP(),
			}).Warn(constants.LogMsgAuthAppKeyInQuery)
		}
	}
	return appKey
}

// extractApplicationID Application ID를 추출합니다.
//
// 우선순위:
//  1. X-Application-Id 헤더 (권장)
//  2. Request Body (레거시, 호환성 유지) - Body 파싱 및 복원 비용 발생
func extractApplicationID(c echo.Context) (string, error) {
	// 우선순위 1: X-Application-Id 헤더
	applicationID := c.Request().Header.Get(constants.XApplicationID)
	if applicationID != "" {
		return applicationID, nil
	}

	// 우선순위 2: Request Body (레거시, 호환성 유지)
	// 헤더가 없는 경우에만 불가피하게 Body를 파싱합니다.
	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		// 413 Request Entity Too Large 처리
		//
		// BodyLimit 미들웨어 또는 http.MaxBytesReader에 의해 요청 본문 크기가 제한된 경우,
		// 읽기 시도 시 아래 두 가지 유형의 에러가 발생할 수 있습니다.
		// 1. http.MaxBytesError: 표준 라이브러리의 MaxBytesReader가 반환하는 에러
		// 2. echo.HTTPError(413): Echo 프레임워크가 래핑하여 반환하는 에러
		//
		// 이들을 포착하여 클라이언트에게 명확한 표준 413 에러 응답으로 정규화합니다.
		if _, ok := err.(*http.MaxBytesError); ok {
			return "", echo.NewHTTPError(http.StatusRequestEntityTooLarge, constants.ErrMsgRequestEntityTooLarge)
		}
		if he, ok := err.(*echo.HTTPError); ok && he.Code == http.StatusRequestEntityTooLarge {
			return "", echo.NewHTTPError(http.StatusRequestEntityTooLarge, constants.ErrMsgRequestEntityTooLarge)
		}
		return "", httputil.NewBadRequestError(constants.ErrMsgBadRequestBodyReadFailed)
	}
	c.Request().Body.Close()

	if len(bodyBytes) == 0 {
		return "", httputil.NewBadRequestError(constants.ErrMsgBadRequestEmptyBody)
	}

	// Body 복원 (다음 핸들러에서 사용 가능하도록)
	c.Request().Body = io.NopCloser(bytes.NewReader(bodyBytes))

	var authRequest struct {
		ApplicationID string `json:"application_id"`
	}
	if err := json.Unmarshal(bodyBytes, &authRequest); err != nil {
		return "", httputil.NewBadRequestError(constants.ErrMsgBadRequestInvalidJSON)
	}

	return authRequest.ApplicationID, nil
}
