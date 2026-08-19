#!/usr/bin/env python3
"""TurtleBot3 /odom subscriber starter template."""

import rclpy
from rclpy.node import Node

# TODO: Import Odometry when you are ready to read robot pose/velocity.
# from nav_msgs.msg import Odometry


class TurtleBot3OdomTemplate(Node):
    def __init__(self):
        super().__init__("turtlebot3_odom_template")

        # TODO: Subscribe to /odom.
        # self.subscription = self.create_subscription(
        #     Odometry,
        #     "/odom",
        #     self.handle_odom,
        #     10,
        # )

    def handle_odom(self, message):
        # TODO: Read pose or twist values from the odometry message.
        self.get_logger().info("TODO: handle TurtleBot3 odometry")


def main(args=None):
    rclpy.init(args=args)
    node = TurtleBot3OdomTemplate()
    rclpy.spin(node)
    node.destroy_node()
    rclpy.shutdown()


if __name__ == "__main__":
    main()
