#!/bin/bash

# todo.sh - A simple CLI todo app
# Usage: ./todo.sh add 'task description'

# File to store todos (in the same directory as the script)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TODO_FILE="$SCRIPT_DIR/todos.txt"

# Show usage information
usage() {
    echo "Usage: ./todo.sh add 'task description'"
    echo ""
    echo "Commands:"
    echo "  add <task>  Add a new todo item"
    exit 1
}

# Add a todo item
add_todo() {
    local task="$1"

    if [ -z "$task" ]; then
        echo "Error: No task provided"
        usage
    fi

    # Append the task to the file with a newline
    echo "$task" >> "$TODO_FILE"
    echo "Added: $task"
}

# Main script logic
main() {
    # Check if any arguments provided
    if [ $# -eq 0 ]; then
        usage
    fi

    local command="$1"
    shift

    case "$command" in
        add)
            add_todo "$1"
            ;;
        *)
            echo "Error: Unknown command '$command'"
            usage
            ;;
    esac
}

main "$@"
