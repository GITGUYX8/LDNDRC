#!/usr/bin/env python3
"""ROS 2 subscriber starter template.

Copy this file into your package and complete the TODOs.
"""

import rclpy
from rclpy.node import Node

# TODO: Import the message type you want to subscribe to.
# Example:
# from std_msgs.msg import String


class TemplateSubscriber(Node):
    def __init__(self):
        super().__init__("template_subscriber")

        # TODO: Replace the message type and topic name.
        # self.subscription = self.create_subscription(
        #     String,
        #     "topic_name",
        #     self.handle_message,
        #     10,
        # )

    def handle_message(self, message):
        # TODO: Read the incoming message and do something useful.
        self.get_logger().info("TODO: handle a received message")


def main(args=None):
    rclpy.init(args=args)
    node = TemplateSubscriber()
    rclpy.spin(node)
    node.destroy_node()
    rclpy.shutdown()


if __name__ == "__main__":
    main()
