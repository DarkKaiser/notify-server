package scraper

import (
	"context"
	"io"
	"net/http"

	"github.com/PuerkitoBio/goquery"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
)

// defaultMaxBodySize HTTP 요청/응답 본문의 기본 최대 크기입니다.
// 이 값은 메모리 사용량을 제어하고 악의적인 대용량 데이터로부터 시스템을 보호하기 위해 사용됩니다.
// WithMaxRequestBodySize 또는 WithMaxResponseBodySize 옵션을 통해 변경할 수 있습니다.
const defaultMaxBodySize = 10 * 1024 * 1024 // 10MB

// HTMLScraper HTML 페이지 스크래핑을 위한 인터페이스입니다.
//
// 이 인터페이스는 웹 페이지에서 HTML 문서를 가져오고 파싱하는 기능을 제공합니다.
// goquery.Document를 반환하여 CSS 선택자 기반의 데이터 추출을 지원합니다.
type HTMLScraper interface {
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
	FetchHTML(ctx context.Context, method, urlStr string, body io.Reader, header http.Header) (*goquery.Document, error)

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
	FetchHTMLDocument(ctx context.Context, urlStr string, header http.Header) (*goquery.Document, error)

	// @@@@@
	// ParseReader io.Reader로부터 HTML 문서를 파싱합니다.
	// 이미 메모리에 로드된 HTML 데이터나 파일에서 읽은 데이터를 파싱할 때 사용합니다.
	//
	// 매개변수:
	//   - ctx: 파싱의 컨텍스트
	//   - r: HTML 데이터를 읽을 Reader
	//   - urlStr: 문서의 URL (상대 경로 링크 처리용, 빈 문자열 가능)
	//   - contentType: Content-Type 헤더 값 (인코딩 감지용, 빈 문자열 가능)
	ParseReader(ctx context.Context, r io.Reader, urlStr string, contentType string) (*goquery.Document, error)
}

// JSONScraper JSON API 스크래핑을 위한 인터페이스입니다.
//
// 이 인터페이스는 RESTful API 등 JSON 형식의 데이터를 제공하는 엔드포인트에서
// 데이터를 가져오고 Go 구조체로 자동 변환하는 기능을 제공합니다.
type JSONScraper interface {
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
	FetchJSON(ctx context.Context, method, urlStr string, body any, header http.Header, v any) error
}

// Scraper 웹 페이지 스크래핑을 위한 통합 인터페이스입니다.
type Scraper interface {
	HTMLScraper
	JSONScraper
}

// scraper Scraper 인터페이스의 구현체입니다.
//
// 이 구조체는 웹 페이지 스크래핑을 수행하는 실제 구현을 담당합니다.
// Fetcher를 통해 HTTP 요청을 수행하고, HTML 파싱 및 JSON 디코딩 기능을 제공합니다.
//
// 주요 기능:
//   - 자동 인코딩 감지 및 변환 (EUC-KR, UTF-8 등)
//   - 응답 크기 제한을 통한 메모리 보호
//   - 컨텍스트 기반 취소 및 타임아웃 지원
//   - 커스텀 응답 콜백 지원
type scraper struct {
	// fetcher 실제 HTTP 요청을 수행하는 컴포넌트입니다.
	fetcher fetcher.Fetcher

	// maxRequestBodySize HTTP 요청 본문의 최대 읽기 크기(바이트)입니다.
	// 이 값을 초과하는 요청 본문은 에러를 발생시킵니다.
	maxRequestBodySize int64

	// maxResponseBodySize HTTP 응답 본문의 최대 읽기 크기(바이트)입니다.
	// 이 값을 초과하는 응답 본문은 잘려서(truncated) 처리됩니다.
	maxResponseBodySize int64

	// responseCallback HTTP 응답 수신 직후 실행될 콜백 함수입니다.
	// 응답 헤더나 상태 코드를 검사할 때 사용할 수 있습니다.
	responseCallback func(*http.Response)
}

// New 새로운 Scraper 인터페이스 구현체를 생성합니다.
//
// 매개변수:
//   - f: HTTP 요청을 수행할 Fetcher (필수, nil이면 패닉 발생)
//   - opts: 선택적 설정 옵션들
//
// 반환값:
//   - Scraper: 생성된 스크래퍼 인스턴스
func New(f fetcher.Fetcher, opts ...Option) Scraper {
	if f == nil {
		panic("Fetcher는 필수입니다")
	}

	s := &scraper{
		fetcher: f,

		maxRequestBodySize:  defaultMaxBodySize,
		maxResponseBodySize: defaultMaxBodySize,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}
