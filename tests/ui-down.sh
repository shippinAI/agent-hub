#!/usr/bin/env bash
# Stops the demo hub started by ui-demo.sh. Touches only that process.
pkill -f "hub-server-ui" 2>/dev/null && echo "demo hub stopped" || echo "demo hub not running"
