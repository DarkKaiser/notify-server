package middleware

import (
	"fmt"
	"runtime"

	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

func LogrusRecover() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if r := recover(); r != nil {
					err, ok := r.(error)
					if !ok {
						err = fmt.Errorf("%v", r)
					}

					stack := make([]byte, 4<<10) // 4KB
					length := runtime.Stack(stack, false)

					log.WithFields(log.Fields{
						"component":  "api.middleware",
						"error":      err,
						"stack":      string(stack[:length]),
						"request_id": c.Response().Header().Get(echo.HeaderXRequestID),
					}).Error("PANIC RECOVERED")

					c.Error(err)
				}
			}()
			return next(c)
		}
	}
}
