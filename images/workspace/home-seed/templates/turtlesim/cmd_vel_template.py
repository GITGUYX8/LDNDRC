#!/usr/bin/env python3
"""Turtlesim command starter template.

Complete the TODOs before using this to move the turtle.
"""

import rclpy
from rclpy.node import Node

# TODO: Import Twist when you are ready to publish velocity commands.
# from geometry_msgs.msg import Twist


class TurtlesimCommandTemplate(Node):
    def __init__(self):
        super().__init__("turtlesim_command_template")

        # TODO: Create a publisher for /turtle1/cmd_vel.
        # self.publisher = self.create_publisher(Twist, "/turtle1/cmd_vel", 10)

        # TODO: Choose how often your command logic should run.
        self.timer = self.create_timer(1.0, self.publish_command)

    def publish_command(self):
        # TODO: Build a Twist message and publish it.
        self.get_logger().info("TODO: publish a turtlesim velocity command")


def main(args=None):
    rclpy.init(args=args)
    node = TurtlesimCommandTemplate()
    rclpy.spin(node)
    node.destroy_node()
    rclpy.shutdown()


if __name__ == "__main__":
    main()
