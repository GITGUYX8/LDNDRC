# Demo stand-in workspace image — PLUMBING TEST ONLY, not a real workspace.
#
# Purpose: let a session reach `ready` on GPU-less demo clusters so the full
# gateway path (Traefik -> cookie auth -> session Service -> workspace port)
# can be verified end to end. It proves plumbing, NOT ROS/Gazebo/code-server.
#
# The session readiness probe is `httpGet: path /, port editor (7682)`, so
# this server answers 200 on / and identifies itself in every response.
# Ports 8080/9002 intentionally have nothing listening: gateway traffic to
# desktop/gazebo hosts should fail with 502, which is the correct stand-in
# behavior.
FROM python:3.14-alpine

WORKDIR /srv

COPY server.py /srv/server.py

EXPOSE 7682

CMD ["python3", "/srv/server.py"]
