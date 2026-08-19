#!/usr/bin/env bash
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

APT_PACKAGES=(
  ca-certificates
  curl
  gnupg
  ros-jazzy-gz-launch-vendor
  ros-jazzy-turtlebot3
  ros-jazzy-turtlebot3-gazebo
  ros-jazzy-turtlebot3-teleop
  supervisor
  tmux
)

apt-get update
apt-get install -y --no-install-recommends "${APT_PACKAGES[@]}"
curl -fsSL https://deb.nodesource.com/setup_24.x | bash -
apt-get install -y --no-install-recommends nodejs

mkdir -p /opt/gazebo-web
npm --prefix /opt/gazebo-web install gzweb

# Apply only non-breaking npm audit fixes. Some gzweb transitive fixes currently
# require `--force`, which can downgrade/break the Gazebo web runtime.
npm --prefix /opt/gazebo-web audit fix --omit=dev \
  || echo "Continuing: non-breaking npm audit fix could not resolve all gzweb advisories."
npm --prefix /opt/gazebo-web audit --omit=dev --audit-level=high \
  || echo "Continuing: gzweb still has upstream npm audit findings; do not use npm audit fix --force without testing Gazebo web."

npm cache clean --force
apt-get clean
rm -rf /var/lib/apt/lists/* /root/.npm
