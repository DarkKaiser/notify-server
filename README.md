# NotifyServer

<a href="https://github.com/DarkKaiser/notify-server/LICENSE">
  <img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-yellow.svg" target="_blank" />
</a>

등록된 태스크(네이버 신규 공연정보, 네이버 쇼핑)들을 지정된 시간에 실행하고, 실행 결과 데이터를 Notifier를 통하여 사용자에게 텔레그램으로 알립니다.

또한 등록된 외부 프로그램으로부터 수신된 메시지를 받아서 사용자에게 텔레그램으로 알립니다.

## Build

```bash
docker build -t darkkaiser/notify-server .
```

## Run

```bash
docker ps -q --filter name=notify-server | grep -q . && docker container stop notify-server && docker container rm notify-server

docker run -d --name notify-server \
              -e TZ=Asia/Seoul \
              -v /usr/local/docker/notify-server:/usr/local/app \
              -v /etc/letsencrypt/:/etc/letsencrypt/ \
              -p 2443:2443 \
              --restart="always" \
              darkkaiser/notify-server
```

## 🤝 Contributing

Contributions, issues and feature requests are welcome.<br />
Feel free to check [issues page](https://github.com/DarkKaiser/notify-server/issues) if you want to contribute.

## Author

👤 **DarkKaiser**

- Blog: [@DarkKaiser](http://www.darkkaiser.com)
- Github: [@DarkKaiser](https://github.com/DarkKaiser)
