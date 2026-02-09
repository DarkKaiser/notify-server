package scraper

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/url"
	"reflect"

	"github.com/PuerkitoBio/goquery"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"golang.org/x/net/html/charset"
)

// FetchHTML 지정된 URL로 HTTP 요청을 보내 HTML 문서를 가져오고, 파싱된 goquery.Document를 반환합니다.
//
// 매개변수:
//   - ctx: 요청의 생명주기를 제어하는 컨텍스트 (취소, 타임아웃 등)
//   - method: HTTP 메서드 (예: "GET", "POST")
//   - urlStr: 요청할 URL
//   - body: 요청 본문 데이터 (nil 가능, GET 요청 시 일반적으로 nil)
//   - header: 추가 HTTP 헤더 (nil 가능, 예: User-Agent, Cookie 등)
//
// 반환값:
//   - *goquery.Document: 파싱된 HTML 문서 객체
//   - error: 네트워크 오류, 파싱 오류, 또는 응답 크기 초과 시 에러 반환
func (s *scraper) FetchHTML(ctx context.Context, method, urlStr string, body io.Reader, header http.Header) (*goquery.Document, error) {
	// 1단계: 요청 본문(Body) 처리
	// prepareBody는 전달받은 body를 메모리 버퍼로 읽어들여 재사용 가능한 리더로 변환합니다.
	// 이를 통해 네트워크 오류 발생 시 Fetcher가 동일한 본문으로 요청을 재시도할 수 있습니다.
	// 또한 maxRequestBodySize를 초과하는 본문은 이 단계에서 거부됩니다.
	reqBody, err := s.prepareBody(ctx, body)
	if err != nil {
		return nil, err
	}

	// 2단계: HTTP 요청 실행을 위한 파라미터 구성
	// executeRequest 함수가 실제 네트워크 요청을 수행할 수 있도록 필요한 정보들을 requestParams 구조체에 담습니다.
	opts := requestParams{
		Method:        method,
		URL:           urlStr,
		Body:          reqBody,
		Header:        header,
		DefaultAccept: "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		Validator: func(resp *http.Response, logger *applog.Entry) error {
			// 응답 검증: 상태 코드 확인(checkResponse) 외에 추가적으로
			// 응답 헤더의 Content-Type이 HTML 형식인지 확인합니다.
			// 비표준 Content-Type(예: text/plain 등)이라도 내용이 HTML일 수 있으므로,
			// 엄격하게 차단하지 않고 경고 로그만 남긴 후 파싱을 계속 진행합니다.
			return s.verifyHTMLContentType(resp, logger)
		},
	}

	// 3단계: HTTP 요청 실행 및 응답 수신
	// executeRequest를 통해 실제 네트워크 요청을 수행하고, 응답 본문을 메모리 버퍼(scrapedResp.Body)로 읽어들입니다.
	// 이때 scrapedResp.Response.Body는 이미 NopCloser로 교체된 상태이므로,
	// 이후의 Close 호출은 실질적인 네트워크 리소스 해제가 아닌, API 규약을 준수하기 위한 관례적 명시입니다.
	// (실제 네트워크 연결 해제는 executeRequest 내부에서 이미 처리되었습니다)
	scrapedResp, logger, err := s.executeRequest(ctx, opts)
	if err != nil {
		return nil, err
	}
	defer scrapedResp.Response.Body.Close()

	// 4단계: 응답 크기 확인
	// executeRequest는 maxResponseBodySize를 초과하는 응답을 자동으로 잘라냅니다.
	// HTML 파싱의 무결성을 보장하기 위해, 잘린(Truncated) 응답은 에러로 처리합니다.
	// (참고: 이미지나 비디오 파일 등은 Stream 처리가 가능하거나 메타데이터만 필요할 수 있어 Truncation을 허용하기도 하지만,
	//  HTML/JSON 구조 데이터는 완결성이 필수적이므로 엄격하게 차단합니다)
	if scrapedResp.IsTruncated {
		logger.WithFields(applog.Fields{
			"truncated":   true,
			"status_code": scrapedResp.Response.StatusCode,
			"body_size":   len(scrapedResp.Body),
		}).Error("[실패]: HTTP 요청 완료 후 파싱 중단, 응답 본문 크기 초과(Truncated)")

		return nil, newErrResponseBodyTooLarge(s.maxResponseBodySize, urlStr)
	}

	logger.WithFields(applog.Fields{
		"status_code": scrapedResp.Response.StatusCode,
		"body_size":   len(scrapedResp.Body),
	}).Debug("[성공]: HTML 요청 완료, 파싱 단계 진입")

	// 5단계: Content-Type 추출 (7단계의 HTML 파싱 시 Charset 변환을 위한 힌트)
	contentType := scrapedResp.Response.Header.Get("Content-Type")

	// 6단계: 문서 URL 결정 (상대 경로 해석용)
	// HTML 내의 상대 경로(예: <a href="/path">)를 절대 경로로 변환하기 위한 기준 URL을 설정합니다.
	// 리다이렉션 후의 최종 URL(Response.Request.URL)을 우선 사용하며,
	// 만약 Request 객체가 없는 경우(Mocking 등)를 대비해 초기 요청 URL을 Fallback으로 사용합니다.
	var baseURL *url.URL
	if scrapedResp.Response.Request != nil {
		baseURL = scrapedResp.Response.Request.URL
	} else {
		if parsedURL, err := url.Parse(urlStr); err == nil {
			baseURL = parsedURL
		} else {
			logger.WithError(err).
				Warn("[주의]: Base URL 결정 실패, Fallback 파싱 에러")
		}
	}

	// 7단계: HTML 파싱 실행
	// parseHTML을 통해 메모리에 버퍼링된 응답 본문을 읽어 goquery.Document를 생성합니다.
	//  - scrapedResp.Response.Body: executeRequest에서 이미 메모리로 읽어들인 응답 본문 (NopCloser로 래핑된 bytes.Reader)
	//  - contextAwareReader 래핑: 파싱 도중 Context가 취소되면 작업을 즉시 중단합니다.
	//  - baseURL: HTML 내의 상대 경로(href="/...")를 절대 경로로 변환하기 위한 기준 URL입니다.
	//  - contentType: 응답 헤더의 Charset 정보를 기반으로 인코딩을 자동 변환(예: EUC-KR → UTF-8)합니다.
	doc, err := s.parseHTML(ctx, &contextAwareReader{ctx: ctx, r: scrapedResp.Response.Body}, baseURL, contentType)
	if err != nil {
		// 파싱 실패 시 디버깅을 위해 응답 본문의 일부를 로그에 포함합니다.
		logger.WithError(err).
			WithFields(applog.Fields{
				"status_code":  scrapedResp.Response.StatusCode,
				"content_type": contentType,
				"body_size":    len(scrapedResp.Body),
				"body_preview": s.previewBody(scrapedResp.Body, contentType),
			}).
			Error("[실패]: HTML 파싱 에러, goquery Document 생성 실패")

		return nil, newErrHTMLParseFailed(urlStr, err)
	}

	logger.WithFields(applog.Fields{
		"status_code":  scrapedResp.Response.StatusCode,
		"content_type": contentType,
		"body_size":    len(scrapedResp.Body),
	}).Debug("[성공]: HTML 요청 및 파싱 완료")

	return doc, nil
}

