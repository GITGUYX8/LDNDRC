#!/usr/bin/env bash
set -euo pipefail

UV_INSTALL_URL="${UV_INSTALL_URL:-https://astral.sh/uv/install.sh}"

export DEBIAN_FRONTEND=noninteractive

APT_PACKAGES=(
  acl
  attr
  bash-completion
  bat
  bc
  bzip2
  build-essential
  ca-certificates
  cargo
  clang
  clangd
  cmake
  coreutils
  curl
  diffutils
  dnsutils
  fd-find
  file
  findutils
  gawk
  gdb
  git
  git-lfs
  gnupg
  grep
  gzip
  htop
  iproute2
  iputils-ping
  jq
  less
  libboost-system-dev
  libglib2.0-dev
  libgnutls28-dev
  libgstreamer-plugins-base1.0-dev
  libqt5core5a
  libqt5widgets5
  libtiff-dev
  libudev-dev
  libxml2-utils
  locales
  lsof
  lld
  lldb
  man-db
  manpages
  manpages-dev
  nano
  net-tools
  netcat-openbsd
  ninja-build
  ncurses-term
  openssh-client
  openssl
  patch
  pkg-config
  procps
  psmisc
  pipx
  pybind11-dev
  python3-argcomplete
  python3-dev
  python3-jinja2
  python3-pip
  python3-ply
  python3-venv
  python3-yaml
  python3-colcon-common-extensions
  qtbase5-dev
  ripgrep
  ros-jazzy-desktop
  ros-jazzy-cartographer
  ros-jazzy-cartographer-ros
  ros-jazzy-camera-ros
  ros-jazzy-coin-d4-driver
  ros-jazzy-dynamixel-sdk
  ros-jazzy-hls-lfcd-lds-driver
  ros-jazzy-ld08-driver
  ros-jazzy-navigation2
  ros-jazzy-nav2-bringup
  ros-jazzy-nav2-route
  ros-jazzy-ros-gz
  ros-jazzy-ros-gz-bridge
  ros-jazzy-ros-gz-image
  ros-jazzy-ros-gz-sim
  ros-jazzy-turtlebot3
  ros-jazzy-turtlebot3-gazebo
  ros-jazzy-turtlebot3-msgs
  ros-jazzy-turtlebot3-simulations
  ros-jazzy-urdf
  ros-jazzy-xacro
  rsync
  rustc
  screen
  sed
  shellcheck
  sqlite3
  strace
  tar
  time
  tmux
  tree
  tzdata
  udev
  unzip
  valgrind
  vim-tiny
  wget
  xz-utils
  zip
  zstd
)

apt-get update
apt-get install -y --no-install-recommends "${APT_PACKAGES[@]}"
locale-gen en_US.UTF-8

if command -v batcat >/dev/null 2>&1 && ! command -v bat >/dev/null 2>&1; then
  ln -s /usr/bin/batcat /usr/local/bin/bat
fi

if command -v fdfind >/dev/null 2>&1 && ! command -v fd >/dev/null 2>&1; then
  ln -s /usr/bin/fdfind /usr/local/bin/fd
fi

curl -LsSf "${UV_INSTALL_URL}" | UV_INSTALL_DIR=/usr/local/bin sh

apt-get clean
rm -rf /var/lib/apt/lists/*
