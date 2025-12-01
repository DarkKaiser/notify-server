#!/bin/bash
set -euo pipefail

APP_PATH=/usr/local/app/
APP_CONFIG_FILE=/usr/local/app/notify-server.json

LATEST_APP_BIN_FILE=/docker-entrypoint/dist/notify-server
LATEST_APP_CONFIG_FILE=/docker-entrypoint/dist/notify-server.json

echo "Docker entrypoint 스크립트 시작..."

if [ -f "$LATEST_APP_BIN_FILE" ]; then
  echo "애플리케이션 바이너리를 $APP_PATH 로 이동 중..."
  mv -f "$LATEST_APP_BIN_FILE" "$APP_PATH" || {
    echo "ERROR: 바이너리 파일 이동 실패"
    exit 1
  }
  echo "바이너리 이동 완료"
fi

if [ -f "$LATEST_APP_CONFIG_FILE" ]; then
  echo "설정 파일을 $APP_PATH 로 이동 중..."
  mv -f "$LATEST_APP_CONFIG_FILE" "$APP_PATH" || {
    echo "ERROR: 설정 파일 이동 실패"
    exit 1
  }
  echo "설정 파일 이동 완료"
fi

echo "애플리케이션 시작: $@"
exec "$@"
