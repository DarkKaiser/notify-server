# NotifyServer

<a href="https://github.com/DarkKaiser/notify-server/blob/master/LICENSE">
  <img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-yellow.svg" target="_blank" />
</a>

외부 프로그램으로부터 수신된 메시지 및 등록된 태스크들의 실행 결과를 알립니다.

## Build

NotifyServer의 도커 이미지를 생성합니다.

```bash
docker build -t darkkaiser/notify-server .
```

## Run

NotifyServer의 도커 컨테이너를 모두 제거하고 다시 실행합니다.

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
