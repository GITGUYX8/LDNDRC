# ROS 2 Publisher / Subscriber Template

Use this as a small Python node starting point.

Suggested flow:

```bash
mkdir -p ~/dev_ws/src
cp -r ~/templates/ros2-pub-sub ~/dev_ws/src/my_pub_sub
cd ~/dev_ws
colcon build
source install/setup.bash
```

Files:

- `publisher_template.py`: fill in the message type, topic name, and publish logic.
- `subscriber_template.py`: fill in the message type, topic name, and callback logic.

The files intentionally do not perform a useful robot action yet. Students should complete the TODOs.
