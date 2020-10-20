# notify-server

### 설치 위치
* 라즈베리파이의 `/usr/local/go-workspace/src/github.com/darkkaiser/notify-server/`에 설치

### 실행
* 재부팅시 실행되도록 crontab에 등록해 놓았음!   
  `@reboot sleep 20 && su - pi -c /usr/local/go-workspace/src/github.com/darkkaiser/notify-server/notify-server.sh`
