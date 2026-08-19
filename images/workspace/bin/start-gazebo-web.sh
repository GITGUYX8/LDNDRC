#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f /opt/gazebo-web/websocket.gzlaunch ]]; then
  cp /usr/local/share/gazebo-web/websocket.gzlaunch /opt/gazebo-web/websocket.gzlaunch
fi

exec gazebo-web
