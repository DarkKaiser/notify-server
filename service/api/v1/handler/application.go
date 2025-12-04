package handler

// Application API 접근이 허용된 애플리케이션 정보를 담고 있습니다.
type Application struct {
	ID                string
	Title             string
	Description       string
	DefaultNotifierID string
	AppKey            string
}
