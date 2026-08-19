#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  gazebo-web

Start the Gazebo websocket bridge for gzweb.

This is platform plumbing, not a student-facing Gazebo launch command.
Students should launch Gazebo Sim with standard commands, for example:

  gz sim -s -v 3 <world.sdf> -r
  gz sim -s -v 3 ~/dev_ws/src/my_workshop/worlds/maze.sdf -r
  turtlebot3-sim
  ros2 launch turtlebot3_gazebo turtlebot3_world.launch.py

Environment:
  GZ_LAUNCH_VERBOSITY         gz launch verbosity. Default: 3
  GZ_WEBSOCKET_LAUNCH_FILE    Launch file path.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

exec gz launch \
  -v "${GZ_LAUNCH_VERBOSITY:-3}" \
  "${GZ_WEBSOCKET_LAUNCH_FILE:-/opt/gazebo-web/websocket.gzlaunch}"
