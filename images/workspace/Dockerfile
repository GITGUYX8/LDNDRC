# LDNDRC workspace image: layers browser tools (code-server editor, Selkies
# desktop streaming, Gazebo websocket bridge) on top of the LDNDRC base image.
# Supervisord keeps the three services running and restarts them on crash.
FROM ldndrc/ros2-gz:jazzy-harmonic-base

ARG USERNAME=student
ARG USER_UID=1000
ARG USER_GID=1000
ARG SELKIES_VERSION=1.6.2
ARG UBUNTU_VERSION=24.04
ARG SELKIES_ARCH=amd64

USER root

COPY installs.sh /tmp/installs.sh

RUN chmod +x /tmp/installs.sh \
    && /tmp/installs.sh \
    && rm -f /tmp/installs.sh
RUN curl -fsSL https://code-server.dev/install.sh | bash

COPY install-selkies.sh /tmp/install-selkies.sh

RUN chmod +x /tmp/install-selkies.sh \
    && SELKIES_VERSION=${SELKIES_VERSION} \
       UBUNTU_VERSION=${UBUNTU_VERSION} \
       SELKIES_ARCH=${SELKIES_ARCH} \
       /tmp/install-selkies.sh \
    && rm -f /tmp/install-selkies.sh

ENV CODE_SERVER_PORT=7682
ENV DISPLAY=:50
ENV SELKIES_DISPLAY=:50
ENV SELKIES_PORT=8080
ENV SELKIES_ENCODER=auto
ENV SELKIES_VIDEO_BITRATE=3000
ENV SELKIES_FRAMERATE=30
# Keep a mono audio track for WebRTC negotiation; 6000 bps is the Opus floor.
ENV SELKIES_AUDIO_BITRATE=6000
ENV SELKIES_AUDIO_CHANNELS=1
ENV SELKIES_ENABLE_RESIZE=true
ENV SELKIES_SCREEN=1440x900x24
ENV SELKIES_WEB_ROOT=/opt/gst-web
ENV SELKIES_VENV=/opt/selkies-venv
ENV ICEWM_PREFERENCES=/usr/local/share/podlab/icewm-preferences
ENV ICEWM_PREFOVERRIDE=/usr/local/share/podlab/icewm-prefoverride
ENV ICEWM_WALLPAPER=/usr/local/share/podlab/wallpaper.jpg
ENV ICEWM_THEME=IceMint\ Blue/default.theme
ENV TZ=Asia/Kolkata
ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=graphics,utility,compute,video,display
ENV VGL_DISPLAY=egl

COPY websocket.gzlaunch /opt/gazebo-web/websocket.gzlaunch
COPY websocket.gzlaunch /usr/local/share/gazebo-web/websocket.gzlaunch
COPY launch/podlab_turtlebot3_web.launch.py /opt/ros/jazzy/share/turtlebot3_gazebo/launch/podlab_web.launch.py
COPY launch/podlab_turtlebot3_web.launch.py /opt/ros/jazzy/share/turtlebot3_gazebo/launch/turtlebot3_world.launch.py
COPY launch/podlab_turtlebot3_web.launch.py /opt/ros/jazzy/share/turtlebot3_gazebo/launch/empty_world.launch.py
COPY launch/podlab_turtlebot3_web.launch.py /opt/ros/jazzy/share/turtlebot3_gazebo/launch/turtlebot3_house.launch.py
COPY supervisord.conf /etc/supervisor/conf.d/workspace-minimal.conf
COPY tmux.conf /etc/tmux.conf
COPY bin/gazebo-web.sh /usr/local/bin/gazebo-web
COPY bin/entrypoint.sh /usr/local/bin/entrypoint
COPY bin/start-code-server.sh /usr/local/bin/start-code-server
COPY bin/start-gazebo-web.sh /usr/local/bin/start-gazebo-web
COPY bin/run-with-virtualgl.sh /usr/local/bin/run-with-virtualgl
COPY bin/init-student-home.sh /usr/local/bin/init-student-home
COPY bin/reset-shell-config.sh /usr/local/bin/reset-shell-config
COPY bin/turtlebot3-sim.sh /usr/local/bin/turtlebot3-sim
COPY bin/turtlebot3-teleop.sh /usr/local/bin/turtlebot3-teleop
COPY bin/start-selkies.sh /usr/local/bin/start-selkies
COPY desktop/wallpaper.jpg /usr/local/share/podlab/wallpaper.jpg
COPY desktop/icewm-preferences /usr/local/share/podlab/icewm-preferences
COPY desktop/icewm-prefoverride /usr/local/share/podlab/icewm-prefoverride
COPY ["desktop/IceMint Blue/", "/usr/share/icewm/themes/IceMint Blue/"]
COPY home-seed/ /opt/student-home-seed/


RUN mkdir -p /home/${USERNAME} \
    && chmod +x /usr/local/bin/entrypoint /usr/local/bin/gazebo-web /usr/local/bin/start-code-server /usr/local/bin/start-gazebo-web /usr/local/bin/run-with-virtualgl /usr/local/bin/init-student-home /usr/local/bin/reset-shell-config /usr/local/bin/start-selkies /usr/local/bin/turtlebot3-sim /usr/local/bin/turtlebot3-teleop \
    && chown -R ${USER_UID}:${USER_GID} /home/${USERNAME} /opt/gazebo-web /usr/local/share/gazebo-web /opt/student-home-seed

EXPOSE 7682 8080 9002

USER ${USERNAME}
WORKDIR /home/${USERNAME}/
CMD ["/usr/local/bin/entrypoint", "/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/workspace-minimal.conf"]
