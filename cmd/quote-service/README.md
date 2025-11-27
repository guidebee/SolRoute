# SolRoute Quote Service

A high-performance HTTP service that provides instant DEX quotes by periodically caching the best routes for SOL↔USDC trades. Instead of querying DEX pools in real-time (which takes 15-30 seconds), this service maintains up-to-date cached quotes and returns them instantly.

## Features

- **Instant Responses**: Returns cached quotes in milliseconds instead of 15-30 seconds
- **Periodic Refresh**: Automatically updates quotes at configurable intervals (default: 30 seconds)
- **Multi-Protocol**: Queries Raydium (AMM, CLMM, CPMM), PumpSwap, and Meteora DLMM
- **RPC Pool**: Built-in load balancing across multiple RPC endpoints
- **RESTful API**: Simple HTTP endpoints for integration
- **CORS Enabled**: Ready for frontend integration

## Installation

```bash
# Build the service
go build -o quote-service ./cmd/quote-service

# Or run directly
go run ./cmd/quote-service/*.go
```

## Usage

### Start the Service

**Basic usage (with default settings):**
```bash
./quote-service
```

**Custom configuration:**
```bash
./quote-service \
  -port 8080 \
  -refresh 30 \
  -slippage 50 \
  -ratelimit 20
```

### Configuration Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-port` | HTTP server port | 8080 |
| `-refresh` | Quote refresh interval (seconds) | 30 |
| `-slippage` | Slippage tolerance (basis points) | 50 (0.5%) |
| `-ratelimit` | RPC requests per second per endpoint | 20 |
| `-rpc` | Comma-separated RPC endpoints | Default pool |

### Default Monitored Pairs

The service automatically caches quotes for:
- **SOL → USDC** (1 SOL)
- **USDC → SOL** (10 USDC)

## API Endpoints

### GET /quote

Get a cached quote for a token pair.

**Query Parameters:**
- `input` - Input token mint address (required)
- `output` - Output token mint address (required)
- `amount` - Input amount in smallest units (required)

**Example Request:**
```bash
# SOL to USDC (1 SOL)
curl "http://localhost:8080/quote?input=So11111111111111111111111111111111111111112&output=EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v&amount=1000000000"

# USDC to SOL (10 USDC)
curl "http://localhost:8080/quote?input=EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v&output=So11111111111111111111111111111111111111112&amount=10000000"
```

**Success Response (200 OK):**
```json
{
  "inputMint": "So11111111111111111111111111111111111111112",
  "outputMint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
  "inAmount": "1000000000",
  "outAmount": "137519139",
  "slippageBps": 50,
  "otherAmountThreshold": "136831543",
  "lastUpdate": "2025-11-25T11:45:00Z",
  "timeTaken": "17.5s",
  "routePlan": [
    {
      "protocol": "meteora_dlmm",
      "poolId": "8sLbNZoA1cfnvMJLPfp98ZLAnFSYCFApfJKMbiXNLwxj",
      "poolAddress": "8sLbNZoA1cfnvMJLPfp98ZLAnFSYCFApfJKMbiXNLwxj",
      "inputMint": "So11111111111111111111111111111111111111112",
      "outputMint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
      "inAmount": "1000000000",
      "outAmount": "137519139",
      "programId": "LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo",
      "tokenASymbol": "Input",
      "tokenBSymbol": "Output"
    }
  ]
}
```

**Error Response (404 Not Found):**
```json
{
  "error": "Quote not found in cache. Available pairs: SOL->USDC (1 SOL), USDC->SOL (10 USDC)"
}
```

### GET /health

Check service health and cache status.

**Example Request:**
```bash
curl http://localhost:8080/health
```

**Response:**
```json
{
  "status": "healthy",
  "lastUpdate": "2025-11-25T11:45:00Z",
  "cachedRoutes": 2,
  "uptime": "5m30s"
}
```

### GET /

Get service information and all cached quotes.

**Example Request:**
```bash
curl http://localhost:8080/
```

**Response:**
```json
{
  "service": "SolRoute Quote Service",
  "status": "running",
  "cachedQuotes": 2,
  "quotes": {
    "So111...112-EPjF...t1v-1000000000": { /* cached quote */ },
    "EPjF...t1v-So111...112-10000000": { /* cached quote */ }
  },
  "endpoints": {
    "quote": "/quote?input=<mint>&output=<mint>&amount=<amount>",
    "health": "/health"
  }
}
```

