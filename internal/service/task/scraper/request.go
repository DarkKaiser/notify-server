package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"reflect"
	"strings"
	"time"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"golang.org/x/net/html/charset"
)

// requestParams executeRequest 함수 실행에 필요한 파라미터들을 묶은 구조체입니다.
type requestParams struct {
	// Method HTTP 요청 메서드입니다. (예: "GET", "POST")
	Method string

	// URL 요청을 보낼 대상 URL입니다.
	URL string

	// Body 요청 본문 데이터입니다.
	// prepareBody를 통해 이미 전송 가능한 형태(io.Reader)로 변환된 상태여야 합니다.
	// GET 요청 등 본문이 없는 경우 nil일 수 있습니다.
	Body io.Reader

	// Header HTTP 요청 헤더입니다.
	// 호출자가 제공한 헤더를 Clone하여 사용하며, 필요시 Content-Type이나 Accept 등의 헤더가 추가될 수 있습니다.
	Header http.Header

	// DefaultAccept Accept 헤더가 설정되지 않았을 때 사용할 기본값입니다.
	// 예: JSON 요청의 경우 "application/json", HTML 요청의 경우 브라우저 호환 값
	DefaultAccept string

	// Validator 응답을 검증하는 함수입니다.
	// 기본 상태 코드 검사 및 에러 처리(checkResponse) 로직 내부에서 호출되며,
	// Content-Type 확인 등 요청 유형(HTML/JSON)에 특화된 추가 검증을 수행할 때 사용됩니다.
	Validator func(*http.Response, *applog.Entry) error
}

// @@@@@
// scrapedResponse HTTP 요청 실행 결과를 담는 내부 구조체입니다.
type scrapedResponse struct {
	Response    *http.Response
	Body        []byte
	IsTruncated bool
}

// @@@@@
// executeRequest HTTP 요청을 생성하고 실행하는 공통 로직입니다.
// 내부적으로 요청 생성(createAndSendRequest), 응답 검증(checkResponse), 본문 읽기(readAndLimitBody) 단계로 분리되어 있습니다.
func (s *scraper) executeRequest(ctx context.Context, params requestParams) (resp scrapedResponse, log *applog.Entry, err error) {
	start := time.Now()
	log = applog.WithComponent("scraper").WithContext(ctx).WithField("url", params.URL).WithField("method", params.Method)

	// 함수 종료 시 로깅 (성공/실패 무관하게 수행 시간 기록)
	defer func() {
		duration := time.Since(start).Milliseconds()
		log = log.WithField("duration_ms", duration)
		// 에러가 없고 성공한 경우에만 Debug 로그를 남길 수도 있으나,
		// 호출자(FetchHTML/FetchJSON)에서 성공 로그를 남기므로 여기서는 생략하거나
		// 필요 시 Trace/Debug 레벨로 남길 수 있음.
	}()

	// 1. 요청 생성 및 전송
	httpResp, err := s.createAndSendRequest(ctx, params)
	if err != nil {
		// createAndSendRequest에서는 에러만 반환하므로, 여기서 수행 시간을 포함하여 로깅합니다.
		log.WithField("duration_ms", time.Since(start).Milliseconds()).WithError(err).Error("[실패]: HTTP 요청 생성 또는 전송 에러")
		return scrapedResponse{}, log, err
	}

	// 2. 응답 검증 (Status Code, Content-Type 등)
	if err := s.checkResponse(httpResp, params, log); err != nil {
		// 검증 실패 시 Body를 닫아야 함
		_, _ = io.Copy(io.Discard, io.LimitReader(httpResp.Body, 4096))
		httpResp.Body.Close()
		log.WithField("duration_ms", time.Since(start).Milliseconds()).WithError(err).Error("[실패]: HTTP 응답 검증 부적합")
		return scrapedResponse{}, log, err
	}

	// 3. 응답 본문 읽기 및 버퍼링
	defer httpResp.Body.Close()

	bodyBytes, isTruncated, err := s.readAndLimitBody(httpResp)
	if err != nil {
		log.WithField("duration_ms", time.Since(start).Milliseconds()).WithError(err).Error("[실패]: 응답 본문 읽기 에러")
		return scrapedResponse{}, log, apperrors.Wrap(err, apperrors.ExecutionFailed, "응답 본문을 읽는 중 오류가 발생했습니다.")
	}

	if isTruncated {
		log.WithField("max_body_size", s.maxResponseBodySize).Warn("[경고]: 응답 본문 크기 초과, 데이터 잘림(Truncated)")
	}

	// 메모리 버퍼로 Body 교체
	httpResp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	resp = scrapedResponse{
		Response:    httpResp,
		Body:        bodyBytes,
		IsTruncated: isTruncated,
	}

	return resp, log, nil
}

