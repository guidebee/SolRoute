# SolRoute Quote API

This command-line tool is part of the SolRoute fork that adds Jupiter-like quoting functionality. It queries on-chain DEX pools (Raydium, PumpSwap, Meteora, etc.) and returns the best single-hop quote as JSON (no transactions executed). See the top-level `README.md` for project-wide details and supported protocols.

A console application that provides Jupiter-style DEX quote functionality for Solana token swaps. This tool queries multiple DEX protocols (Raydium, PumpSwap, Meteora) and returns the best available quote without executing any transactions.

## Features

- **Multi-Protocol Support**: Queries Raydium (AMM, CLMM, CPMM), PumpSwap, and Meteora DLMM
- **Best Route Selection**: Automatically finds the pool with the highest output amount
- **JSON Output**: Returns structured data similar to Jupiter's quote API
- **Configurable Slippage**: Set slippage tolerance in basis points
- **No Transaction Execution**: Quote-only, no wallet interaction required

## Installation

```bash
# Build the quote tool
go build -o quote ./cmd/quote

# Or run directly
go run ./cmd/quote/main.go [flags]
```

## Usage

### Basic Usage

```bash
./quote \
  -rpc https://api.mainnet-beta.solana.com \
  -input So11111111111111111111111111111111111111112 \
  -output EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v \
  -amount 10000000 \
  -slippage 50
```

### Command-Line Flags

| Flag | Description | Required | Default |
|------|-------------|----------|---------|
| `-rpc` | Solana RPC endpoint URL | Yes | - |
| `-input` | Input token mint address | Yes | - |
| `-output` | Output token mint address | Yes | - |
| `-amount` | Input amount in smallest units (e.g., lamports for SOL) | Yes | - |
| `-slippage` | Slippage tolerance in basis points | No | 50 (0.5%) |
| `-ratelimit` | RPC requests per second | No | 20 |
| `-json` | Output as JSON format | No | true |

### Examples

**SOL to USDC Quote:**
```bash
./quote \
  -rpc https://api.mainnet-beta.solana.com \
  -input So11111111111111111111111111111111111111112 \
  -output EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v \
  -amount 10000000 \
  -slippage 100
```

**Human-Readable Output:**
```bash
./quote \
  -rpc https://api.mainnet-beta.solana.com \
  -input So11111111111111111111111111111111111111112 \
  -output EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v \
  -amount 10000000 \
  -json=false
```

## Response Format

### Success Response

```json
{
  "inputMint": "So11111111111111111111111111111111111111112",
  "outputMint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
  "inAmount": "10000000",
  "outAmount": "9850000",
  "slippageBps": 50,
  "otherAmountThreshold": "9800750",
  "routePlan": [
    {
      "protocol": "raydium-amm",
      "poolId": "58oQChx4yWmvKdwLLZzBi4ChoCc2fqCUWBkwMihLYQo2",
      "inputMint": "So11111111111111111111111111111111111111112",
      "outputMint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
      "inAmount": "10000000",
      "outAmount": "9850000"
    }
  ]
}
```

### Error Response

```json
{
  "error": "No pools found for this token pair"
}
```

## Response Fields

| Field | Description |
|-------|-------------|
| `inputMint` | Input token mint address |
| `outputMint` | Output token mint address |
| `inAmount` | Input amount in smallest units |
| `outAmount` | Expected output amount in smallest units |
| `slippageBps` | Slippage tolerance in basis points (100 = 1%) |
| `otherAmountThreshold` | Minimum output amount after applying slippage |
| `routePlan` | Array of route steps (currently single-hop only) |
| `routePlan[].protocol` | Protocol name (e.g., "raydium-amm", "pump-amm") |
| `routePlan[].poolId` | Pool account address |

## Common Token Addresses

| Token | Mint Address |
|-------|--------------|
| SOL (wrapped) | `So11111111111111111111111111111111111111112` |
| USDC | `EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v` |
| USDT | `Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB` |

## Notes

- **Amount Format**: All amounts are in the smallest unit (e.g., lamports for SOL with 9 decimals, so 1 SOL = 1,000,000,000)
- **RPC Limits**: Use a high-performance RPC endpoint for best results; public endpoints may rate limit
- **No Wallet Required**: This tool only queries data and does not require a private key
- **Single-Hop Routes**: Currently supports direct swaps only (no multi-hop routing)

## Integration Example

```bash
# Get quote and parse with jq
QUOTE=$(./quote \
  -rpc https://api.mainnet-beta.solana.com \
  -input So11111111111111111111111111111111111111112 \
  -output EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v \
  -amount 10000000)

# Extract output amount
OUT_AMOUNT=$(echo $QUOTE | jq -r '.outAmount')
echo "Expected output: $OUT_AMOUNT"

# Extract best protocol
PROTOCOL=$(echo $QUOTE | jq -r '.routePlan[0].protocol')
echo "Best route: $PROTOCOL"
```

## Supported Protocols

- **Raydium AMM V4**: Constant product AMM pools
- **Raydium CLMM**: Concentrated liquidity pools
- **Raydium CPMM**: Constant product market maker
- **PumpSwap AMM**: PumpSwap protocol pools
- **Meteora DLMM**: Dynamic liquidity market maker

## Error Handling

The tool returns appropriate error messages for common issues:
- Invalid token addresses
- No pools found for token pair
- RPC connection failures
- Invalid amount format

All errors are returned as JSON when `-json=true` (default).