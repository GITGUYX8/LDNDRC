# Turtlesim Template

Use this folder for beginner ROS 2 experiments with turtlesim.

Start turtlesim in one terminal:

```bash
ros2 run turtlesim turtlesim_node
```

Then run your node from another terminal after building and sourcing your workspace.

Common topics:

- `/turtle1/cmd_vel`: velocity commands for the turtle.
- `/turtle1/pose`: pose feedback from the turtle.

The template scripts contain TODOs only. The runnable circle example is in `../examples/turtlesim-circle-demo.py`.
