#!/usr/bin/env bash
set -euo pipefail

# LDNDRC pod entrypoint.
#
# Derives this pod's ROS_DOMAIN_ID from the StatefulSet pod name
# (ros2-platform-0 -> 100, ros2-platform-1 -> 101, ...) and pins DDS to
# localhost, then hands control to init-student-home (which execs supervisord).
# Because ROS_DOMAIN_ID is exported before supervisord starts, every supervised
# process (code-server, Selkies, gazebo-web) inherits the same isolated domain.

pod_name="${POD_NAME:-}"

if [[ -z "${pod_name}" ]]; then
  # Outside a StatefulSet (local docker run / debugging) there is no ordinal.
  # Fall back to a fixed domain so the workspace still starts.
  export ROS_DOMAIN_ID="${ROS_DOMAIN_ID:-30}"
else
  ordinal="${pod_name##*-}"
  if [[ "${ordinal}" =~ ^[0-9]+$ ]]; then
    export ROS_DOMAIN_ID="$((100 + ordinal))"
    echo "entrypoint: pod ${pod_name} -> ROS_DOMAIN_ID=${ROS_DOMAIN_ID}"
  else
    export ROS_DOMAIN_ID="${ROS_DOMAIN_ID:-30}"
  fi
fi

# Defense-in-depth alongside the unique domain: ROS2 discovery traffic stays on
# localhost only, so topics can never cross pod network namespaces.
export ROS_AUTOMATIC_DISCOVERY_RANGE="${ROS_AUTOMATIC_DISCOVERY_RANGE:-LOCALHOST}"

exec /usr/local/bin/init-student-home "$@"
