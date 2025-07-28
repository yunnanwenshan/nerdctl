#!/bin/bash
# nerdctl wrapper script to use system containerd

# Set the containerd socket address
export CONTAINERD_ADDRESS="/run/containerd/containerd.sock"

# Check if containerd is running
if ! sudo test -S "$CONTAINERD_ADDRESS"; then
    echo "Error: containerd not running. Start it with: sudo containerd &"
    exit 1
fi

# Run nerdctl with sudo and proper socket address
exec sudo /home/runner/app/_output/nerdctl --address "$CONTAINERD_ADDRESS" "$@"