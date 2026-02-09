package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"golang.org/x/net/html/charset"
)

// FetchJSON 지정된 URL로 HTTP 요청을 보내 JSON 응답을 가져오고, 지정된 구조체로 디코딩합니다.
//
// 이 함수는 RESTful API 호출에 최적화되어 있으며, 다음과 같은 주요 기능을 제공합니다:
//   - 요청 본문 자동 처리: 구조체를 전달하면 자동으로 JSON 마샬링하여 전송
//   - 응답 검증: Content-Type 확인 및 HTML 응답 감지
//   - 메모리 보호: maxResponseBodySize 제한을 통한 대용량 응답 방어
//   - 자동 재시도 지원: 네트워크 오류 시 Fetcher가 요청을 재시도할 수 있도록 본문을 메모리 버퍼링
//
// 매개변수:
//   - ctx: 요청의 생명주기를 제어하는 컨텍스트 (취소, 타임아웃 등)
//   - method: HTTP 메서드 (예: "GET", "POST")
//   - urlStr: 요청할 URL
//   - body: 요청 본문 데이터 (nil 가능, GET 요청 시 일반적으로 nil)
//   - header: 추가 HTTP 헤더 (nil 가능, 예: User-Agent, Cookie 등)
//   - v: JSON 응답을 디코딩할 대상 구조체의 포인터 (반드시 nil이 아닌 포인터여야 함)
//
// 반환값:
//   - error: 네트워크 오류, JSON 파싱 오류, 또는 응답 크기 초과 시 에러 반환
func (s *scraper) FetchJSON(ctx context.Context, method, urlStr string, body any, header http.Header, v any) error {
	// 0단계: 디코딩 대상(v) 검증
	// JSON 디코딩을 위해서는 결과를 담을 '구조체의 포인터'가 필요합니다.
	// 만약 v가 nil이거나 포인터가 아니라면, 디코딩된 데이터를 저장할 수 없으므로
	// 네트워크 요청 전에 즉시 에러를 반환하여 개발자의 실수를 조기에 알립니다.
	if v == nil {
		return ErrDecodeTargetNil
	}
	if rv := reflect.ValueOf(v); rv.Kind() != reflect.Ptr || rv.IsNil() {
		return newErrDecodeTargetInvalidType(v)
	}

	// 1단계: 요청 본문(Body) 처리
	// prepareBody는 전달받은 body를 메모리 버퍼로 읽어들여 재사용 가능한 리더로 변환합니다.
	// 이를 통해 네트워크 오류 발생 시 Fetcher가 동일한 본문으로 요청을 재시도할 수 있습니다.
	// 또한 maxRequestBodySize를 초과하는 본문은 이 단계에서 거부됩니다.
	reqBody, err := s.prepareBody(ctx, body)
	if err != nil {
		return err
	}

	// 2단계: HTTP 헤더 구성
	// 요청 본문이 존재하는 경우, 올바른 처리를 위해 Content-Type 헤더를 설정합니다.
	// 사용자가 명시적으로 헤더를 제공한 경우 이를 존중하되, 필수 헤더가 누락된 경우 기본값을 적용합니다.
	if reqBody != nil {
		if header == nil {
			// 헤더가 없는 경우 새로 생성합니다.
			header = make(http.Header)
		} else {
			// 호출자가 전달한 원본 헤더가 변경되지 않도록 복사본을 사용합니다.
			header = header.Clone()
		}

		// Content-Type이 명시되지 않은 경우, JSON API 호출의 표준인 "application/json"을 기본값으로 설정합니다.
		if header.Get("Content-Type") == "" {
			header.Set("Content-Type", "application/json")
		}
	}

	// 3단계: HTTP 요청 실행을 위한 파라미터 구성
	// executeRequest 함수가 실제 네트워크 요청을 수행할 수 있도록 필요한 정보들을 requestParams 구조체에 담습니다.
	opts := requestParams{
		Method:        method,
		URL:           urlStr,
		Body:          reqBody,
		Header:        header,
		DefaultAccept: "application/json", // 서버에 JSON 응답을 선호함을 알립니다.
		Validator: func(resp *http.Response, logger *applog.Entry) error {
			// 응답 검증: 상태 코드 확인(checkResponse) 외에 추가적으로
			// 응답 헤더의 Content-Type이 JSON인지, 혹은 HTML 페이지가 잘못 반환되었는지 검사합니다.
			// (REST API 요청 시 종종 발생하는 에러 페이지 반환 케이스를 감지하기 위함)
			return s.verifyJSONContentType(resp, urlStr, logger)
		},
	}

	// 4단계: HTTP 요청 실행 및 응답 수신
	// executeRequest를 통해 실제 네트워크 요청을 수행하고, 응답 본문을 메모리 버퍼(scrapedResp.Body)로 읽어들입니다.
	// 이때 scrapedResp.Response.Body는 이미 NopCloser로 교체된 상태이므로,
	// 이후의 Close 호출은 실질적인 네트워크 리소스 해제가 아닌, API 규약을 준수하기 위한 관례적 명시입니다.
	// (실제 네트워크 연결 해제는 executeRequest 내부에서 이미 처리되었습니다)
	scrapedResp, logger, err := s.executeRequest(ctx, opts)
	if err != nil {
		return err
	}
	defer scrapedResp.Response.Body.Close()

	// 5단계: JSON 디코딩 및 데이터 매핑
	// 메모리에 확보된 응답 본문(scrapedResp.Body)을 디코딩하여 대상 구조체(v)에 저장합니다.
	// 이 과정에서 문자열 인코딩 변환(Charset)과 JSON 문법 검사(Strict Mode)가 수행됩니다.
	return s.decodeJSONResponse(scrapedResp, v, urlStr, logger)
}

