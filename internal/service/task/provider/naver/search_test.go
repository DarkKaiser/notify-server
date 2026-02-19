package naver

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPerformanceSearchURL(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		page       int
		wantSubstr []string
	}{
		{
			name:       "기본 URL 생성 확인",
			query:      "뮤지컬",
			page:       1,
			wantSubstr: []string{"u1=%EB%AE%A4%EC%A7%80%EC%BB%AC", "u7=1"}, // u1, u7 check
		},
		{
			name:       "특수문자 쿼리 인코딩 확인",
			query:      "A & B",
			page:       2,
			wantSubstr: []string{"u1=A+%26+B", "u7=2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPerformanceSearchURL(tt.query, tt.page)
			assert.Contains(t, got, performanceSearchEndpoint)
			for _, substr := range tt.wantSubstr {
				assert.Contains(t, got, substr)
			}
		})
	}
}

func TestResolveAbsoluteURL(t *testing.T) {
	baseURL := "https://example.com/base/page.html"

	tests := []struct {
		name      string
		targetURL string
		want      string
		wantErr   bool
	}{
		{
			name:      "절대 URL: 그대로 반환",
			targetURL: "https://other.com/image.jpg",
			want:      "https://other.com/image.jpg",
			wantErr:   false,
		},
		{
			name:      "프로토콜 생략형(//): 스킴 적용",
			targetURL: "//cdn.example.com/image.jpg",
			want:      "https://cdn.example.com/image.jpg",
			wantErr:   false,
		},
		{
			name:      "절대 경로(/): 호스트 루트 기준",
			targetURL: "/images/logo.png",
			want:      "https://example.com/images/logo.png",
			wantErr:   false,
		},
		{
			name:      "상대 경로(./): 현재 경로 기준",
			targetURL: "./icon.png",
			want:      "https://example.com/base/icon.png",
			wantErr:   false,
		},
		{
			name:      "상대 경로(../): 상위 경로 기준",
			targetURL: "../style.css",
			want:      "https://example.com/style.css",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveAbsoluteURL(baseURL, tt.targetURL)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// 헬퍼: HTML 문자열에서 goquery.Selection 생성
func createSelectionFromHTML(t *testing.T, html string, selector string) *goquery.Selection {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	require.NoError(t, err)
	return doc.Find(selector).First()
}

func TestParsePerformance(t *testing.T) {
	pageURL := "https://search.naver.com/search.naver"

	t.Run("성공: 모든 필드 존재", func(t *testing.T) {
		html := `
			<li>
				<div class="title_box">
					<span class="name">뮤지컬 오페라의 유령</span>
					<span class="sub_text">샤롯데씨어터</span>
				</div>
				<div class="thumb">
					<img src="https://ssl.pstatic.net/img.png">
				</div>
			</li>`
		sel := createSelectionFromHTML(t, html, "li")

		perf, err := parsePerformance(sel, pageURL)
		assert.NoError(t, err)
		assert.Equal(t, "뮤지컬 오페라의 유령", perf.Title)
		assert.Equal(t, "샤롯데씨어터", perf.Place)
		assert.Equal(t, "https://ssl.pstatic.net/img.png", perf.Thumbnail)
	})

	t.Run("실패: 제목 누락", func(t *testing.T) {
		html := `
			<li>
				<div class="title_box">
					<!-- name class missing -->
					<span class="sub_text">샤롯데씨어터</span>
				</div>
			</li>`
		sel := createSelectionFromHTML(t, html, "li")

		perf, err := parsePerformance(sel, pageURL)
		assert.Error(t, err)
		assert.Nil(t, perf)
		assert.Contains(t, err.Error(), "HTML 구조 변경")
	})

	t.Run("실패: 장소 누락", func(t *testing.T) {
		html := `
			<li>
				<div class="title_box">
					<span class="name">뮤지컬 오페라의 유령</span>
					<!-- sub_text missing -->
				</div>
			</li>`
		sel := createSelectionFromHTML(t, html, "li")

		perf, err := parsePerformance(sel, pageURL)
		assert.Error(t, err)
		assert.Nil(t, perf)
	})

	t.Run("성공: 썸네일 없음 (선택적 필드)", func(t *testing.T) {
		html := `
			<li>
				<div class="title_box">
					<span class="name">뮤지컬 오페라의 유령</span>
					<span class="sub_text">샤롯데씨어터</span>
				</div>
				<!-- thumb missing -->
			</li>`
		sel := createSelectionFromHTML(t, html, "li")

		perf, err := parsePerformance(sel, pageURL)
		assert.NoError(t, err)
		assert.Equal(t, "뮤지컬 오페라의 유령", perf.Title)
		assert.Empty(t, perf.Thumbnail)
	})
}
