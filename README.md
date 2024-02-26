# NotifyServer

<p>
  <img src="https://img.shields.io/badge/Go-00ADD8?style=flat&logo=Go&logoColor=white" />
  <img src="https://img.shields.io/badge/jenkins-%232C5263.svg?style=flat&logo=jenkins&logoColor=white">
  <img src="https://img.shields.io/badge/Docker-2496ED?style=flat&logo=Docker&logoColor=white">
  <img src="https://img.shields.io/badge/Linux-FCC624?style=flat&logo=linux&logoColor=black">
  <a href="https://github.com/DarkKaiser/notify-server/blob/master/LICENSE">
    <img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-yellow.svg" target="_blank" />
  </a>
</p>

외부 프로그램으로부터 수신된 메시지 및 등록된 태스크들의 실행 결과를 알립니다.

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
              -v /usr/local/docker/nginx-proxy-manager/letsencrypt:/etc/letsencrypt:ro \
              -p 2443:2443 \
              --restart="always" \
              darkkaiser/notify-server
```

## 🤝 Contributing

Contributions, issues and feature requests are welcome.<br />
Feel free to check [issues page](https://github.com/DarkKaiser/notify-server/issues) if you want to contribute.

## Author

👤 **DarkKaiser**

- Blog: [@DarkKaiser](https://www.darkkaiser.com)
- Github: [@DarkKaiser](https://github.com/DarkKaiser)