// @@@@@
// prepareBody는 body 파라미터를 전송 가능한 리더로 준비합니다.
func (s *scraper) prepareBody(ctx context.Context, body any) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}

	// Typed Nil 체크 (인터페이스에 nil 포인터가 할당된 경우 방지)
	// 예: var b *bytes.Buffer = nil; createBodyReader(b) -> panic 방지
	val := reflect.ValueOf(body)
	if val.Kind() == reflect.Ptr && val.IsNil() {
		return nil, nil // 명시적으로 nil 반환
	}

	switch v := body.(type) {
	case io.Reader:
		// [Check] 크기를 알 수 있는 Reader(Buffer, strings.Reader 등)의 경우 미리 크기를 검사합니다.
		if v, ok := v.(interface{ Len() int }); ok {
			if int64(v.Len()) > s.maxRequestBodySize {
				return nil, apperrors.Wrap(ErrTooLarge, apperrors.ExecutionFailed, fmt.Sprintf("요청 본문이 허용된 크기(%d bytes)를 초과했습니다.", s.maxRequestBodySize))
			}
		}

		// [Optimization] 이미 GetBody 생성을 지원하는 타입들은 그대로 반환하여 불필요한 메모리 복사 방지
		switch v.(type) {
		case *bytes.Buffer, *bytes.Reader, *strings.Reader:
			return v, nil
		}

		// [Fix] 고루틴 누수 방지를 위한 동기적 읽기 처리
		// 기존에는 고루틴을 생성하여 읽었으나, Context 취소 시 고루틴이 종료되지 않는 문제가 있었습니다.
		// 동기적으로 읽되, 읽기 시작 전에 Context 취소 여부를 확인합니다.
		// (io.Reader의 Read 메서드 자체가 Context를 지원하지 않으므로, Read 진입 후 블로킹되면 취소할 수 없는 한계는 여전하지만,
		//  최소한 고루틴을 남기지 않으므로 리소스 누수는 방지됩니다.)
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// LimitReader를 사용하여 최대 크기 제한
		limitReader := io.LimitReader(v, s.maxRequestBodySize+1)

		// [Fix] Context 취소를 감지하기 위해 contextAwareReader로 래핑
		// io.ReadAll은 내부적으로 Read를 반복 호출하므로, 매 Read마다 Context를 확인하면
		// Slow Reader 공격이나 타임아웃 발생 시 중간에 멈출 수 있습니다.
		ctxOrLimitReader := &contextAwareReader{ctx: ctx, r: limitReader}

		data, err := io.ReadAll(ctxOrLimitReader)
		if err != nil {
			return nil, apperrors.Wrap(err, apperrors.Internal, "요청 본문 읽기 실패")
		}

		if int64(len(data)) > s.maxRequestBodySize {
			return nil, apperrors.Wrap(ErrTooLarge, apperrors.ExecutionFailed, fmt.Sprintf("요청 본문이 허용된 크기(%d bytes)를 초과했습니다.", s.maxRequestBodySize))
		}

		return bytes.NewReader(data), nil

	case string:
		if int64(len(v)) > s.maxRequestBodySize {
			return nil, apperrors.Wrap(ErrTooLarge, apperrors.ExecutionFailed, fmt.Sprintf("요청 본문이 허용된 크기(%d bytes)를 초과했습니다.", s.maxRequestBodySize))
		}
		return strings.NewReader(v), nil
	case []byte:
		if int64(len(v)) > s.maxRequestBodySize {
			return nil, apperrors.Wrap(ErrTooLarge, apperrors.ExecutionFailed, fmt.Sprintf("요청 본문이 허용된 크기(%d bytes)를 초과했습니다.", s.maxRequestBodySize))
		}
		return bytes.NewReader(v), nil
	default:
		// 메모리 버퍼링 방식: JSON 직렬화 후 전송
		// json.NewEncoder 대신 json.Marshal 사용 (불필요한 개행 문자 제거)
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return nil, apperrors.Wrap(err, apperrors.Internal, "JSON 요청 본문 인코딩 실패")
		}
		if int64(len(jsonBytes)) > s.maxRequestBodySize {
			return nil, apperrors.Wrap(ErrTooLarge, apperrors.ExecutionFailed, fmt.Sprintf("요청 본문이 허용된 크기(%d bytes)를 초과했습니다.", s.maxRequestBodySize))
		}
		return bytes.NewReader(jsonBytes), nil
	}
}

