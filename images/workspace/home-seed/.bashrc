source /opt/ros/jazzy/setup.bash
export ROS_DOMAIN_ID="${ROS_DOMAIN_ID:-30}"
export WORKSPACE_PROMPT_NAME="${WORKSPACE_PROMPT_NAME:-ros-workspace}"
export PS1="\u@${WORKSPACE_PROMPT_NAME}:\w$ "
export HISTFILE=$HOME/.bash_history

# Turtlebot Settings
export TURTLEBOT3_MODEL="${TURTLEBOT3_MODEL:-burger}"