// @@@@@
// verifyJSONContentType JSON 응답에 대한 Content-Type 유효성을 검증합니다.
func (s *scraper) verifyJSONContentType(resp *http.Response, url string, logger *applog.Entry) error {
	// 204 No Content 응답은 Body가 없으므로 Content-Type 검사를 건너뜁니다.
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	contentType := resp.Header.Get("Content-Type")

	// 1. HTML Content-Type은 명확히 에러 처리
	if isHTMLContentType(contentType) {
		// ErrValidation으로 래핑
		return apperrors.Wrap(ErrValidation, apperrors.ExecutionFailed, fmt.Sprintf("JSON 응답을 기대했으나 HTML 응답이 수신되었습니다. (URL: %s, Content-Type: %s)", url, contentType))
	}

	// 2. JSON Content-Type이 아닌 경우 경고 로그
	// 많은 API가 text/plain이나 잘못된 Content-Type을 사용하므로 강제 에러 처리(Strict Mode)는 하지 않습니다.
	// 대신 경고 로그를 남겨 추후 디버깅에 활용합니다.
	if contentType != "" && !strings.Contains(strings.ToLower(contentType), "json") {
		logger.WithField("content_type", contentType).Warn("JSON 응답을 기대했으나 비표준 Content-Type이 수신되었습니다. (파싱 계속 진행)")
	}
	return nil
}