// @@@@@
// createAndSendRequest 요청을 생성하고 전송합니다.
// 에러 발생 시 로그는 호출자(executeRequest)에서 통합 처리하도록 에러만 반환합니다.
func (s *scraper) createAndSendRequest(ctx context.Context, params requestParams) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, params.Method, params.URL, params.Body)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.ExecutionFailed, fmt.Sprintf("HTTP 요청 생성에 실패했습니다. (URL: %s)", params.URL))
	}

	if params.Header != nil {
		req.Header = params.Header.Clone()
	}

	if req.Header.Get("Accept") == "" && params.DefaultAccept != "" {
		req.Header.Set("Accept", params.DefaultAccept)
	}

	httpResp, err := s.fetcher.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, apperrors.Wrap(ctx.Err(), apperrors.Unavailable, fmt.Sprintf("HTTP 요청이 취소되거나 타임아웃 되었습니다. (URL: %s)", params.URL))
		}
		// ErrNetwork로 래핑하지 않고 원본 에러를 유지하여 호출자가 에러 원인을 파악할 수 있게 함
		return nil, apperrors.Wrap(err, apperrors.Unavailable, fmt.Sprintf("HTTP 페이지(%s) 요청 중 네트워크 또는 클라이언트 에러가 발생했습니다", params.URL))
	}

	return httpResp, nil
}

// @@@@@
// checkResponse 응답 상태 코드 확인 및 에러 객체 생성, 사용자 정의 Validator 실행을 담당합니다.
func (s *scraper) checkResponse(resp *http.Response, params requestParams, log *applog.Entry) error {
	if s.responseCallback != nil {
		safeResp := *resp
		safeResp.Body = http.NoBody
		s.responseCallback(&safeResp)
	}

	// 204 No Content는 성공으로 간주
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	// 상태 코드 검사
	if err := fetcher.CheckResponseStatus(resp); err != nil {
		// [Improvement] 에러 발생 시 Body를 읽어 ResponseError에 포함
		// 최대 1KB(readErrorBody 내부 제한)까지만 읽어 메모리 부담을 줄임
		// 인코딩 변환을 통해 깨진 문자열 방지
		errorBodyStr, _ := s.readErrorBody(resp)
		bodyBytes := []byte(errorBodyStr)

		// ResponseError 생성
		respErr := &ResponseError{
			StatusCode: resp.StatusCode,
			Header:     resp.Header,
			Body:       bodyBytes,
			URL:        params.URL,
			Err:        err,
		}

		// 에러 타입 분류
		// 4xx: 클라이언트 에러 -> 재시도 불필요 (ExecutionFailed)
		// 단, 408(Request Timeout)과 429(Too Many Requests)는 일시적일 수 있으므로 재시도 대상(Unavailable)
		// 5xx 등 그 외: 서버 에러 -> 재시도 가능 (Unavailable)
		errType := apperrors.Unavailable
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			if resp.StatusCode != http.StatusRequestTimeout && resp.StatusCode != http.StatusTooManyRequests {
				errType = apperrors.ExecutionFailed
			}
		}

		return apperrors.Wrap(respErr, errType, respErr.Error())
	}

	// 사용자 정의 Validator (Content-Type 등)
	if params.Validator != nil {
		if err := params.Validator(resp, log); err != nil {
			// 검증 실패 시 내용을 확인할 수 있도록 Body의 앞부분을 읽어서 에러 메시지에 포함합니다.
			// 주의: 이미 CheckResponseStatus에서 에러가 없었으므로 Body는 살아있습니다.
			// 하지만 Validator 내에서 이미 Body를 읽었을 수도 있으니 유의해야 합니다.
			// 현재 구조상 Validator는 Header만 검사하므로 Body는 읽지 않은 상태입니다.

			// previewBody를 사용하여 안전하게 일부만 읽습니다.
			// 읽은 후에는 Body를 다시 읽을 수 없게 되므로, 어차피 에러 발생 시 Body는 버려질 것이라 괜찮습니다.
			// 다만 clean한 리소스 정리를 위해 호출자(executeRequest)에서 Close()가 호출됩니다.

			// 최대 1KB 정도만 읽어서 힌트로 제공
			const hintSize = 1024
			limitReader := io.LimitReader(resp.Body, hintSize)
			snippetBytes, _ := io.ReadAll(limitReader) // 에러 발생 시 무시하고 읽은 만큼만 사용
			snippet := s.previewBody(snippetBytes, resp.Header.Get("Content-Type"))

			if snippet != "" {
				return apperrors.Wrap(err, apperrors.ExecutionFailed, fmt.Sprintf("응답 검증 실패. Body Snippet: %s", snippet))
			}

			return err
		}
	}

	return nil
}

