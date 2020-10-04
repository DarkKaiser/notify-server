package router

import (
	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/service/api/handlers"
	"github.com/darkkaiser/notify-server/service/notification"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"net/http"
)

func New(config *g.AppConfig, notificationSender notification.NotificationSender) *echo.Echo {
	e := echo.New()

	e.Debug = true
	e.HideBanner = true

	// echo middleware func
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{ // Setting logger
		Format: `time="${time_rfc3339}" remote_ip="${remote_ip}" host="${host}" method="${method}" uri="${uri}" user_agent="${user_agent}" ` +
			`status=${status} error="${error}" latency=${latency} latency_human="${latency_human}" bytes_in=${bytes_in} bytes_out=${bytes_out}` + "\n",
	}))
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{ // CORS Middleware
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))
	e.Use(middleware.Recover()) // Recover from panics anywhere in the chain

	// @@@@@
	//////////////////
	// Initialize handler
	h := handlers.NewNotifyHandler(config, notificationSender)

	e.GET("/api/:message", h.MessageHandler)

	// Router List
	//getList := e.Group("/get")
	//{
	//		getList.GET("[path]", handler.[요청함수])
	//		getList.GET("[path][:pathParameter]", handler.[요청함수])
	//}

	//admin := e.Group("/admin")
	//{
	//	admin.GET("[path]", handler.[요청함수])
	//	admin.GET("[path]", handler.[요청함수], auth.[로그인체크함수], auth.[어드민체크함수])
	//}
	//
	//'''
	//[중략]
	//'''
	//
	//login := e.Group("/login")
	//{
	//	login.POST("", auth.auth.[로그인함수])
	//}
	//////////////////

	return e
}
