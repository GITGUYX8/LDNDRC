#!/usr/bin/env python3
"""Publish a simple circular velocity command for turtlesim."""

import rclpy
from geometry_msgs.msg import Twist
from rclpy.node import Node


class TurtlesimCircleDemo(Node):
    def __init__(self):
        super().__init__("turtlesim_circle_demo")
        self.publisher = self.create_publisher(Twist, "/turtle1/cmd_vel", 10)
        self.timer = self.create_timer(0.1, self.publish_command)

    def publish_command(self):
        command = Twist()
        command.linear.x = 2.0
        command.angular.z = 1.0
        self.publisher.publish(command)


def main(args=None):
    rclpy.init(args=args)
    node = TurtlesimCircleDemo()
    rclpy.spin(node)
    node.destroy_node()
    rclpy.shutdown()


if __name__ == "__main__":
    main()
