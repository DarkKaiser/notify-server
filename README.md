# NotifyServer

<p align="center">
  <img src="https://img.shields.io/badge/Go-00ADD8?style=flat&logo=Go&logoColor=white" />
  <img src="https://img.shields.io/badge/jenkins-%232C5263.svg?style=flat&logo=jenkins&logoColor=white">
  <img src="https://img.shields.io/badge/Docker-2496ED?style=flat&logo=Docker&logoColor=white">
  <img src="https://img.shields.io/badge/Linux-FCC624?style=flat&logo=linux&logoColor=black">
  <a href="https://github.com/DarkKaiser/notify-server/blob/master/LICENSE">
    <img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-yellow.svg" target="_blank" />
  </a>
</p>

NotifyServer는 외부 프로그램으로부터 수신된 메시지 및 등록된 태스크들의 실행 결과를 텔레그램 등을 통해 알리는 서버입니다.

## 🌟 주요 기능

- **다양한 알림 채널**: 텔레그램 등 다양한 메신저 지원 (확장 가능)
- **스케줄링**: Cron 표현식을 사용한 정기적인 작업 실행
- **REST API**: 외부 애플리케이션 연동을 위한 API 제공
- **웹 스크래핑**: 웹 페이지 변동 감지 및 알림 (Lotto, 쇼핑몰 가격 등)

## 🏗 아키텍처

```mermaid
graph TD
    User[User / External App] -->|HTTP Request| API[Notify API]
    Scheduler[Task Scheduler] -->|Trigger| TaskService

    subgraph Notify Server
        API -->|Send Notification| NotificationService
        TaskService -->|Execute| Tasks["Tasks (Scraping, etc.)"]
        Tasks -->|Result| NotificationService
        NotificationService -->|Push| Telegram[Telegram Bot]
    end

    Telegram -->|Message| UserDevice[User Device]
```

## 🚀 시작하기

### 전제 조건

- Docker & Docker Compose
- Go 1.23+ (로컬 개발 시)

### 설치 및 실행 (Docker)

1. **이미지 빌드**

   ```bash
   docker build -t darkkaiser/notify-server .
   ```

2. **컨테이너 실행**

   ```bash
   docker ps -q --filter name=notify-server | grep -q . && docker container stop notify-server && docker container rm notify-server

   docker run -d --name notify-server \
     -e TZ=Asia/Seoul \
     -v /usr/local/docker/notify-server:/usr/local/app \
     -v /usr/local/docker/nginx-proxy-manager/letsencrypt:/etc/letsencrypt:ro \
     -p 2443:2443 \
     --restart="always" \
     darkkaiser/notify-server
   ```

## � 개발 도구

### 코드 품질 관리

프로젝트는 다음 도구들을 사용하여 코드 품질을 관리합니다:

- **golangci-lint**: 프로젝트 맞춤형 린트 규칙 적용 (`.golangci.yml`)

  ```bash
  golangci-lint run ./...
  ```

- **Docker 빌드 최적화**: `.dockerignore`로 불필요한 파일 제외
  - 테스트 파일, 로그, IDE 설정 등 제외
  - 빌드 속도 향상 및 이미지 크기 감소

### 빌드 메타데이터

빌드된 바이너리와 Docker 이미지에는 다음 정보가 포함됩니다:

- Git 커밋 해시
- 빌드 날짜 및 시간
- 빌드 번호

실행 중인 애플리케이션에서 확인:

```bash
# 로그에서 빌드 정보 확인
docker logs notify-server | grep "빌드 정보"

# Docker 이미지 레이블 확인
docker inspect darkkaiser/notify-server:latest | grep -A 10 Labels
```

## 🔧 CI/CD

### Jenkins 파이프라인

프로젝트는 Jenkins를 통한 자동화된 CI/CD 파이프라인을 제공합니다:

**주요 기능:**

- ✅ 환경 변수 검증 (빌드 시작 전)
- ✅ 자동 테스트 및 golangci-lint 검사
- ✅ Docker 이미지 빌드 (Git 커밋 해시로 태그)
- ✅ 자동 배포 및 컨테이너 재시작
- ✅ Telegram 알림 (성공/실패)

**빌드 결과:**

- `darkkaiser/notify-server:latest` - 최신 이미지
- `darkkaiser/notify-server:{커밋해시}` - 버전별 이미지

### 로컬 개발

```bash
# 테스트 실행
go test ./... -v

# 린트 검사
golangci-lint run ./...

# 로컬 빌드
go build -o notify-server .
```

## 📝 설정 가이드

`notify-server.json` 파일을 통해 서버 동작을 설정합니다.

```json
{
  "debug": true,
  "notifiers": {
    "default_notifier_id": "my-telegram",
    "telegrams": [
      {
        "id": "my-telegram",
        "bot_token": "YOUR_BOT_TOKEN",
        "chat_id": 123456789
      }
    ]
  },
  "notify_api": {
    "ws": {
      "listen_port": 2443,
      "tls_server": false
    }
  }
}
```

## 📚 API 문서

서버가 실행 중일 때 다음 주소에서 Swagger UI를 통해 API 문서를 확인할 수 있습니다.

- URL: `http://localhost:2443/swagger/index.html`

## 📖 Task 문서

지원하는 Task의 상세 설명과 설정 방법은 다음 문서를 참고하세요:

- [Task 상세 문서](docs/TASKS.md)

지원하는 Task 목록:

- **JDC** - 전남디지털역량교육 모니터링
- **JYIU** - 전남여수산학융합원 공지사항 및 교육 모니터링
- **KURLY** - 마켓컬리 상품 가격 모니터링
- **LOTTO** - 로또 번호 예측
- **NAVER** - 네이버 공연 정보 모니터링
- **NS** - 네이버쇼핑 가격 모니터링

## 🤝 Contributing

Contributions, issues and feature requests are welcome.<br />
Feel free to check [issues page](https://github.com/DarkKaiser/notify-server/issues) if you want to contribute.

## Author

👤 **DarkKaiser**

- Blog: [@DarkKaiser](https://www.darkkaiser.com)
- Github: [@DarkKaiser](https://github.com/DarkKaiser)
