#!/bin/bash

# scripts/local_run.sh
# Orchestrates starting all FitGlue services locally.

# 1. Load Environment Variables
if [ -f .env ]; then
    echo "Loading .env file..."
    export $(grep -v '^#' .env | xargs)
else
    echo "WARNING: No .env file found. Secrets may fail if not set."
    echo "Copy .env.example to .env and configure it."
fi

# Function to kill all background jobs on exit
cleanup() {
    echo "Shutting down all services..."
    kill $(jobs -p) 2>/dev/null
}
trap cleanup SIGINT SIGTERM EXIT

echo "Starting generic services..."

# 2. Start Services
# Hevy Handler (TS) - Port 8080
echo "[Hevy Handler] Starting on :8080..."
(cd functions/hevy-handler && npm run dev > ../../hevy.log 2>&1) &

# Keiser Poller (TS) - Port 8084
echo "[Keiser Poller] Starting on :8084..."
(cd functions/keiser-poller && PORT=8084 npm run dev > ../../keiser.log 2>&1) &

# Enricher (Go) - Port 8081
echo "[Enricher] Starting on :8081..."
(cd functions/enricher && FUNCTION_TARGET=EnrichActivity go run cmd/main.go > ../../enricher.log 2>&1) &

# Router (Go) - Port 8082
echo "[Router] Starting on :8082..."
(cd functions/router && FUNCTION_TARGET=RouteActivity go run cmd/main.go > ../../router.log 2>&1) &

# Strava Uploader (Go) - Port 8083
echo "[Strava Uploader] Starting on :8083..."
(cd functions/strava-uploader && FUNCTION_TARGET=UploadToStrava go run cmd/main.go > ../../uploader.log 2>&1) &

echo "All services started. Logs are being written to *.log files in root."
echo "Press Ctrl+C to stop."
echo "---------------------------------------------------"
echo "Hevy Handler:   http://localhost:8080"
echo "Keiser Poller:  http://localhost:8084"
echo "Enricher:       http://localhost:8081"
echo "Router:         http://localhost:8082"
echo "Uploader:       http://localhost:8083"
echo "---------------------------------------------------"

# Wait forever
wait
