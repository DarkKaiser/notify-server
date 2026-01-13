package auth

import (
	"errors"
	"fmt"

	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/model/domain"
	"github.com/labstack/echo/v4"
)

// SetApplication 인증된 애플리케이션 정보를 Context에 저장합니다.
func SetApplication(c echo.Context, app *domain.Application) {
	c.Set(constants.ContextKeyApplication, app)
}

// GetApplication Context에서 애플리케이션 정보를 조회합니다.
func GetApplication(c echo.Context) (*domain.Application, error) {
	val := c.Get(constants.ContextKeyApplication)
	if val == nil {
		return nil, errors.New(constants.ErrMsgAuthApplicationMissingInContext)
	}

	app, ok := val.(*domain.Application)
	if !ok {
		return nil, errors.New(constants.ErrMsgAuthApplicationTypeMismatch)
	}

	return app, nil
}

// MustGetApplication Context에서 애플리케이션 정보를 조회합니다.
// 인증 미들웨어를 통과하여 애플리케이션 정보가 반드시 존재한다고 보장될 때 사용합니다.
// 조회에 실패하면 panic이 발생하므로 주의해서 사용해야 합니다.
func MustGetApplication(c echo.Context) *domain.Application {
	app, err := GetApplication(c)
	if err != nil {
		panic(fmt.Sprintf(constants.PanicMsgAuthContextApplicationNotFound, err))
	}
	return app
}
