#!/usr/bin/env python3
"""Turtlesim pose subscriber starter template."""

import rclpy
from rclpy.node import Node

# TODO: Import Pose when you are ready to read turtle position.
# from turtlesim.msg import Pose


class TurtlesimPoseTemplate(Node):
    def __init__(self):
        super().__init__("turtlesim_pose_template")

        # TODO: Subscribe to /turtle1/pose.
        # self.subscription = self.create_subscription(
        #     Pose,
        #     "/turtle1/pose",
        #     self.handle_pose,
        #     10,
        # )

    def handle_pose(self, message):
        # TODO: Read x, y, theta, linear_velocity, or angular_velocity.
        self.get_logger().info("TODO: handle turtlesim pose")


def main(args=None):
    rclpy.init(args=args)
    node = TurtlesimPoseTemplate()
    rclpy.spin(node)
    node.destroy_node()
    rclpy.shutdown()


if __name__ == "__main__":
    main()
