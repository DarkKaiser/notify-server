package router

import (
	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/service/api/handler"
	_middleware_ "github.com/darkkaiser/notify-server/service/api/middleware"
	"github.com/darkkaiser/notify-server/service/notification"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func New(config *g.AppConfig, notificationSender notification.NotificationSender) *echo.Echo {
	e := echo.New()

	e.Debug = true
	e.HideBanner = true

	// echo에서 출력되는 로그를 Logrus Logger로 출력되도록 한다.
	// echo Logger의 인터페이스를 래핑한 객체를 이용하여 Logrus Logger로 보낸다.
	e.Logger = _middleware_.Logger{Logger: log.StandardLogger()}
	e.Use(_middleware_.LogrusLogger())
	// echo 기본 로그출력 구문, 필요치 않음!!!
	//e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
	//	Format: `time="${time_rfc3339}" level=${level} remote_ip="${remote_ip}" host="${host}" method="${method}" uri="${uri}" user_agent="${user_agent}" ` +
	//		`status=${status} error="${error}" latency=${latency} latency_human="${latency_human}" bytes_in=${bytes_in} bytes_out=${bytes_out}` + "\n",
	//}))

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{ // CORS Middleware
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))
	e.Use(middleware.Recover()) // Recover from panics anywhere in the chain
	e.Use(middleware.Secure())

	grp := e.Group("/api/notify")
	{
		grp.Use(middleware.KeyAuthWithConfig(middleware.KeyAuthConfig{
			KeyLookup:  "header:Authorization",
			AuthScheme: "Bearer",
			Validator: func(key string, c echo.Context) (bool, error) {
				return key == config.NotifyAPI.APIKey, nil
			},
		}))

		h := handler.NewNotifyAPIHandlers(config, notificationSender)

		grp.POST("/message/send", h.NotifyMessageSendHandler)
	}

	return e
}