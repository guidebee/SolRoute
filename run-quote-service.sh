#!/bin/bash
# Script to build and run the SolRoute Quote Service

# Default configuration
PORT=${1:-8080}
REFRESH=${2:-30}
SLIPPAGE=${3:-50}
RATELIMIT=${4:-20}

echo "========================================"
echo "SolRoute Quote Service Builder"
echo "========================================"
echo ""

# Check if .env file exists
if [ ! -f ".env" ]; then
    echo "WARNING: .env file not found!"
    echo "Please copy .env.example to .env and configure your RPC endpoints."
    echo ""
    echo "Example:"
    echo "  cp .env.example .env"
    echo "  nano .env"
    echo ""
    exit 1
fi

echo "Building quote-service..."
go build -o quote-service ./cmd/quote-service

if [ $? -ne 0 ]; then
    echo ""
    echo "ERROR: Build failed!"
    exit 1
fi

echo ""
echo "Build successful!"
echo ""

echo "Starting quote-service with configuration:"
echo "  Port: $PORT"
echo "  Refresh interval: ${REFRESH}s"
echo "  Slippage: $SLIPPAGE bps"
echo "  Rate limit: $RATELIMIT req/s"
echo ""
echo "Press Ctrl+C to stop the service"
echo "========================================"
echo ""

# Run the service
./quote-service -port $PORT -refresh $REFRESH -slippage $SLIPPAGE -ratelimit $RATELIMIT
