package response

// SuccessResponse API 요청이 성공적으로 처리되었을 때 반환되는 표준 응답 모델입니다.
type SuccessResponse struct {
	// 처리 결과 코드입니다.
	// 0은 성공을 의미하며, 그 외의 값은 정의된 에러 코드를 나타냅니다.
	ResultCode int `json:"result_code" example:"0"`
}
