# LDNDRC base image: ROS2 Jazzy desktop-full + Gazebo Harmonic + TurtleBot3.
# DDS is pinned to localhost (ROS_AUTOMATIC_DISCOVERY_RANGE=LOCALHOST) so each
# pod is a fully isolated ROS2 island; ROS_DOMAIN_ID is a fallback default that
# the workspace entrypoint overrides per-pod.
FROM osrf/ros:jazzy-desktop-full-noble

ARG USERNAME=student
ARG USER_UID=1000
ARG USER_GID=1000

ENV DEBIAN_FRONTEND=noninteractive
ENV SHELL=/bin/bash
ENV WORKSPACE_DIR=/home/${USERNAME}/dev_ws
ENV ROS_DOMAIN_ID=30
ENV TURTLEBOT3_MODEL=burger
ENV LDS_MODEL=LDS-02
ENV ROS_AUTOMATIC_DISCOVERY_RANGE=LOCALHOST

USER root

COPY installs.sh /tmp/installs.sh

RUN chmod +x /tmp/installs.sh \
    && /tmp/installs.sh \
    && rm -f /tmp/installs.sh

RUN usermod --login ${USERNAME} --home /home/${USERNAME} --move-home ubuntu \
    && groupmod --new-name ${USERNAME} ubuntu \
    && mkdir -p ${WORKSPACE_DIR} \
    && printf '\nsource /opt/ros/${ROS_DISTRO}/setup.bash\nexport ROS_DOMAIN_ID="${ROS_DOMAIN_ID:-30}"\nexport TURTLEBOT3_MODEL="${TURTLEBOT3_MODEL:-burger}"\nexport LDS_MODEL="${LDS_MODEL:-LDS-02}"\nexport WORKSPACE_PROMPT_NAME="${WORKSPACE_PROMPT_NAME:-ros-workspace}"\nexport PS1="\\u@${WORKSPACE_PROMPT_NAME}:\\w$ "\n' >> /home/${USERNAME}/.bashrc \
    && chown -R ${USER_UID}:${USER_GID} /home/${USERNAME}

USER ${USERNAME}
WORKDIR ${WORKSPACE_DIR}
CMD ["bash"]
