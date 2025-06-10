#!/bin/bash

# Clear golang cache
# go clean -cache

# Navigate to the script's directory to ensure we are in the project root.
# This makes the script runnable from anywhere, as long as it's in the project root.
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
cd "$SCRIPT_DIR" || exit 1 # Exit if cd fails

echo "=== Building toolkit... ==="
# The -v flag will print the names of packages as they are compiled.
# Remove -v if you prefer less output.
if go build -v -o toolkit .; then
    echo "=== Build successful. ==="
    echo ""

    # Optional: If you always want to move it to /usr/local/bin
    # Make sure you have sudo privileges or run this script with sudo.
    # This step is generally done once, or when you want to update the global command.
    # If you uncomment this, be aware it will ask for your password if not run as root.
    #
    # echo "Moving toolkit to /usr/local/bin/toolkit (requires sudo)..."
    # if sudo mv toolkit /usr/local/bin/toolkit; then
    #   echo "Moved successfully."
    #   echo "Running toolkit from /usr/local/bin/toolkit..."
    #   # Run the globally installed version
    #   /usr/local/bin/toolkit "$@"
    # else
    #   echo "Failed to move toolkit to /usr/local/bin. Running local build..."
    #   ./toolkit "$@"
    # fi

    # Run the locally built toolkit executable
    echo "=== Running local build: ./toolkit $@ ==="
    ./toolkit "$@"

else
    echo ""
    echo "!!! Build failed. Please check errors above. !!!"
    exit 1
fi