// FetchHTMLDocument 지정된 URL로 GET 요청을 보내 HTML 문서를 가져오는 헬퍼 함수입니다.
//
// 이 함수는 FetchHTML을 내부적으로 호출하며, HTTP 메서드를 "GET"으로 고정하고 요청 본문(Body)을 nil로 설정합니다.
// 단순히 웹 페이지를 읽어오는 가장 일반적인 사용 사례를 위한 간편한 인터페이스를 제공합니다.
//
// 매개변수:
//   - ctx: 요청의 생명주기를 제어하는 컨텍스트 (취소, 타임아웃 등)
//   - urlStr: 요청할 URL
//   - header: 추가 HTTP 헤더 (nil 가능, 예: User-Agent, Cookie 등)
//
// 반환값:
//   - *goquery.Document: 파싱된 HTML 문서 객체
//   - error: 네트워크 오류, 파싱 오류, 또는 응답 크기 초과 시 에러 반환
func (s *scraper) FetchHTMLDocument(ctx context.Context, urlStr string, header http.Header) (*goquery.Document, error) {
	return s.FetchHTML(ctx, http.MethodGet, urlStr, nil, header)
}

// @@@@@
// verifyHTMLContentType HTTP 응답의 Content-Type 헤더가 HTML 형식인지 검증합니다.
//
// 이 함수는 FetchHTML 요청 시 응답 검증 단계에서 호출되며, 예상치 못한 응답 타입(예: JSON, 이미지 등)을
// 조기에 감지하여 파싱 오류를 방지합니다. 다만 실무에서 많은 웹 서버가 비표준 Content-Type을 사용하므로,
// Strict Mode를 완화하여 경고 로그만 남기고 파싱을 계속 진행합니다.
//
// 매개변수:
//   - resp: 검증할 HTTP 응답 객체
//   - log: 검증 결과를 기록할 로그 엔트리
//
// 반환값:
//   - error: 현재는 항상 nil을 반환하며, 비표준 Content-Type은 경고 로그로만 처리됩니다.
//
// 특수 케이스:
//   - 204 No Content: Body가 없으므로 Content-Type 검사를 건너뜁니다.
func (s *scraper) verifyHTMLContentType(resp *http.Response, logger *applog.Entry) error {
	// 204 No Content 응답은 Body가 없으므로 Content-Type 검사를 건너뜁니다.
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	contentType := resp.Header.Get("Content-Type")
	if !isHTMLContentType(contentType) {
		// Strict Mode 완화: 에러 반환 대신 경고 로그 출력 후 진행
		// 많은 웹 서버가 잘못된 Content-Type을 반환하지만 실제로는 HTML인 경우가 많으므로
		// 파싱을 시도하고 실패 시 그때 에러 처리하는 것이 더 실용적입니다.
		logger.WithField("content_type", contentType).Warn("HTML 응답을 기대했으나 비표준 Content-Type이 수신되었습니다. (파싱 계속 진행)")
	}

	return nil
}

