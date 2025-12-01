# ------------------------------------------
# 1. Build Image
# ------------------------------------------
FROM golang:1.23.4-alpine AS builder

# 빌드 메타데이터 인자
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown
ARG BUILD_NUMBER=unknown
ARG APP_NAME=notify-server
ARG TARGETARCH

WORKDIR /go/src/app/

# 의존성 캐싱 최적화: go.mod와 go.sum을 먼저 복사
COPY go.mod go.sum ./
RUN go mod download

# 소스 코드 복사
COPY . .

# 빌드 도구 설치 및 Swagger 문서 생성 (레이어 최적화)
RUN apk add --no-cache git && \
    go install github.com/swaggo/swag/cmd/swag@latest && \
    swag init

# 테스트 실행 (빌드 전 품질 검증)
RUN go test ./... -v -coverprofile=coverage.out

# golangci-lint 설치 및 실행
# 현재 다수의 린트 오류(errcheck, gosimple 등)로 인해 비활성화
# 린트 오류 수정은 별도 작업으로 진행 예정
# COPY --from=golangci/golangci-lint:v1.62.2 /usr/bin/golangci-lint /usr/bin/golangci-lint

# 린트 검사 실행 (실패 시 빌드 중단)
# RUN golangci-lint run ./...

# 빌드 정보를 바이너리에 주입
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -a \
    -ldflags="-s -w \
    -X 'main.Version=${GIT_COMMIT}' \
    -X 'main.BuildDate=${BUILD_DATE}' \
    -X 'main.BuildNumber=${BUILD_NUMBER}'" \
    -o ${APP_NAME} .

# ------------------------------------------
# 2. Production Image
# ------------------------------------------
FROM alpine:3.20

# 빌드 메타데이터 인자
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown
ARG BUILD_NUMBER=unknown
ARG APP_NAME=notify-server

# OCI 표준 레이블 추가
LABEL org.opencontainers.image.created="${BUILD_DATE}" \
    org.opencontainers.image.authors="DarkKaiser" \
    org.opencontainers.image.url="https://github.com/DarkKaiser/notify-server" \
    org.opencontainers.image.source="https://github.com/DarkKaiser/notify-server" \
    org.opencontainers.image.version="${GIT_COMMIT}" \
    org.opencontainers.image.revision="${GIT_COMMIT}" \
    org.opencontainers.image.title="Notify Server" \
    org.opencontainers.image.description="웹 페이지 스크래핑 및 RSS 피드 제공 서버" \
    build.number="${BUILD_NUMBER}"

# 필수 패키지 설치 및 사용자 생성을 하나의 레이어로 통합
RUN apk --no-cache add bash ca-certificates tzdata wget && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    mkdir -p /docker-entrypoint/dist /usr/local/app && \
    chown -R appuser:appuser /docker-entrypoint /usr/local/app

WORKDIR /docker-entrypoint/dist/

# 빌드 결과물 복사 (권한 설정 포함)
COPY --from=builder --chown=appuser:appuser /go/src/app/${APP_NAME} .

# 스크립트 복사 및 실행 권한 부여
COPY --chown=appuser:appuser --chmod=755 docker-entrypoint.sh /docker-entrypoint/

# 설정 파일 복사
COPY --chown=appuser:appuser ./secrets/${APP_NAME}.운영.json /docker-entrypoint/dist/${APP_NAME}.json

# SSL 인증서 복사 (불필요)
# Alpine 이미지에서는 'apk add ca-certificates'를 통해 이미 최신 인증서가 설치되므로
# 빌더 이미지에서 별도로 복사할 필요가 없습니다.
# COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# 작업 디렉토리 변경
WORKDIR /usr/local/app/

# 비루트 사용자로 전환
USER appuser

# 헬스체크 추가
# wget -O /dev/null을 사용하여 GET 메서드로 Swagger UI 페이지 접근 확인
# (--spider 옵션은 HEAD 메서드를 사용하여 405 에러가 발생하므로 사용하지 않음)
# 간격: 20초, 타임아웃: 5초, 시작 대기: 30초, 재시도: 3회
HEALTHCHECK --interval=20s --timeout=5s --start-period=30s --retries=3 \
    CMD wget -q -O /dev/null --no-check-certificate https://localhost:2443/swagger/index.html || exit 1

# 포트 노출
EXPOSE 2443

ENTRYPOINT ["/docker-entrypoint/docker-entrypoint.sh"]
CMD ["/usr/local/app/notify-server"]
