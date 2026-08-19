#!/usr/bin/env python3
"""ROS 2 publisher starter template.

Copy this file into your package and complete the TODOs.
"""

import rclpy
from rclpy.node import Node

# TODO: Import the message type you want to publish.
# Example:
# from std_msgs.msg import String


class TemplatePublisher(Node):
    def __init__(self):
        super().__init__("template_publisher")

        # TODO: Replace the message type and topic name.
        # self.publisher = self.create_publisher(String, "topic_name", 10)

        # TODO: Choose a timer interval in seconds.
        self.timer = self.create_timer(1.0, self.publish_message)

    def publish_message(self):
        # TODO: Create and publish your message here.
        # message = String()
        # message.data = "replace this with your data"
        # self.publisher.publish(message)
        self.get_logger().info("TODO: publish a message")


def main(args=None):
    rclpy.init(args=args)
    node = TemplatePublisher()
    rclpy.spin(node)
    node.destroy_node()
    rclpy.shutdown()


if __name__ == "__main__":
    main()
