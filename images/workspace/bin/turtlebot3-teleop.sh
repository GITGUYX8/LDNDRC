#!/usr/bin/env bash
set -euo pipefail

set +u
source /opt/ros/jazzy/setup.bash
set -u
export TURTLEBOT3_MODEL="${TURTLEBOT3_MODEL:-burger}"

exec ros2 run turtlebot3_teleop teleop_keyboard
