#!/bin/bash
set -e

APP_PATH=/usr/local/app/
APP_CONFIG_FILE=/usr/local/app/notify-server.json

LATEST_APP_BIN_FILE=/docker-entrypoint/dist/notify-server
LATEST_APP_CONFIG_FILE=/docker-entrypoint/dist/notify-server.json

if [ -f "$LATEST_APP_BIN_FILE" ]; then
  mv -f $LATEST_APP_BIN_FILE $APP_PATH
fi

if [ -f "$LATEST_APP_CONFIG_FILE" ]; then
  mv -f $LATEST_APP_CONFIG_FILE $APP_PATH
  chown +1000:staff APP_CONFIG_FILE
fi

exec "$@"
