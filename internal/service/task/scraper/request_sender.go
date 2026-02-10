package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
)

// prepareBody HTTP 요청 본문 데이터를 전송 가능한 io.Reader로 변환합니다.
//
// 이 함수는 다양한 타입의 요청 본문(string, []byte, io.Reader, 구조체 등)을 받아서
// HTTP 클라이언트가 전송할 수 있는 형태로 준비하며, 다음과 같은 처리를 수행합니다:
//   - 크기 검증: maxRequestBodySize를 초과하는 본문은 거부
//   - 메모리 버퍼링: 스트림 형태의 Reader를 메모리로 읽어들여 재사용 가능하게 변환
//   - JSON 직렬화: 구조체/맵 등 임의의 타입을 JSON으로 변환
//   - Context 지원: 읽기 작업 중 Context 취소 감지
//
// 매개변수:
//   - ctx: 요청의 생명주기를 제어하는 컨텍스트 (취소, 타임아웃 등)
//   - body: 요청 본문 데이터 (nil, string, []byte, io.Reader, 또는 JSON 직렬화 가능한 타입)
//
// 반환값:
//   - io.Reader: 전송 가능한 형태로 변환된 요청 본문 (nil 가능)
//   - error: 크기 초과, 읽기 실패, JSON 인코딩 실패 등의 에러
//
// 지원하는 body 타입:
//   - nil: 요청 본문 없음 (GET 요청 등)
//   - string: 문자열 데이터
//   - []byte: 바이너리 데이터
//   - io.Reader: 스트림 데이터 (메모리로 버퍼링됨)
//   - 기타 타입: JSON으로 직렬화하여 전송
func (s *scraper) prepareBody(ctx context.Context, body any) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}

	// Typed Nil 체크: 인터페이스 내부의 실제 포인터 값이 nil인지 확인하여 패닉을 방지합니다.
	rv := reflect.ValueOf(body)
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		return nil, nil
	}

	switch v := body.(type) {
	case io.Reader:
		// 요청 본문의 크기를 미리 알 수 있는 Reader의 경우 조기 검증
		if v, ok := v.(interface{ Len() int }); ok {
			if int64(v.Len()) > s.maxRequestBodySize {
				return nil, newErrRequestBodyTooLarge(s.maxRequestBodySize)
			}
		}

		// 재사용 가능한 Reader 타입은 그대로 반환
		//
		// bytes.Buffer, bytes.Reader, strings.Reader는 이미 메모리에 있는 데이터를 읽는 타입이므로,
		// http.Request.GetBody 함수 생성 시 추가 메모리 복사 없이 재사용할 수 있습니다.
		// (Fetcher가 재시도 시 동일한 본문을 다시 읽을 수 있도록 GetBody를 자동 생성함)
		//
		// 이러한 타입들은 이미 최적화된 형태이므로 그대로 반환하여 불필요한 io.ReadAll을 방지합니다.
		switch v.(type) {
		case *bytes.Buffer, *bytes.Reader, *strings.Reader:
			return v, nil
		}

		// 컨텍스트 취소 확인
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// 크기 제한을 위한 LimitReader 생성
		//
		// maxRequestBodySize+1 만큼 읽도록 제한하여:
		//   - maxRequestBodySize 이하인 경우: 전체 데이터를 읽음
		//   - maxRequestBodySize를 초과하는 경우: maxRequestBodySize+1 바이트를 읽어서 초과 여부를 감지
		//
		// 이를 통해 메모리 고갈 공격(DoS)을 방지하면서도 정확한 크기 검증이 가능합니다.
		limitReader := io.LimitReader(v, s.maxRequestBodySize+1)

		// 컨텍스트 취소 감지를 위한 Reader 래핑
		reader := &contextAwareReader{ctx: ctx, r: limitReader}

		// 전체 데이터를 메모리로 읽어들입니다.
		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, newErrReadRequestBody(err)
		}

		// 요청 본문의 크기 초과 여부 최종 검증
		// LimitReader가 maxRequestBodySize+1 만큼 읽었으므로, 실제로 maxRequestBodySize를 초과했는지 확인합니다.
		if int64(len(data)) > s.maxRequestBodySize {
			return nil, newErrRequestBodyTooLarge(s.maxRequestBodySize)
		}

		return bytes.NewReader(data), nil

	case string:
		// 문자열 타입: 요청 본문 크기 검증 후 strings.Reader로 변환
		if int64(len(v)) > s.maxRequestBodySize {
			return nil, newErrRequestBodyTooLarge(s.maxRequestBodySize)
		}

		return strings.NewReader(v), nil

	case []byte:
		// 바이트 슬라이스 타입: 요청 본문 크기 검증 후 bytes.Reader로 변환
		if int64(len(v)) > s.maxRequestBodySize {
			return nil, newErrRequestBodyTooLarge(s.maxRequestBodySize)
		}

		return bytes.NewReader(v), nil

	default:
		// 기타 타입: JSON으로 직렬화하여 전송
		data, err := json.Marshal(body)
		if err != nil {
			return nil, newErrEncodeJSONBody(err)
		}

		// 요청 본문 크기 검증
		if int64(len(data)) > s.maxRequestBodySize {
			return nil, newErrRequestBodyTooLarge(s.maxRequestBodySize)
		}

		return bytes.NewReader(data), nil
	}
}

// createAndSendRequest HTTP 요청 객체를 생성하고 네트워크를 통해 전송합니다.
//
// 매개변수:
//   - ctx: 요청의 생명주기를 제어하는 컨텍스트 (취소, 타임아웃 등)
//   - params: 요청 객체 생성에 필요한 파라미터 (Method, URL, Body, Header, DefaultAccept, Validator)
//
// 반환값:
//   - *http.Response: 성공 시 HTTP 응답 객체 (Body는 아직 읽지 않은 상태)
//   - error: 요청 객체 생성 실패, 컨텍스트 취소, 네트워크 오류 등
func (s *scraper) createAndSendRequest(ctx context.Context, params requestParams) (*http.Response, error) {
	// [1단계] HTTP 요청 객체 생성
	req, err := http.NewRequestWithContext(ctx, params.Method, params.URL, params.Body)
	if err != nil {
		return nil, newErrCreateHTTPRequest(params.URL, err)
	}

	// [2단계] 요청 헤더 설정
	if params.Header != nil {
		req.Header = params.Header.Clone()
	}

	// [3단계] Accept 헤더 기본값 설정
	if req.Header.Get("Accept") == "" && params.DefaultAccept != "" {
		req.Header.Set("Accept", params.DefaultAccept)
	}

	// [4단계] HTTP 요청 전송
	httpResp, err := s.fetcher.Do(req)
	if err != nil {
		// 컨텍스트 취소/타임아웃 여부 확인
		if ctx.Err() != nil {
			return nil, newErrHTTPRequestCanceled(params.URL, ctx.Err())
		}

		// 네트워크 에러 처리
		return nil, newErrNetworkError(params.URL, err)
	}

	return httpResp, nil
}