// @@@@@
// readAndLimitBody 응답 본문을 읽고 최대 크기를 제한합니다.
func (s *scraper) readAndLimitBody(resp *http.Response) ([]byte, bool, error) {
	// 204 No Content: 바디가 없으므로 즉시 반환 (할당 방지)
	if resp.StatusCode == http.StatusNoContent {
		return nil, false, nil
	}

	// [Optimization] io.ReadAll + LimitReader 조합 사용
	// bytes.Buffer를 직접 관리하는 것보다 Go 런타임의 최적화된 io.ReadAll을 사용하는 것이
	// 메모리 할당 패턴 측면에서 유리할 수 있습니다.
	// MaxBodySize + 1만큼 읽어서 Truncation 여부를 판단합니다.
	limit := s.maxResponseBodySize + 1
	limitReader := io.LimitReader(resp.Body, limit)

	bodyBytes, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, false, err
	}

	// Truncation 확인
	isTruncated := false
	if int64(len(bodyBytes)) > s.maxResponseBodySize {
		// 실제 사용할 데이터는 MaxBodySize만큼만 유지
		bodyBytes = bodyBytes[:s.maxResponseBodySize]
		isTruncated = true
	}

	return bodyBytes, isTruncated, nil
}

// @@@@@
// readErrorBody 응답 본문의 일부를 읽어 에러 메시지에 포함할 문자열을 반환합니다.
// Content-Type을 기반으로 인코딩을 감지하여 UTF-8로 변환합니다.
func (s *scraper) readErrorBody(resp *http.Response) (string, error) {
	const maxErrorBodySize = 1024

	// 제한된 리더 생성
	limitReader := io.LimitReader(resp.Body, maxErrorBodySize)

	// Charset 디코딩 리더 생성
	contentType := resp.Header.Get("Content-Type")
	utf8Reader, err := charset.NewReader(limitReader, contentType)
	if err != nil {
		// 변환 실패 시 원본 리더 사용 (Fallback)
		utf8Reader = limitReader
	}

	errorBody, err := io.ReadAll(utf8Reader)
	if err != nil {
		return "", err
	}

	// UTF-8 유효성 정제 및 제어 문자 처리 (previewBody 로직 일부 차용)
	// 로그에 남길 목적이므로 깔끔하게 정제합니다.
	cleanBody := strings.ToValidUTF8(string(errorBody), "")
	return cleanBody, nil
}

