#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  turtlebot3-sim [--force] [world]

Launch TurtleBot3 in Gazebo Sim for the web cockpit.

Worlds:
  world   TurtleBot3 obstacle world. Default.
  empty   Empty world with one TurtleBot3.
  house   TurtleBot3 house world.

Environment:
  TURTLEBOT3_MODEL    burger, waffle, or waffle_pi. Default: burger.
  TURTLEBOT3_GAZEBO_GUI
                    Set to true/1/yes/on to open Gazebo in the IceWM desktop
                    instead of the browser websocket viewer.

Examples:
  turtlebot3-sim
  turtlebot3-sim --force
  turtlebot3-sim empty
  TURTLEBOT3_GAZEBO_GUI=true turtlebot3-sim
  TURTLEBOT3_MODEL=waffle_pi turtlebot3-sim house
EOF
}

is_true() {
  case "${1:-}" in
    true|TRUE|True|1|yes|YES|Yes|on|ON|On) return 0 ;;
    *) return 1 ;;
  esac
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

force=0
if [[ "${1:-}" == "-f" || "${1:-}" == "--force" ]]; then
  force=1
  shift
fi

# Only one Gazebo server may own the gz-transport bus. A second one creates a
# duplicate world/robot that confuses the web viewer and the cmd_vel bridge.
if pgrep -f "gz sim" >/dev/null 2>&1; then
  if [[ "${force}" == "1" ]]; then
    echo "Stopping the running Gazebo simulation before relaunching..." >&2
    pkill -f "gz sim" 2>/dev/null || true
    pkill -f "parameter_bridge" 2>/dev/null || true
    pkill -f "ros2 launch turtlebot3_gazebo" 2>/dev/null || true
    sleep 1
  else
    echo "A Gazebo simulation is already running in this workspace." >&2
    echo "Stop it with Ctrl-C in its terminal, or relaunch with: turtlebot3-sim --force ${1:-}" >&2
    exit 1
  fi
fi

set +u
source /opt/ros/jazzy/setup.bash
set -u

export TURTLEBOT3_MODEL="${TURTLEBOT3_MODEL:-burger}"
gazebo_gui=false
if is_true "${TURTLEBOT3_GAZEBO_GUI:-}"; then
  gazebo_gui=true
fi

world="${1:-world}"
share_dir="$(ros2 pkg prefix turtlebot3_gazebo)/share/turtlebot3_gazebo"
case "${world}" in
  world)
    world_file="${share_dir}/worlds/turtlebot3_world.world"
    ;;
  empty)
    world_file="${share_dir}/worlds/empty_world.world"
    ;;
  house)
    world_file="${share_dir}/worlds/turtlebot3_house.world"
    ;;
  *)
    echo "Unknown TurtleBot3 world: ${world}" >&2
    usage >&2
    exit 2
    ;;
esac

exec ros2 launch turtlebot3_gazebo podlab_web.launch.py \
  world_file:="${world_file}" \
  gazebo_gui:="${gazebo_gui}"
