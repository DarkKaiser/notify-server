# NotifyServer

등록된 태스크들을 지정된 시간에 실행하고, 실행 결과 데이터를 Notifier를 통하여 사용자에게 알립니다.

또한 등록된 외부 프로그램으로부터 수신된 메시지를 받아서 사용자에게 알립니다.

## 설치 경로
라즈베리파이의 아래 경로에 설치됩니다.   
`/usr/local/notify-server/`

## 실행
라즈베리파이 재부팅시 자동으로 실행되도록 crontab에 등록되어 있습니다.   
`@reboot sleep 20 && su - pi -c /usr/local/notify-server/notify-server.sh`