## Response Fields

| Field | Description |
|-------|-------------|
| `inputMint` | Input token mint address |
| `outputMint` | Output token mint address |
| `inAmount` | Input amount in smallest units |
| `outAmount` | Expected output amount |
| `slippageBps` | Slippage tolerance in basis points |
| `otherAmountThreshold` | Minimum output after slippage |
| `lastUpdate` | Timestamp of last cache update |
| `timeTaken` | Time taken to compute the quote |
| `routePlan` | Array of route details |

### RoutePlan Fields

| Field | Description |
|-------|-------------|
| `protocol` | DEX protocol name (e.g., "meteora_dlmm") |
| `poolId` | Pool account address |
| `poolAddress` | Pool address (same as poolId) |
| `programId` | DEX program ID |
| `tokenASymbol` | Token A symbol |
| `tokenBSymbol` | Token B symbol |

## Common Token Addresses

| Token | Mint Address |
|-------|--------------|
| SOL (wrapped) | `So11111111111111111111111111111111111111112` |
| USDC | `EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v` |

## Architecture

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ HTTP GET /quote (instant)
       ▼
┌─────────────────────┐
│  Quote Service      │
│  ┌───────────────┐  │
│  │ Quote Cache   │  │
│  │ - SOL→USDC    │  │
│  │ - USDC→SOL    │  │
│  └───────────────┘  │
└──────┬──────────────┘
       │ Periodic Refresh (30s)
       ▼
┌─────────────────────┐
│   RPC Pool          │
│  ┌────┬────┬────┐   │
│  │RPC1│RPC2│RPC3│   │
│  └────┴────┴────┘   │
└──────┬──────────────┘
       │
       ▼
┌─────────────────────┐
│  DEX Protocols      │
│  - Raydium          │
│  - Meteora          │
│  - PumpSwap         │
└─────────────────────┘
```

## Performance

- **Cached Response Time**: < 5ms
- **Cache Update Time**: 15-30 seconds
- **Default Refresh Interval**: 30 seconds
- **RPC Pool**: Distributes load across multiple endpoints

## Use Cases

1. **Trading Bots**: Get instant quotes without waiting for pool queries
2. **Price Feeds**: Display real-time SOL/USDC prices
3. **Frontend Integration**: Show quotes to users instantly
4. **Arbitrage Monitoring**: Fast access to best routes

## Integration Example

### JavaScript/TypeScript
```javascript
const response = await fetch(
  'http://localhost:8080/quote?' +
  'input=So11111111111111111111111111111111111111112&' +
  'output=EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v&' +
  'amount=1000000000'
);
const quote = await response.json();
console.log(`Best rate: ${quote.outAmount} USDC for 1 SOL`);
console.log(`Protocol: ${quote.routePlan[0].protocol}`);
```

### Python
```python
import requests

response = requests.get('http://localhost:8080/quote', params={
    'input': 'So11111111111111111111111111111111111111112',
    'output': 'EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v',
    'amount': '1000000000'
})
quote = response.json()
print(f"Best rate: {quote['outAmount']} USDC for 1 SOL")
```

### cURL
```bash
curl -s "http://localhost:8080/quote?input=So11111111111111111111111111111111111111112&output=EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v&amount=1000000000" | jq
```

## Production Deployment

### Docker (Example)
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o quote-service ./cmd/quote-service

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/quote-service /usr/local/bin/
EXPOSE 8080
CMD ["quote-service"]
```

### systemd Service
```ini
[Unit]
Description=SolRoute Quote Service
After=network.target

[Service]
Type=simple
User=solroute
ExecStart=/usr/local/bin/quote-service -port 8080 -refresh 30
Restart=always

[Install]
WantedBy=multi-user.target
```

## Monitoring

Check service logs for refresh activity:
```bash
./quote-service 2>&1 | tee quote-service.log
```

Monitor health endpoint:
```bash
watch -n 5 'curl -s http://localhost:8080/health | jq'
```

## Troubleshooting

**Quote not found:**
- Only predefined pairs are cached (SOL→USDC, USDC→SOL)
- Check the exact amounts match (1 SOL = 1000000000, 10 USDC = 10000000)

**Slow initial startup:**
- First refresh takes 15-30 seconds to query all DEX pools
- Subsequent requests are instant

**Rate limiting errors:**
- Increase RPC pool size with `-rpc` flag
- Reduce `-ratelimit` value
- Increase `-refresh` interval

## License

Same as parent project