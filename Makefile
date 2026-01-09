# Makefile for notify-server
# Go 프로젝트 빌드 및 개발 자동화

.PHONY: help
help: ## 도움말 표시
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: install-tools
install-tools: ## 필수 개발 도구 설치
	@echo "📦 필수 도구 설치 중..."
	go install golang.org/x/tools/cmd/stringer@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "✅ 도구 설치 완료"

.PHONY: generate
generate: ## 코드 생성 (stringer, swagger)
	@echo "🔄 코드 생성 중..."
	go generate ./...
	swag init -g cmd/notify-server/main.go
	@echo "✅ 코드 생성 완료"

.PHONY: test
test: generate ## 테스트 실행 (커버리지 포함)
	@echo "🧪 테스트 실행 중..."
	go test ./... -v -coverprofile=coverage.out
	@echo "📊 커버리지 요약:"
	@go tool cover -func=coverage.out | tail -n 1

.PHONY: test-short
test-short: generate ## 빠른 테스트 (커버리지 제외)
	@echo "🧪 빠른 테스트 실행 중..."
	go test ./... -short

.PHONY: coverage
coverage: test ## 커버리지 HTML 리포트 생성
	@echo "📊 커버리지 리포트 생성 중..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ coverage.html 파일 생성 완료"

.PHONY: build
build: generate ## 바이너리 빌드
	@echo "🔨 빌드 중..."
	go build -o notify-server ./cmd/notify-server
	@echo "✅ 빌드 완료: notify-server"

.PHONY: build-windows
build-windows: generate ## Windows용 빌드
	@echo "🔨 Windows 빌드 중..."
	GOOS=windows GOARCH=amd64 go build -o notify-server.exe ./cmd/notify-server
	@echo "✅ 빌드 완료: notify-server.exe"

.PHONY: run
run: generate ## 로컬 실행
	@echo "🚀 서버 실행 중..."
	go run ./cmd/notify-server

.PHONY: docker-build
docker-build: ## Docker 이미지 빌드
	@echo "🐳 Docker 이미지 빌드 중..."
	docker build -t notify-server:dev .
	@echo "✅ 이미지 빌드 완료: notify-server:dev"

.PHONY: docker-run
docker-run: docker-build ## Docker 컨테이너 실행
	@echo "🐳 Docker 컨테이너 실행 중..."
	docker run --rm -p 2443:2443 notify-server:dev

.PHONY: lint
lint: generate ## 린트 검사
	@echo "🔍 린트 검사 중..."
	golangci-lint run ./...

.PHONY: fmt
fmt: ## 코드 포맷팅
	@echo "✨ 코드 포맷팅 중..."
	go fmt ./...
	@echo "✅ 포맷팅 완료"

.PHONY: clean
clean: ## 빌드 산출물 정리
	@echo "🧹 정리 중..."
	rm -f notify-server notify-server.exe
	rm -f coverage.out coverage.html
	rm -f internal/pkg/errors/errortype_string.go
	@echo "✅ 정리 완료"

.PHONY: deps
deps: ## 의존성 다운로드
	@echo "📦 의존성 다운로드 중..."
	go mod download
	go mod tidy
	@echo "✅ 의존성 업데이트 완료"

.PHONY: verify
verify: generate lint test ## 전체 검증 (lint + test)
	@echo "✅ 모든 검증 통과"

# 기본 타겟
.DEFAULT_GOAL := help
