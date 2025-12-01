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

# Alpine에서 빌드 시 필요한 패키지 설치
RUN apk add --no-cache git

# Swagger 문서 생성
RUN go install github.com/swaggo/swag/cmd/swag@latest
RUN swag init

# golangci-lint 설치 및 실행
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

# 필수 패키지 설치 (bash, ca-certificates, tzdata)
RUN apk --no-cache add bash ca-certificates tzdata

WORKDIR /docker-entrypoint/dist/

# 빌드 결과물 복사
COPY --from=builder /go/src/app/${APP_NAME} .

# SSL 인증서 복사
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# 스크립트 및 설정 복사
COPY docker-entrypoint.sh /docker-entrypoint/
RUN chmod +x /docker-entrypoint/docker-entrypoint.sh

COPY ./secrets/${APP_NAME}.운영.json /docker-entrypoint/dist/${APP_NAME}.json

# 작업 디렉토리 변경
WORKDIR /usr/local/app/


# 포트 노출
EXPOSE 2443

ENTRYPOINT ["/docker-entrypoint/docker-entrypoint.sh"]
CMD ["/usr/local/app/notify-server"]
