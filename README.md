# notify-server

### 설치 위치
* 라즈베리파이의 `/usr/local/notify-server/`

### 실행
* 재부팅시 자동 실행되도록 crontab에 등록   
  `@reboot sleep 20 && su - pi -c /usr/local/notify-server/notify-server.sh`
