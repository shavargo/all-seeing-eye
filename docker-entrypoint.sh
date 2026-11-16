#!/bin/sh
set -e

# Start the universal Go master server daemon in the background
/usr/local/bin/master &
MASTER_PID=$!
echo "[Entrypoint] OpenMaster Universal Daemon started (PID: $MASTER_PID)"

# Start Nginx in the background
nginx -g "daemon off;" &
NGINX_PID=$!
echo "[Entrypoint] Nginx Web Server started (PID: $NGINX_PID)"

# Trap signals and forward to both child processes
trap 'kill -TERM $MASTER_PID 2>/dev/null; kill -TERM $NGINX_PID 2>/dev/null; wait' INT TERM

# Wait for any process to exit
wait -n $MASTER_PID $NGINX_PID
EXIT_CODE=$?

echo "[Entrypoint] Process exited with status $EXIT_CODE, shutting down..."
kill -TERM $MASTER_PID 2>/dev/null || true
kill -TERM $NGINX_PID 2>/dev/null || true
wait
exit $EXIT_CODE
