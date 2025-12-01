# Task 상세 문서

NotifyServer가 지원하는 Task들에 대한 상세 설정 가이드입니다.

## 목차

- [Task 목록](#task-목록)
- [JDC - 전남디지털역량교육](#jdc---전남디지털역량교육)
- [JYIU - 전남여수산학융합원](#jyiu---전남여수산학융합원)
- [KURLY - 마켓컬리](#kurly---마켓컬리)
- [LOTTO - 로또 번호 예측](#lotto---로또-번호-예측)
- [NAVER - 네이버 공연정보](#naver---네이버-공연정보)
- [NS - 네이버쇼핑](#ns---네이버쇼핑)
- [공통 설정](#공통-설정)
- [전체 설정 예시](#전체-설정-예시)

## Task 목록

| Task ID | 설명               | 주요 기능                          | 웹사이트                    |
| ------- | ------------------ | ---------------------------------- | --------------------------- |
| JDC     | 전남디지털역량교육 | 신규 온라인 교육 과정 모니터링     | http://전남디지털역량.com/  |
| JYIU    | 전남여수산학융합원 | 공지사항 및 교육 프로그램 모니터링 | https://www.jyiu.or.kr/     |
| KURLY   | 마켓컬리           | 상품 가격 변동 추적                | https://www.kurly.com/      |
| LOTTO   | 로또 번호 예측     | 외부 Java 프로그램 실행            | -                           |
| NAVER   | 네이버 공연정보    | 공연 정보 검색 및 알림             | https://search.naver.com/   |
| NS      | 네이버쇼핑         | 상품 최저가 모니터링               | https://shopping.naver.com/ |

---

## JDC - 전남디지털역량교육

**웹사이트**: http://전남디지털역량.com/

### 설명

신규 비대면 온라인 특별/정규교육 과정을 모니터링하고 알림을 전송합니다.

### Commands

#### WatchNewOnlineEducation

신규 온라인 교육 과정을 확인합니다.

**설정 예시**

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

**설정 옵션**

| 옵션          | 설명                    | 필수   | 기본값         |
| ------------- | ----------------------- | ------ | -------------- |
| `cron`        | 실행 주기 (Cron 표현식) | 예     | -              |
| `notifier_id` | 알림 채널 ID            | 아니오 | 기본 알림 채널 |

**알림 내용**

- 교육 과정명
- 교육 기간
- 신청 링크

---

## JYIU - 전남여수산학융합원

**웹사이트**: https://www.jyiu.or.kr/

### 설명

전남여수산학융합원의 공지사항 및 신규 교육 프로그램을 모니터링합니다.

### Commands

#### WatchNewNotice

공지사항의 새 글을 확인합니다.

**설정 예시**

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

**권장 설정**: 2시간마다 확인 (`0 */2 * * *`)

#### WatchNewEducation

신규 교육 프로그램을 확인합니다.

**설정 예시**

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

**권장 설정**: 매일 오전 10시 (`0 10 * * *`)

**알림 내용**

- 교육 프로그램명
- 교육 기간
- 접수 기간
- 신청 링크

---

## KURLY - 마켓컬리

**웹사이트**: https://www.kurly.com/

### 설명

마켓컬리 상품의 가격 변동을 모니터링합니다.

### Commands

#### WatchProductPrice

지정한 상품의 가격을 확인하고 변동 시 알림을 전송합니다.

**설정 예시**

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

**설정 옵션**

| 옵션                  | 설명                           | 필수 | 예시                                |
| --------------------- | ------------------------------ | ---- | ----------------------------------- |
| `watch_products_file` | 감시할 상품 목록 CSV 파일 경로 | 예   | `/usr/local/app/kurly_products.csv` |
| `cron`                | 가격 확인 주기                 | 예   | `0 8,20 * * *`                      |

**감시 상품 파일 형식 (CSV)**

```csv
상품코드,상품명,감시여부
1234567,유기농 우유,1
2345678,신선한 계란,1
3456789,프리미엄 쌀,0
```

**필드 설명**

- `상품코드`: 마켓컬리 상품 URL의 상품 번호 (예: `https://www.kurly.com/goods/1234567`)
- `상품명`: 상품 이름 (참고용)
- `감시여부`: `1` (활성), `0` (비활성)

**알림 내용**

- 상품명
- 현재 가격
- 이전 가격 (변동 시)
- 가격 변동률

---

## LOTTO - 로또 번호 예측

### 설명

외부 Java 프로그램을 실행하여 로또 번호를 예측합니다.

### Commands

#### Prediction

로또 당첨 번호를 예측합니다.

**설정 예시**

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

**설정 옵션**

| 옵션       | 설명                                      | 필수 |
| ---------- | ----------------------------------------- | ---- |
| `app_path` | 로또 예측 Java 애플리케이션 디렉토리 경로 | 예   |

**권장 설정**: 매주 토요일 오전 10시 (`0 10 * * 6`)

**알림 내용**

- 예측된 당첨번호 5개

---

## NAVER - 네이버 공연정보

**웹사이트**: https://search.naver.com/

### 설명

네이버에서 특정 키워드의 공연 정보를 검색하고 신규 공연을 알립니다.

### Commands

#### WatchNewPerformances

신규 공연 정보를 확인합니다.

**설정 예시**

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

**설정 옵션**

| 옵션                              | 설명                                      | 필수   |
| --------------------------------- | ----------------------------------------- | ------ |
| `query`                           | 검색 키워드                               | 예     |
| `filters.title.included_keywords` | 제목에 포함되어야 할 키워드 (쉼표로 구분) | 아니오 |
| `filters.title.excluded_keywords` | 제목에서 제외할 키워드 (쉼표로 구분)      | 아니오 |
| `filters.place.included_keywords` | 장소에 포함되어야 할 키워드               | 아니오 |
| `filters.place.excluded_keywords` | 장소에서 제외할 키워드                    | 아니오 |

**알림 내용**

- 공연명
- 공연 장소
- 공연 기간
- 예매 링크

---

## NS - 네이버쇼핑

**웹사이트**: https://shopping.naver.com/

### 설명

네이버쇼핑 API를 사용하여 상품 가격을 모니터링합니다.

### Commands

#### WatchPrice\_{상품명}

특정 상품의 최저가를 확인합니다.

**설정 예시**

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

**Task 레벨 설정**

| 옵션            | 설명                                          | 필수 |
| --------------- | --------------------------------------------- | ---- |
| `client_id`     | 네이버 개발자 센터에서 발급받은 Client ID     | 예   |
| `client_secret` | 네이버 개발자 센터에서 발급받은 Client Secret | 예   |

**Command 레벨 설정**

| 옵션                        | 설명                               | 필수   |
| --------------------------- | ---------------------------------- | ------ |
| `query`                     | 검색 키워드                        | 예     |
| `filters.included_keywords` | 포함되어야 할 키워드 (쉼표로 구분) | 아니오 |
| `filters.excluded_keywords` | 제외할 키워드 (쉼표로 구분)        | 아니오 |
| `filters.price_less_than`   | 최대 가격 (원)                     | 아니오 |

**알림 내용**

- 상품명
- 최저가
- 판매처
- 상품 링크

---

## 공통 설정

### Cron 표현식

모든 Task는 Cron 표현식을 사용하여 실행 스케줄을 설정합니다.

**형식**: `분 시 일 월 요일`

**자주 사용하는 패턴**

| Cron 표현식    | 설명                     | 사용 예시        |
| -------------- | ------------------------ | ---------------- |
| `0 9 * * *`    | 매일 오전 9시            | 아침 알림        |
| `0 */2 * * *`  | 2시간마다                | 정기 모니터링    |
| `0 9,18 * * *` | 매일 오전 9시와 오후 6시 | 출퇴근 시간 알림 |
| `0 10 * * 6`   | 매주 토요일 오전 10시    | 주간 리포트      |
| `*/30 * * * *` | 30분마다                 | 빈번한 체크      |

참고: [crontab.guru](https://crontab.guru/)

### Notifier 설정

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

**Telegram 봇 설정 방법**

1. [@BotFather](https://t.me/botfather)에서 새 봇 생성
2. 발급받은 `bot_token` 복사
3. [@userinfobot](https://t.me/userinfobot)에서 `chat_id` 확인
4. 설정 파일에 입력

> **주의**: `bot_token`과 `chat_id`는 민감한 정보입니다. Git에 커밋하지 마세요.

---

## 전체 설정 예시

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

## 빠른 시작

1. `notify-server.json` 파일 생성 및 작성
2. [@BotFather](https://t.me/botfather)에서 Telegram 봇 생성 및 토큰 발급
3. [@userinfobot](https://t.me/userinfobot)에서 Chat ID 확인
4. Docker 또는 로컬에서 서버 실행
5. 로그 확인하여 Task 실행 여부 확인

---

## 문제 해결

### Task가 실행되지 않는 경우

1. Cron 표현식 확인: [crontab.guru](https://crontab.guru/)에서 검증
2. 로그 확인: `docker logs notify-server | grep "Task ID"`
3. 설정 파일 검증: JSON 형식이 올바른지 확인

### 알림이 전송되지 않는 경우

1. Telegram 봇 토큰 확인: [@BotFather](https://t.me/botfather)에서 재확인
2. Chat ID 확인: [@userinfobot](https://t.me/userinfobot)에서 재확인
3. 네트워크 연결 확인: Telegram API 접근 가능 여부 확인

### 웹 스크래핑 실패

1. 웹사이트 구조 변경: HTML 구조가 변경되었는지 확인
2. User-Agent 설정: 일부 사이트는 User-Agent 검증
3. Rate Limiting: 요청 간격 조정 필요
