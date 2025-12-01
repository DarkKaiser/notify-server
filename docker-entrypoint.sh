#!/bin/bash
set -euo pipefail

APP_PATH=/usr/local/app/
APP_CONFIG_FILE=/usr/local/app/notify-server.json

LATEST_APP_BIN_FILE=/docker-entrypoint/dist/notify-server
LATEST_APP_CONFIG_FILE=/docker-entrypoint/dist/notify-server.json

echo "Docker entrypoint 스크립트 시작..."

# 기존 파일/디렉토리 권한 수정 (root → appuser)
echo "기존 파일 권한 확인 및 수정 중..."
if [ -d "$APP_PATH" ]; then
  # logs 디렉토리가 있으면 권한 변경
  if [ -d "${APP_PATH}logs" ]; then
    echo "  - logs 디렉토리 권한 변경"
    chown -R appuser:appuser "${APP_PATH}logs" || true
  fi
  
  # 설정 파일이 있으면 권한 변경
  if [ -f "$APP_CONFIG_FILE" ]; then
    echo "  - 설정 파일 권한 변경"
    chown appuser:appuser "$APP_CONFIG_FILE" || true
  fi
  
  # 기타 파일들 권한 변경
  chown -R appuser:appuser "$APP_PATH" || true
fi

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
