#!/usr/bin/env bash
# examples/quote-service/run_example.sh
# Build, run in background, query endpoints, then stop
set -e

# Build the binary
go build -o quote-service ./cmd/quote-service

# Start service in background
./quote-service -port 8080 -refresh 30 -slippage 50 -ratelimit 20 &
PID=$!
echo "quote-service started (PID=$PID)"

# Wait briefly for startup
sleep 3

# Query a quote (1 SOL -> USDC)
curl -s "http://localhost:8080/quote?input=So11111111111111111111111111111111111111112&output=EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v&amount=1000000000" | jq

# Query health
curl -s http://localhost:8080/health | jq

# Stop the service
kill $PID
wait $PID 2>/dev/null || true
echo "quote-service stopped"

