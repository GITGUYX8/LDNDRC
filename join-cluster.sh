#!/bin/bash
# Auto-discovery agent for the distributed ROS2 platform.
#
# Profiles this laptop's hardware and decides whether it should join the
# K3s cluster as a compute "Host" or remain a thin "Guest" client that
# only uses a web browser to reach the platform.
#
# Host criteria (>= 8 cores, >= 16 GB RAM, NVIDIA GPU) must all be met;
# falling below any single requirement routes the laptop to Guest mode.
#
# The TEST_* variables exist so the detection logic can be validated on a
# single machine (see test-join.sh) without real hardware variation.
set -euo pipefail

MASTER_IP="${MASTER_IP:-192.168.1.50}"
NODE_TOKEN="${NODE_TOKEN:-YOUR_K3S_NODE_TOKEN}"

# --- Hardware detection -------------------------------------------------
# Values are read from the live system unless overridden for testing:
#   TEST_CPU=<cores>        TEST_RAM=<GB>        TEST_GPU=nvidia|none
CPU="${TEST_CPU:-$(nproc)}"
if [ -n "$TEST_RAM" ]; then
    RAM_GB="$TEST_RAM"
else
    # /proc/meminfo reports kilobytes; divide by 1024 twice to get GB.
    RAM_GB=$(($(grep MemTotal /proc/meminfo | awk '{print $2}') / 1024 / 1024))
fi
if [ -n "$TEST_GPU" ]; then
    HAS_GPU=1
    [ "$TEST_GPU" = "none" ] && HAS_GPU=0
else
    HAS_GPU=$(lspci | grep -i -E 'vga|3d|nvidia' | wc -l)
fi

echo "[join-cluster] Detected: CPU=${CPU} cores, RAM=${RAM_GB}GB, GPU=$( [ "$HAS_GPU" -ge 1 ] && echo yes || echo no )"

# --- Decision ------------------------------------------------------------
# Every criterion must pass; the log above is what test-join.sh greps for.
if [ "$CPU" -ge 8 ] && [ "$RAM_GB" -ge 16 ] && [ "$HAS_GPU" -ge 1 ]; then
    echo "[join-cluster] High-end laptop detected. Joining as HOST."
    # The installer reads K3S_URL/K3S_TOKEN from the environment and the
    # --node-label marks this node so simulation pods can be scheduled
    # onto it via nodeSelector.
    curl -sfL https://get.k3s.io | K3S_URL="https://${MASTER_IP}:6443" K3S_TOKEN="${NODE_TOKEN}" \
        sh -s - agent --node-label "node-role.kubernetes.io/role=host"
else
    echo "[join-cluster] Low-end laptop detected. Do not join cluster. Use browser to access platform."
    exit 0
fi