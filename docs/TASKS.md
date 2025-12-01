# 📋 Task 상세 문서

NotifyServer는 다양한 웹 스크래핑 및 자동화 작업을 지원합니다. 각 Task는 특정 웹사이트나 서비스를 모니터링하고, 변경사항이 발생하면 알림을 전송합니다.

## 📑 목차

- [Task 목록 요약](#task-목록-요약)
- [JDC - 전남디지털역량교육](#jdc---전남디지털역량교육)
- [JYIU - 전남여수산학융합원](#jyiu---전남여수산학융합원)
- [KURLY - 마켓컬리](#kurly---마켓컬리)
- [LOTTO - 로또 번호 예측](#lotto---로또-번호-예측)
- [NAVER - 네이버 공연정보](#naver---네이버-공연정보)
- [NS - 네이버쇼핑](#ns---네이버쇼핑)
- [공통 설정](#-공통-설정)
- [전체 설정 예시](#-전체-설정-예시)

---

## 📊 Task 목록 요약

| Task ID   | 이모지 | 설명               | 주요 기능                          | 웹사이트                                          |
| --------- | ------ | ------------------ | ---------------------------------- | ------------------------------------------------- |
| **JDC**   | 🎓     | 전남디지털역량교육 | 신규 온라인 교육 과정 모니터링     | [전남디지털역량.com](http://전남디지털역량.com/)  |
| **JYIU**  | 🏫     | 전남여수산학융합원 | 공지사항 및 교육 프로그램 모니터링 | [jyiu.or.kr](https://www.jyiu.or.kr/)             |
| **KURLY** | 🛒     | 마켓컬리           | 상품 가격 변동 추적                | [kurly.com](https://www.kurly.com/)               |
| **LOTTO** | 🎰     | 로또 번호 예측     | 외부 Java 프로그램 실행            | -                                                 |
| **NAVER** | 🎭     | 네이버 공연정보    | 공연 정보 검색 및 알림             | [search.naver.com](https://search.naver.com/)     |
| **NS**    | 💰     | 네이버쇼핑         | 상품 최저가 모니터링               | [shopping.naver.com](https://shopping.naver.com/) |

> **💡 팁**: 각 Task는 Cron 표현식으로 스케줄링되며, Telegram을 통해 알림을 받을 수 있습니다.

---

## JDC - 전남디지털역량교육

**웹사이트:** http://전남디지털역량.com/

### 기능

신규 비대면 온라인 특별/정규교육 과정을 모니터링하고 알림을 전송합니다.

### Task ID

- `JDC`

### Commands

#### WatchNewOnlineEducation

신규 온라인 교육 과정을 확인합니다.

**설정 예시:**

```json
{
  "id": "JDC",
  "title": "전남디지털역량 신규 교육 확인",
  "commands": [
    {
      "id": "WatchNewOnlineEducation",
      "cron": "0 9,18 * * *",
      "notifier_id": "my-telegram"
    }
  ]
}
```

**알림 내용:**

- 교육 과정명
- 교육 기간
- 신청 링크

---

## JYIU - 전남여수산학융합원

**웹사이트:** https://www.jyiu.or.kr/

### 기능

전남여수산학융합원의 공지사항 및 신규 교육 프로그램을 모니터링합니다.

### Task ID

- `JYIU`

### Commands

#### WatchNewNotice

공지사항의 새 글을 확인합니다.

**설정 예시:**

```json
{
  "id": "JYIU",
  "title": "전남여수산학융합원 모니터링",
  "commands": [
    {
      "id": "WatchNewNotice",
      "cron": "0 */2 * * *",
      "notifier_id": "my-telegram"
    }
  ]
}
```

#### WatchNewEducation

신규 교육 프로그램을 확인합니다.

**설정 예시:**

```json
{
  "commands": [
    {
      "id": "WatchNewEducation",
      "cron": "0 10 * * *",
      "notifier_id": "my-telegram"
    }
  ]
}
```

**알림 내용:**

- 교육 프로그램명
- 교육 기간
- 접수 기간
- 신청 링크

---

## KURLY - 마켓컬리

**웹사이트:** https://www.kurly.com/

### 기능

마켓컬리 상품의 가격 변동을 모니터링합니다.

### Task ID

- `KURLY`

### Commands

#### WatchProductPrice

지정한 상품의 가격을 확인하고 변동 시 알림을 전송합니다.

**설정 예시:**

```json
{
  "id": "KURLY",
  "title": "마켓컬리 가격 모니터링",
  "commands": [
    {
      "id": "WatchProductPrice",
      "cron": "0 8,20 * * *",
      "notifier_id": "my-telegram",
      "data": {
        "watch_products_file": "/usr/local/app/kurly_products.csv"
      }
    }
  ]
}
```

**감시 상품 파일 형식 (CSV):**

```csv
상품코드,상품명,감시여부
1234567,유기농 우유,1
2345678,신선한 계란,1
3456789,프리미엄 쌀,0
```

- **상품코드**: 마켓컬리 상품 URL의 상품 번호
- **상품명**: 상품 이름 (참고용)
- **감시여부**: `1` (활성), `0` (비활성)

**알림 내용:**

- 상품명
- 현재 가격
- 이전 가격 (변동 시)
- 가격 변동률

---

## LOTTO - 로또 번호 예측

### 기능

외부 Java 프로그램을 실행하여 로또 번호를 예측합니다.

### Task ID

- `LOTTO`

### Commands

#### Prediction

로또 당첨 번호를 예측합니다.

**설정 예시:**

```json
{
  "id": "LOTTO",
  "title": "로또 번호 예측",
  "commands": [
    {
      "id": "Prediction",
      "cron": "0 10 * * 6",
      "notifier_id": "my-telegram"
    }
  ],
  "data": {
    "app_path": "/usr/local/app/lotto/"
  }
}
```

**필수 설정:**

- `app_path`: 로또 예측 Java 애플리케이션이 있는 디렉토리 경로

**알림 내용:**

- 예측된 당첨번호 5개

---

## NAVER - 네이버 공연정보

**웹사이트:** https://search.naver.com/

### 기능

네이버에서 특정 키워드의 공연 정보를 검색하고 신규 공연을 알립니다.

### Task ID

- `NAVER`

### Commands

#### WatchNewPerformances

신규 공연 정보를 확인합니다.

**설정 예시:**

```json
{
  "id": "NAVER",
  "title": "네이버 공연 모니터링",
  "commands": [
    {
      "id": "WatchNewPerformances",
      "cron": "0 9 * * *",
      "notifier_id": "my-telegram",
      "data": {
        "query": "뮤지컬",
        "filters": {
          "title": {
            "included_keywords": "오페라의 유령,레미제라블",
            "excluded_keywords": "어린이,키즈"
          },
          "place": {
            "included_keywords": "서울,경기",
            "excluded_keywords": ""
          }
        }
      }
    }
  ]
}
```

**필터 옵션:**

- `query`: 검색 키워드 (필수)
- `filters.title.included_keywords`: 제목에 포함되어야 할 키워드 (쉼표로 구분)
- `filters.title.excluded_keywords`: 제목에서 제외할 키워드 (쉼표로 구분)
- `filters.place.included_keywords`: 장소에 포함되어야 할 키워드
- `filters.place.excluded_keywords`: 장소에서 제외할 키워드

**알림 내용:**

- 공연명
- 공연 장소
- 공연 기간
- 예매 링크

---

## NS - 네이버쇼핑

**웹사이트:** https://shopping.naver.com/

### 기능

네이버쇼핑 API를 사용하여 상품 가격을 모니터링합니다.

### Task ID

- `NS`

### Commands

#### WatchPrice\_{상품명}

특정 상품의 최저가를 확인합니다.

**설정 예시:**

```json
{
  "id": "NS",
  "title": "네이버쇼핑 가격 모니터링",
  "commands": [
    {
      "id": "WatchPrice_노트북",
      "cron": "0 */6 * * *",
      "notifier_id": "my-telegram",
      "data": {
        "query": "LG 그램 17",
        "filters": {
          "included_keywords": "2024,신형",
          "excluded_keywords": "중고,리퍼",
          "price_less_than": 2000000
        }
      }
    }
  ],
  "data": {
    "client_id": "YOUR_NAVER_CLIENT_ID",
    "client_secret": "YOUR_NAVER_CLIENT_SECRET"
  }
}
```

**필수 설정:**

- `client_id`: 네이버 개발자 센터에서 발급받은 Client ID
- `client_secret`: 네이버 개발자 센터에서 발급받은 Client Secret

**Command 데이터:**

- `query`: 검색 키워드 (필수)
- `filters.included_keywords`: 포함되어야 할 키워드 (쉼표로 구분)
- `filters.excluded_keywords`: 제외할 키워드 (쉼표로 구분)
- `filters.price_less_than`: 최대 가격 (원)

**알림 내용:**

- 상품명
- 최저가
- 판매처
- 상품 링크

---

## ⚙️ 공통 설정

### ⏰ Cron 표현식

모든 Task는 Cron 표현식을 사용하여 실행 스케줄을 설정합니다.

**형식:** `분 시 일 월 요일`

#### 자주 사용하는 패턴

| Cron 표현식    | 설명                     | 사용 예시        |
| -------------- | ------------------------ | ---------------- |
| `0 9 * * *`    | 매일 오전 9시            | 아침 알림        |
| `0 */2 * * *`  | 2시간마다                | 정기 모니터링    |
| `0 9,18 * * *` | 매일 오전 9시와 오후 6시 | 출퇴근 시간 알림 |
| `0 10 * * 6`   | 매주 토요일 오전 10시    | 주간 리포트      |
| `*/30 * * * *` | 30분마다                 | 빈번한 체크      |

> **🔗 유용한 도구**: [crontab.guru](https://crontab.guru/)에서 Cron 표현식을 테스트해보세요!

### 📢 Notifier 설정

각 Command는 `notifier_id`를 지정하여 알림을 전송할 채널을 선택합니다.

```json
{
  "notifiers": {
    "default_notifier_id": "my-telegram",
    "telegrams": [
      {
        "id": "my-telegram",
        "bot_token": "YOUR_BOT_TOKEN",
        "chat_id": 123456789
      }
    ]
  }
}
```

#### Telegram 봇 설정 방법

1. **[@BotFather](https://t.me/botfather)**에서 새 봇 생성
2. 발급받은 `bot_token` 복사
3. **[@userinfobot](https://t.me/userinfobot)**에서 `chat_id` 확인
4. 설정 파일에 입력

> **⚠️ 보안 주의**: `bot_token`과 `chat_id`는 민감한 정보입니다. Git에 커밋하지 마세요!

---

## 📝 전체 설정 예시

```json
{
  "debug": false,
  "notifiers": {
    "default_notifier_id": "my-telegram",
    "telegrams": [
      {
        "id": "my-telegram",
        "bot_token": "YOUR_TELEGRAM_BOT_TOKEN",
        "chat_id": 123456789
      }
    ]
  },
  "tasks": [
    {
      "id": "JDC",
      "title": "전남디지털역량 교육 모니터링",
      "commands": [
        {
          "id": "WatchNewOnlineEducation",
          "cron": "0 9,18 * * *"
        }
      ]
    },
    {
      "id": "KURLY",
      "title": "마켓컬리 가격 모니터링",
      "commands": [
        {
          "id": "WatchProductPrice",
          "cron": "0 8,20 * * *",
          "data": {
            "watch_products_file": "/usr/local/app/kurly_products.csv"
          }
        }
      ]
    },
    {
      "id": "NS",
      "title": "네이버쇼핑 가격 모니터링",
      "commands": [
        {
          "id": "WatchPrice_노트북",
          "cron": "0 */6 * * *",
          "data": {
            "query": "LG 그램 17",
            "filters": {
              "included_keywords": "2024,신형",
              "excluded_keywords": "중고,리퍼",
              "price_less_than": 2000000
            }
          }
        }
      ],
      "data": {
        "client_id": "YOUR_NAVER_CLIENT_ID",
        "client_secret": "YOUR_NAVER_CLIENT_SECRET"
      }
    }
  ],
  "notify_api": {
    "ws": {
      "listen_port": 2443,
      "tls_server": true,
      "tls_cert_file": "/etc/letsencrypt/live/yourdomain.com/fullchain.pem",
      "tls_key_file": "/etc/letsencrypt/live/yourdomain.com/privkey.pem"
    },
    "allowed_applications": [
      {
        "id": "my-app",
        "title": "My Application",
        "app_key": "YOUR_APP_KEY",
        "default_notifier_id": "my-telegram"
      }
    ]
  }
}
```

---

## 🚀 빠른 시작

1. **설정 파일 생성**: `notify-server.json` 파일을 생성하고 위 예시를 참고하여 작성
2. **Telegram 봇 설정**: [@BotFather](https://t.me/botfather)에서 봇 생성 및 토큰 발급
3. **Chat ID 확인**: [@userinfobot](https://t.me/userinfobot)에서 Chat ID 확인
4. **서버 실행**: Docker 또는 로컬에서 서버 실행
5. **로그 확인**: 스케줄에 따라 Task가 실행되는지 확인

---

## 🛠 개발자 가이드

### 새로운 Task 추가하기

1. **Task 파일 생성**: `service/task/task_{taskname}.go` 파일 생성
2. **Task 구조체 정의**: `BaseTask`를 임베드하여 Task 구조체 정의
3. **Command 구현**: `ExecuteCommand` 메서드 구현
4. **Task 등록**: `task.go`의 `createTask` 함수에 새 Task 추가
5. **테스트 작성**: 단위 테스트, 통합 테스트, 벤치마크 테스트 작성

### 테스트 작성 가이드

```go
// 단위 테스트 예시
func TestTaskName_ExecuteCommand(t *testing.T) {
    // Given
    mockFetcher := &MockFetcher{}
    task := &TaskName{Fetcher: mockFetcher}

    // When
    result, err := task.ExecuteCommand(ctx, "CommandID", data)

    // Then
    assert.NoError(t, err)
    assert.NotNil(t, result)
}

// 통합 테스트 예시 (testdata 활용)
func TestTaskName_Integration(t *testing.T) {
    htmlContent := readTestData(t, "testdata/sample.html")
    // 실제 HTML 파싱 테스트
}

// 벤치마크 테스트 예시
func BenchmarkTaskName_Parse(b *testing.B) {
    for i := 0; i < b.N; i++ {
        parseData(htmlContent)
    }
}
```

### 디버깅 팁

```bash
# 특정 Task만 실행하여 테스트
go test -v -run TestTaskName

# 로그 레벨 조정 (notify-server.json)
{
  "debug": true  # 상세 로그 출력
}

# Docker 컨테이너 로그 실시간 확인
docker logs -f notify-server

# 특정 시간대 로그만 확인
docker logs notify-server 2>&1 | grep "2025-12-01 22:"
```

### 성능 최적화

- **HTTP 요청 재시도**: `fetcher_retry.go`의 재시도 로직 활용
- **캐싱**: 반복적인 데이터는 파일 또는 메모리에 캐싱
- **동시성**: 여러 상품/페이지 처리 시 goroutine 활용
- **벤치마크**: 성능 개선 전후 벤치마크 테스트로 검증

---

## 📚 추가 정보

- **API 문서**: [Swagger UI](https://your-domain:2443/swagger/index.html)
- **GitHub**: [notify-server](https://github.com/DarkKaiser/notify-server)
- **이슈 리포트**: [Issues](https://github.com/DarkKaiser/notify-server/issues)
- **Task 상세 문서**: 각 Task의 구현 세부사항은 소스 코드 참조

---

## 🔍 문제 해결

### Task가 실행되지 않는 경우

1. **Cron 표현식 확인**: [crontab.guru](https://crontab.guru/)에서 검증
2. **로그 확인**: `docker logs notify-server | grep "Task ID"`
3. **설정 파일 검증**: JSON 형식이 올바른지 확인

### 알림이 전송되지 않는 경우

1. **Telegram 봇 토큰 확인**: [@BotFather](https://t.me/botfather)에서 재확인
2. **Chat ID 확인**: [@userinfobot](https://t.me/userinfobot)에서 재확인
3. **네트워크 연결 확인**: Telegram API 접근 가능 여부 확인

### 웹 스크래핑 실패

1. **웹사이트 구조 변경**: HTML 구조가 변경되었는지 확인
2. **User-Agent 설정**: 일부 사이트는 User-Agent 검증
3. **Rate Limiting**: 요청 간격 조정 필요
