#!/bin/bash
# Script to start containerd and set up environment for nerdctl

echo "🚀 Starting containerd..."

# Check if containerd is already running
if sudo test -S /run/containerd/containerd.sock; then
    echo "✅ containerd is already running"
else
    echo "⏳ Starting containerd service..."
    sudo containerd &
    sleep 3
    if sudo test -S /run/containerd/containerd.sock; then
        echo "✅ containerd started successfully"
    else
        echo "❌ Failed to start containerd"
        exit 1
    fi
fi

# Create convenient alias
echo "📝 Creating nerdctl alias..."
alias nerdctl='/home/runner/app/nerdctl-wrapper.sh'

echo ""
echo "🎉 Setup complete! You can now use:"
echo "   ./nerdctl-wrapper.sh version"
echo "   ./nerdctl-wrapper.sh images"
echo "   ./nerdctl-wrapper.sh ps"
echo ""
echo "Or source this script to get the alias:"
echo "   source /home/runner/app/start-containerd.sh"