// @@@@@
// previewBody 로깅을 위해 본문의 앞부분을 잘라서 반환합니다.
// contentType을 기반으로 인코딩을 감지하여 UTF-8로 변환하고 안전하게 자릅니다.
func (s *scraper) previewBody(body []byte, contentType string) string {
	const maxPeekSize = 1024

	if len(body) == 0 {
		return ""
	}

	limit := len(body)
	if limit > maxPeekSize {
		limit = maxPeekSize
	}

	// 1. 우선 Max 크기만큼 자름
	data := body[:limit]

	// 2. 인코딩 변환 (필요한 경우)
	// charset.NewReader는 contentType을 보고 적절한 디코더를 선택합니다.
	// Content-Type이 비어있는 경우에도 HTML 태그 등을 통해 인코딩 감지를 시도합니다.
	if !isUTF8(contentType) {
		reader, err := charset.NewReader(bytes.NewReader(data), contentType)
		if err == nil {
			decoded, err := io.ReadAll(reader)
			if err == nil {
				data = decoded
			} else if len(decoded) > 0 {
				// 에러가 발생했더라도 디코딩된 부분까지만 사용 (Truncated Multi-byte handling)
				data = decoded
			}
		}
	}

	// 3. 변환 후 다시 크기 제한 (변환으로 커질 수 있음)
	if len(data) > maxPeekSize {
		data = data[:maxPeekSize]
	}

	// 4. UTF-8 유효성 정제
	// 잘린 멀티바이트 문자로 인해 끝부분이 깨질 수 있으므로 제거합니다.
	// strings.ToValidUTF8은 잘못된 시퀀스를 ReplacementChar(U+FFFD)로 바꾸거나 제거할 수 있습니다.
	// 여기서는 제거("")하여 깔끔한 로그를 유지합니다.
	preview := strings.ToValidUTF8(string(data), "")

	// 5. 바이너리 감지 (제어 문자 확인)
	// 텍스트로 보이지만 실제로는 바이너리인 경우 필터링
	isBinary := false
	for _, r := range preview {
		// 허용된 제어 문자(Tab, LF, CR) 외의 제어 문자가 있으면 바이너리로 간주
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			isBinary = true
			break
		}
	}

	if isBinary {
		return fmt.Sprintf("[Binary Data] (%d bytes)", len(body))
	}

	// Truncation 표시
	if len(body) > len(preview) {
		return preview + "...(truncated)"
	}

	return preview
}

// @@@@@
// isUTF8 Content-Type이 명시적으로 UTF-8인지 확인하는 헬퍼
// 비어있는 경우 false를 반환하여 감지 로직을 수행하도록 합니다.
func isUTF8(contentType string) bool {
	lowerType := strings.ToLower(contentType)
	return strings.Contains(lowerType, "utf-8")
}

// @@@@@
// isHTMLContentType은 주어진 Content-Type 문자열이 HTML 형식인지 판단합니다.
//
// 이 함수는 두 단계의 검증 전략을 사용합니다:
//  1. 표준 파싱: mime.ParseMediaType을 사용하여 정확한 미디어 타입을 추출합니다.
//  2. Fallback 파싱: 표준 파싱 실패 시, 문자열 접두사 매칭으로 레거시/비표준 헤더를 처리합니다.
//
// 이러한 이중 전략은 실무에서 발생하는 다양한 비표준 Content-Type 헤더
// (예: 중복 세미콜론, 잘못된 공백 등)를 관대하게 처리하기 위함입니다.
//
// 매개변수:
//   - contentType: 검증할 Content-Type 헤더 값 (예: "text/html; charset=utf-8")
//
// 반환값:
//   - bool: HTML 타입이면 true, 그렇지 않으면 false
//
// 인식하는 HTML 타입:
//   - text/html: 표준 HTML 문서
//   - application/xhtml+xml: XHTML 문서
func isHTMLContentType(contentType string) bool {
	// 1. 표준 파싱 시도
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		return mediaType == "text/html" || mediaType == "application/xhtml+xml"
	}

	// 2. 파싱 실패 시, Fallback으로 문자열 접두사 확인 (레거시/비표준 헤더 대응)
	// 많은 레거시/비표준 서버들이 표준을 따르지 않는 헤더를 보냄 (예: 중복 세미콜론 등)
	lowerType := strings.ToLower(strings.TrimSpace(contentType))

	// 단순 접두사 확인 전에 세미콜론으로 분리하여 첫 번째 부분만 확인하는 것이 더 안전할 수 있으나,
	// 파싱 실패한 케이스이므로 최대한 관대하게 처리
	return strings.HasPrefix(lowerType, "text/html") || strings.HasPrefix(lowerType, "application/xhtml+xml")
}
