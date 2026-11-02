#!/bin/sh
set -e

# Start the universal Go master server daemon in the background
/usr/local/bin/master &
MASTER_PID=$!
echo "[Entrypoint] Universal Master Daemon started (PID: $MASTER_PID)"

# Start the Next.js All-Seeing Eye web launcher in the background
node server.js &
NODE_PID=$!
echo "[Entrypoint] The All-Seeing Eye Web Launcher started (PID: $NODE_PID on port ${PORT:-3000})"

# Trap signals and forward to both child processes
trap 'kill -TERM $MASTER_PID 2>/dev/null; kill -TERM $NODE_PID 2>/dev/null; wait' INT TERM

# Wait for any process to exit
wait -n $MASTER_PID $NODE_PID
EXIT_CODE=$?

echo "[Entrypoint] Process exited with status $EXIT_CODE, shutting down..."
kill -TERM $MASTER_PID 2>/dev/null || true
kill -TERM $NODE_PID 2>/dev/null || true
wait
exit $EXIT_CODE