// @@@@@
// ParseReader io.Reader로부터 HTML 문서를 파싱하여 goquery.Document를 반환합니다.
// 이미 메모리에 로드된 HTML 데이터(문자열 등)를 처리할 때 유용하게 사용됩니다.
// url 인자는 문서 내 상대 경로 링크 처리를 위해 사용됩니다. (선택 사항)
// contentType 인자는 인코딩 감지를 위해 사용됩니다. (선택 사항, 예: "text/html; charset=euc-kr")
func (s *scraper) ParseReader(ctx context.Context, r io.Reader, urlStr string, contentType string) (*goquery.Document, error) {
	if r == nil {
		return nil, apperrors.New(apperrors.Internal, "ParseReader: reader must not be nil")
	}

	// Typed Nil 체크 (인터페이스에 nil 포인터가 할당된 경우 방지)
	val := reflect.ValueOf(r)
	if val.Kind() == reflect.Ptr && val.IsNil() {
		return nil, apperrors.New(apperrors.Internal, "ParseReader: reader must not be nil (typed nil)")
	}
	if err := ctx.Err(); err != nil {
		return nil, apperrors.Wrap(err, apperrors.ExecutionFailed, "컨텍스트가 취소되었습니다.")
	}

	var targetURL *url.URL
	if urlStr != "" {
		u, err := url.Parse(urlStr)
		if err != nil {
			// URL 파싱 실패 시, 에러를 반환하지 않고 경고 로그만 남긴 후 진행합니다.
			// URL 정보는 상대 경로 링크 처리에만 사용되므로, 파싱 자체를 막을 필요는 없습니다.
			applog.WithContext(ctx).WithField("url_string", urlStr).Warn("HTML 파싱 중 잘못된 URL 형식이 감지되었습니다. (상대 경로 링크 처리가 제한될 수 있음)")
			targetURL = nil
		} else {
			targetURL = u
		}
	}

	// [Security] 무제한 읽기 방지: maxResponseBodySize만큼만 읽도록 제한합니다.
	// 이를 통해 악의적인 대용량 입력으로 인한 DoS 공격을 방지합니다.
	limitedReader := io.LimitReader(r, s.maxResponseBodySize)

	doc, err := s.parseHTML(ctx, &contextAwareReader{ctx: ctx, r: limitedReader}, targetURL, contentType)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.ExecutionFailed, "HTML 데이터 파싱이 실패하였습니다.")
	}
	return doc, nil
}

// @@@@@
// parseHTML HTML 데이터 파싱 및 Document 생성을 담당하는 내부 공통 메서드입니다.
func (s *scraper) parseHTML(ctx context.Context, r io.Reader, targetURL *url.URL, contentType string) (*goquery.Document, error) {
	// [Fix] charset.NewReader의 불투명한 버퍼링으로 인한 데이터 소실 방지
	// bufio.Reader를 사용하여 먼저 데이터를 Peek한 후 인코딩을 결정하고,
	// 결정된 인코딩으로 원본 Reader를 래핑하여 파싱합니다.

	// 1. bufio.Reader로 래핑하여 Peek 기능 사용 (기본 4KB 버퍼)
	bufReader := bufio.NewReader(r)

	// 2. 1KB를 미리 읽어서 인코딩 감지 시도
	const peekSize = 1024
	peekBytes, _ := bufReader.Peek(peekSize) // 에러(EOF 등)가 발생해도 읽은 만큼 반환

	// 3. 인코딩 감지
	e, name, _ := charset.DetermineEncoding(peekBytes, contentType)

	// 4. 감지된 인코딩으로 변환 리더 생성
	var utf8Reader io.Reader
	if name != "" && e != nil {
		utf8Reader = e.NewDecoder().Reader(bufReader)
	} else {
		// 감지 실패 또는 기본값인 경우: UTF-8로 가정하고 원본 그대로 사용하거나,
		// DetermineEncoding이 반환한 기본 인코딩(e)을 사용합니다.
		// DetermineEncoding은 실패 시 보통 replacement 인코딩이나 windows-1252 등을 반환할 수 있으므로,
		// e가 nil이 아니라면 사용하는 것이 안전합니다.
		if e != nil {
			utf8Reader = e.NewDecoder().Reader(bufReader)
		} else {
			// 정말 아무것도 모르는 경우 (Fallback to UTF-8 / No conversion)
			// 경고 로그는 남길 수 있으나, 파싱은 시도해야 함
			applog.WithContext(ctx).Warn("HTML 인코딩 감지 실패, UTF-8로 가정하고 원본 리더를 사용합니다.")
			utf8Reader = bufReader
		}
	}

	doc, err := goquery.NewDocumentFromReader(utf8Reader)
	if err != nil {
		return nil, err
	}

	// 상대 경로 링크 처리를 위해 Document에 URL 정보 주입
	if targetURL != nil {
		doc.Url = targetURL
	}

	return doc, nil
}
