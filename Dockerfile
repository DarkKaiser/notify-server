# ------------------------------------------
# 1. Build Image
# ------------------------------------------
FROM golang:1.23.4-alpine AS builder

ARG APP_NAME=notify-server
ARG TARGETARCH=arm64

WORKDIR /go/src/app/

COPY . .

ENV GO111MODULE=on

# Alpine에서 빌드 시 필요한 패키지 설치 (필요한 경우)
# RUN apk add --no-cache git

# golangci-lint 설치 (공식 이미지에서 바이너리 복사)
# COPY --from=golangci/golangci-lint:v1.55.2 /usr/bin/golangci-lint /usr/bin/golangci-lint

# 린트 검사 실행 (실패 시 빌드 중단)
# RUN golangci-lint run ./...

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -a -ldflags="-s -w" -o ${APP_NAME} .

# ------------------------------------------
# 2. Production Image
# ------------------------------------------
FROM alpine:latest

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

WORKDIR /usr/local/app/

ENTRYPOINT ["/docker-entrypoint/docker-entrypoint.sh"]
CMD ["/usr/local/app/notify-server"]
