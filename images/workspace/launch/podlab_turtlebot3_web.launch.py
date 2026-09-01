#!/usr/bin/env python3

import os

from ament_index_python.packages import get_package_share_directory
from launch import LaunchDescription
from launch.actions import AppendEnvironmentVariable
from launch.actions import DeclareLaunchArgument
from launch.actions import IncludeLaunchDescription
from launch.actions import OpaqueFunction
from launch.launch_description_sources import PythonLaunchDescriptionSource
from launch.substitutions import LaunchConfiguration


def generate_launch_description():
    turtlebot3_gazebo = get_package_share_directory('turtlebot3_gazebo')
    ros_gz_sim = get_package_share_directory('ros_gz_sim')
    launch_file_dir = os.path.join(turtlebot3_gazebo, 'launch')
    launch_name = os.path.basename(__file__)
    default_world_name = {
        'empty_world.launch.py': 'empty_world.world',
        'turtlebot3_house.launch.py': 'turtlebot3_house.world',
    }.get(launch_name, 'turtlebot3_world.world')

    world_file = LaunchConfiguration(
        'world_file',
        default=os.path.join(turtlebot3_gazebo, 'worlds', default_world_name),
    )
    use_sim_time = LaunchConfiguration('use_sim_time', default='true')
    x_pose = LaunchConfiguration('x_pose', default='-2.0')
    y_pose = LaunchConfiguration('y_pose', default='-0.5')
    gazebo_gui = LaunchConfiguration('gazebo_gui', default='false')

    def gazebo_launch(context):
        gui_enabled = gazebo_gui.perform(context).lower() in ('1', 'true', 'yes', 'on')
        gz_args = ['-r -v2 ' if gui_enabled else '-r -s -v2 ', world_file]
        return [
            IncludeLaunchDescription(
                PythonLaunchDescriptionSource(os.path.join(ros_gz_sim, 'launch', 'gz_sim.launch.py')),
                launch_arguments={
                    'gz_args': gz_args,
                    'on_exit_shutdown': 'true',
                }.items(),
            ),
        ]

    robot_state_publisher = IncludeLaunchDescription(
        PythonLaunchDescriptionSource(os.path.join(launch_file_dir, 'robot_state_publisher.launch.py')),
        launch_arguments={'use_sim_time': use_sim_time}.items(),
    )

    spawn_turtlebot = IncludeLaunchDescription(
        PythonLaunchDescriptionSource(os.path.join(launch_file_dir, 'spawn_turtlebot3.launch.py')),
        launch_arguments={'x_pose': x_pose, 'y_pose': y_pose}.items(),
    )

    return LaunchDescription([
        DeclareLaunchArgument(
            'world_file',
            default_value=os.path.join(turtlebot3_gazebo, 'worlds', default_world_name),
        ),
        DeclareLaunchArgument('use_sim_time', default_value='true'),
        DeclareLaunchArgument('x_pose', default_value='-2.0'),
        DeclareLaunchArgument('y_pose', default_value='-0.5'),
        DeclareLaunchArgument('gazebo_gui', default_value='false'),
        AppendEnvironmentVariable(
            'GZ_SIM_RESOURCE_PATH',
            os.path.join(turtlebot3_gazebo, 'models'),
        ),
        OpaqueFunction(function=gazebo_launch),
        spawn_turtlebot,
        robot_state_publisher,
    ])