// @@@@@
// decodeJSONResponse 응답 본문을 JSON으로 디코딩합니다.
func (s *scraper) decodeJSONResponse(scraped scrapedResponse, v any, url string, logger *applog.Entry) error {
	// 204 No Content 처리: Body가 없으므로 디코딩 없이 종료
	if scraped.Response.StatusCode == http.StatusNoContent {
		logger.Debug("[성공]: 204 No Content 응답 수신, 디코딩 생략")
		return nil
	}

	// Truncation 확인: JSON은 부분 파싱이 불가능하므로 Truncated된 경우 에러 처리
	if scraped.IsTruncated {
		errMsg := fmt.Sprintf("[JSON 파싱 불가]: 응답 본문 크기가 설정된 제한(%d bytes)을 초과하여 데이터 무결성을 보장할 수 없습니다. (대상 URL: %s)", s.maxResponseBodySize, url)
		logger.WithField("truncated", true).Error("[실패]: JSON 파싱 중단, 응답 본문 크기 초과(Truncated)")
		// ErrTooLarge로 래핑
		return apperrors.Wrap(ErrTooLarge, apperrors.ExecutionFailed, errMsg)
	}

	logger.Debug("[성공]: JSON 요청 완료, 파싱 단계 진입")

	// Charset 감지 및 디코딩: Content-Type 헤더를 확인하여 UTF-8이 아닌 경우 변환 시도
	contentType := scraped.Response.Header.Get("Content-Type")
	utf8Reader, err := charset.NewReader(scraped.Response.Body, contentType)
	if err != nil {
		// 변환 실패 시 경고 로그 후 원본 스트림 사용 (매우 드문 케이스)
		logger.WithError(err).Warn("[경고]: Charset 리더 생성 실패, 원본 리더 사용(Fallback)")
		utf8Reader = scraped.Response.Body
	}

	// UseNumber를 제거하여 표준 json.Unmarshal 동작(float64)을 따릅니다.
	decoder := json.NewDecoder(utf8Reader)
	// decoder.UseNumber() // Removed for compatibility logic
	if err = decoder.Decode(v); err != nil {
		logEntry := logger.WithError(err).WithField("body_snippet", s.previewBody(scraped.Body, scraped.Response.Header.Get("Content-Type")))

		// Syntax 에러인 경우 오프셋 정보를 포함하여 디버깅을 돕습니다.
		var syntaxErr *json.SyntaxError
		errMsg := fmt.Sprintf("불러온 페이지(%s) 데이터의 JSON 변환이 실패하였습니다.", url)

		if errors.As(err, &syntaxErr) {
			// 오프셋은 디코딩된 UTF-8 스트림 기준이므로, 원본 바이트(scraped.Body)가 아닌
			// 디코딩된 바이트에서 스니펫을 추출해야 정확합니다.
			// 성능 비용이 들지만 에러 상황 디버깅을 위해 재변환을 수행합니다.
			// [Optimization] 전체를 다시 읽어 메모리에 올리는 대신, 에러 오프셋 주변만 읽습니다.

			const contextSize = 50
			offset := int(syntaxErr.Offset)

			// 읽기 시작 위치 계산 (오프셋 - contextSize, 0보다 작으면 0)
			startPos := int64(offset - contextSize)
			if startPos < 0 {
				startPos = 0
			}

			// 읽을 길이 계산 (contextSize * 2, 적절히 조정)
			readLen := int64(contextSize * 2)

			var snippet string

			if r, err := charset.NewReader(bytes.NewReader(scraped.Body), contentType); err == nil {
				// 1. 시작 위치까지 스킵
				if startPos > 0 {
					// charset Reader는 Seek을 지원하지 않을 수 있으므로 Discard 사용
					// (bytes.Reader 기반이지만 charset 변환 래퍼가 씌워져 있음)
					_, _ = io.CopyN(io.Discard, r, startPos)
				}

				// 2. 필요한 만큼만 읽기
				buf := make([]byte, readLen)
				n, _ := io.ReadFull(r, buf)
				snippet = string(buf[:n])
			} else {
				// 변환 실패 시 원본 사용 (Fallback)
				start := offset - contextSize
				if start < 0 {
					start = 0
				}
				end := offset + contextSize
				if end > len(scraped.Body) {
					end = len(scraped.Body)
				}
				snippet = string(scraped.Body[start:end])
			}

			// UTF-8 Rune 경계 보정은 복잡하므로 단순화하거나 생략 (이미 charset reader를 통과했거나, 원본 바이트 슬라이싱)
			// 여기서는 로그 목적이므로 깨진 문자 포함 가능성 감수하고 단순화

			errMsg += fmt.Sprintf(" (SyntaxError at offset %d: ...%s...)", syntaxErr.Offset, snippet)
			logEntry = logEntry.WithField("syntax_error_offset", syntaxErr.Offset).WithField("syntax_error_context", snippet)
		}

		logEntry.Error("[실패]: JSON 디코딩 에러, 데이터 변환 불가")
		// ErrParsing으로 래핑
		return apperrors.Wrap(err, apperrors.ParsingFailed, errMsg)
	}

	// Strict Mode: JSON 데이터 뒤에 불필요한 데이터가 있는지 확인
	// decoder.More()는 현재 파싱 중인 객체/배열 내부에서 다음 요소가 있는지 확인할 때 사용합니다.
	// 최상위 객체 파싱이 끝난 시점에서는 호출하면 안 됩니다.
	// 대신 decoder.Token()을 호출하여 EOF가 반환되는지 확인합니다.
	if token, err := decoder.Token(); err != io.EOF {
		errMsg := fmt.Sprintf("불러온 페이지(%s) 데이터에 불필요한 데이터가 포함되어 있습니다. (Unexpected Token)", url)
		logger.WithField("unexpected_token", token).Error("[취약]: JSON Strict Parsing 실패 (EOF Expected)")
		return apperrors.New(apperrors.ParsingFailed, errMsg)
	}

	logger.WithField("status_code", scraped.Response.StatusCode).Debug("[성공]: JSON 요청 및 파싱 완료")
	return nil
}
