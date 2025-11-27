## Project Overview
This project is a fork of [solroute](https://github.com/Solana-ZH/solroute).

SolRoute is a Go SDK for building DEX routing services on Solana. It provides direct blockchain interaction without relying on third-party APIs and implements an interface-based design for protocols and pools.

This repository adds Jupiter-like quoting functionality on top of the SDK: a command-line `quote` tool and a `quote-service` HTTP server that aggregate on-chain quotes from many well-known Solana DEX pools and return the best single-hop quote as JSON.

Major supported protocols/pool types include:
- Raydium (AMM V4, CPMM, CLMM)
- Meteora DLMM
- PumpSwap AMM
- Whirlpool (Concentrated Liquidity)
- Saber, Orca, GooseFX, Woofi, Saros, SPL Token Swap and other community protocols

Key features:
- Best single-hop quote selection (Jupiter-style) across supported protocols
- JSON output suitable for UIs and bots
- RPC endpoint pooling and per-endpoint rate limiting
- Optional caching in `quote-service` for popular pairs (e.g., SOL↔USDC) to provide instant responses
- Both CLI (`cmd/quote`) and HTTP service (`cmd/quote-service`) entry points

Common use cases: trading frontends, automated trading bots, price feeds, and arbitrage monitoring.

## Development Commands

### Building and Running
```bash
# Build the project
go build -o solroute .

# Run the main example
go run main.go

# Install dependencies
go mod download

# Update dependencies
go mod tidy
```

### Quick start (recommended)
A few useful commands for the most common entry points in this repository:

```bash
# Build the command-line quote tool (outputs `quote`)
go build -o quote ./cmd/quote

# Run the quote tool directly
go run ./cmd/quote/main.go -input <INPUT_MINT> -output <OUTPUT_MINT> -amount <AMOUNT>

# Build the HTTP quote service (outputs `quote-service`)
go build -o quote-service ./cmd/quote-service

# Run the quote service (reads RPC endpoints from .env if not provided)
./quote-service -port 8080 -refresh 30 -slippage 50 -ratelimit 20

# Helper scripts are provided for convenience (Windows PowerShell, Bash, and batch):
# - run-quote-service.ps1, run-quote-service.sh, run-quote-service.bat
```

Note: The service and tools expect RPC endpoints configured in a `.env` file (see `.env.example`). If `.env` is not present some commands will warn or exit.

### Testing
```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for a specific package
go test -v ./pkg/router
```

### Examples (quick)
Below are minimal examples you can copy-paste to try the main entry points in this repo.

1) `quote` (CLI)
```bash
# Query a SOL -> USDC quote using a public RPC
go run ./cmd/quote/main.go \
  -rpc https://api.mainnet-beta.solana.com \
  -input So11111111111111111111111111111111111111112 \
  -output EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v \
  -amount 1000000000
```
Sample JSON output (truncated):
```json
{
  "inputMint": "So11111111111111111111111111111111111111112",
  "outputMint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
  "inAmount": "1000000000",
  "outAmount": "137519139",
  "slippageBps": 50,
  "routePlan": [ { "protocol": "meteora_dlmm", "poolId": "..." } ]
}
```

2) `quote-service` (HTTP)
```bash
# Build and run the service
go build -o quote-service ./cmd/quote-service
./quote-service -port 8080 -refresh 30 -slippage 50 -ratelimit 20

# Then query the running service:
curl -s "http://localhost:8080/quote?input=So11111111111111111111111111111111111111112&output=EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v&amount=1000000000" | jq
```
Sample response (truncated):
```json
{
  "inputMint": "So11111111111111111111111111111111111111112",
  "outputMint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
  "inAmount": "1000000000",
  "outAmount": "137519139",
  "lastUpdate": "2025-11-25T11:45:00Z",
  "routePlan": [ { "protocol": "meteora_dlmm", "poolId": "..." } ]
}
```

### Development environment
- This project uses Go (see `go.mod` — module: `soltrading`, `go 1.24`).
- The binaries read RPC endpoints from a `.env` file by default. Copy `.env.example` to `.env` and set `RPC_ENDPOINTS` as a comma-separated list, for example:
```env
RPC_ENDPOINTS="https://api.mainnet-beta.solana.com"
```
- Helper scripts are included for convenience: `run-quote-service.ps1`, `run-quote-service.sh`, `run-quote-service.bat`.

### Contributing (short)
- Fork and create a feature branch
- Run `go test ./...` and ensure tests pass
- Open a PR with a clear description and changelog
- Optionally run `go vet` and `golangci-lint` if available

## Core Architecture

### Interface-Based Design

The codebase uses two core interfaces defined in [pkg/api.go](pkg/api.go):

- **`Protocol`**: Represents a DEX protocol implementation (e.g., Raydium, Pump, Meteora)
    - `FetchPoolsByPair(ctx, baseMint, quoteMint)` - Fetches all pools for a token pair
    - `FetchPoolByID(ctx, poolID)` - Fetches a specific pool by ID
    - `ProtocolName()` - Returns the protocol identifier

- **`Pool`**: Represents a liquidity pool instance
    - `Quote(ctx, solClient, inputMint, inputAmount)` - Calculates expected output
    - `BuildSwapInstructions(ctx, solClient, user, inputMint, inputAmount, minOut, userBaseAccount, userQuoteAccount)` - Constructs swap transaction instructions
    - `GetID()`, `GetTokens()`, `GetProgramID()` - Pool metadata accessors

### Package Structure

```
pkg/
├── api.go              # Core Pool and Protocol interfaces
├── anchor/             # Anchor discriminator utilities
├── pool/               # Pool implementations
│   ├── raydium/        # Raydium AMM, CLMM, CPMM pool logic
│   ├── pump/           # PumpSwap AMM pool logic
│   └── meteora/        # Meteora DLMM pool logic
├── protocol/           # Protocol implementations that fetch and manage pools
│   ├── raydium_amm.go
│   ├── raydium_clmm.go
│   ├── raydium_cpmm.go
│   ├── pump_amm.go
│   └── meteora_dlmm.go
├── router/             # SimpleRouter that finds best execution paths
└── sol/                # Solana client wrapper with rate limiting
```

### Routing Flow

1. **Protocol Registration**: Initialize `SimpleRouter` with desired protocols
2. **Pool Discovery**: Call `QueryAllPools(ctx, baseMint, quoteMint)` to fetch all available pools across protocols
3. **Route Selection**: Call `GetBestPool(ctx, solClient, tokenIn, amountIn)` to find optimal pool (uses concurrent quotes)
4. **Transaction Building**: Call `BuildSwapInstructions()` on selected pool to construct transaction

The router's `GetBestPool` method concurrently queries all discovered pools using goroutines and selects the one with the highest output amount.

## Solana Client Wrapper

The [pkg/sol/client.go](pkg/sol/client.go) provides a rate-limited RPC client wrapper:

- **Rate Limiting**: Configured via `reqLimitPerSecond` parameter to prevent RPC throttling
- **Jito Support**: Optional Jito integration for MEV protection and bundle submission
- **Helper Methods**:
    - `GetMultipleAccountsWithOpts()` - Batch account fetching
    - `GetUserTokenBalance()` - Token balance queries
    - `SignTransaction()`, `SendTx()` - Transaction handling
    - `SendTxWithJito()` - Jito bundle submission

## Token Account Management

Before executing swaps, ensure proper SPL token accounts exist:

- **WSOL Handling**: Use `CoverWsol(ctx, privateKey, amount)` to wrap SOL into WSOL and `CloseWsol(ctx, privateKey)` to unwrap
- **Token Account Creation**: Use `SelectOrCreateSPLTokenAccount(ctx, privateKey, mint)` to get or create the associated token account

The wrapped SOL workflow is critical: Solana's native SOL must be wrapped into WSOL (wrapped SOL) before swapping, as DEXes operate on SPL tokens.

## Pool Implementation Patterns

### State Fetching
All pool implementations follow this pattern:
1. Fetch on-chain pool account data using `GetProgramAccountsWithOpts` or `GetAccountInfoWithOpts`
2. Decode binary data into Go structs (see [pkg/pool/raydium/ammPool.go](pkg/pool/raydium/ammPool.go) for complex decoding example)
3. Fetch associated vault/reserve balances for accurate quotes

### Quote Calculation
- **AMM Pools**: Use constant product formula `x * y = k` with fee adjustments
- **CLMM Pools**: Calculate across tick ranges with concentrated liquidity
- **DLMM Pools**: Calculate across discrete bins with dynamic fees

### Instruction Building
Each pool type constructs protocol-specific instructions with proper account ordering and data encoding (see `BuildSwapInstructions` implementations).

## Important Utilities

### Anchor Discriminator
[pkg/anchor/anchor.go](pkg/anchor/anchor.go) provides `GetDiscriminator(namespace, name)` to generate 8-byte discriminators for Anchor program accounts (SHA256 hash of "namespace:name").

### Beautiful Address Generation
[utils/beautiful_address.go](utils/beautiful_address.go) contains `FindKeyPairWithPrefix` and `FindKeyPairWithSuffix` for vanity address generation.

### Jito Integration
[pkg/sol/jito.go](pkg/sol/jito.go) provides:
- `NewJitoClient(ctx, endpoint)` - Initialize Jito client
- `SendTxWithJito(ctx, tipAmount, signers, tx)` - Submit transactions via Jito
- `CheckBundleStatus(bundleId)` - Monitor bundle execution

## Program IDs

The SDK interacts with these Solana programs:
- Raydium AMM V4: `675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8`
- Raydium CPMM: `CPMMoo8L3F4NbTegBCKVNunggL7H1ZpdTHKxQB5qKP1C`
- Raydium CLMM: `CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK`
- PumpSwap AMM: `pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA`
- Meteora DLMM: `LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo`
- Whirlpool CLMM: `whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc`

These are defined as constants in respective pool implementation files (e.g., `pkg/pool/raydium/constants.go`).

## Code Style

- Use `context.Context` for all blockchain operations
- Leverage `cosmossdk.io/math.Int` for arbitrary precision arithmetic (critical for financial calculations)
- Pool state modifications should update both raw values and computed reserves
- Always check pool status/state flags before quoting or swapping
- Use struct field tags for binary encoding/decoding (`borsh`, `bin`)

## Adding New Protocol Support

1. Create pool struct implementing `Pool` interface in `pkg/pool/{protocol}/`
2. Implement on-chain data decoding (use reflection-based `Offset()` and `Span()` for RPC filters)
3. Implement `Quote()` logic matching protocol's pricing formula
4. Implement `BuildSwapInstructions()` with correct account ordering
5. Create protocol struct implementing `Protocol` interface in `pkg/protocol/`
6. Register protocol instance in router initialization

## Common Gotchas

- **Account Derivation**: Raydium pools require deriving multiple PDAs (authority, market authority). See [pkg/protocol/raydium_amm.go](pkg/protocol/raydium_amm.go) `processAMMPool()`.
- **Decimals Handling**: Token amounts are stored as raw integers; apply decimals for display only
- **Reserve Calculations**: Raydium AMM reserves subtract pending PnL (`BaseNeedTakePnl`, `QuoteNeedTakePnl`)
- **Concurrent Quotes**: The router queries pools concurrently; ensure thread-safe client usage
- **Binary Encoding**: Solana uses little-endian for all numeric types; use `encoding/binary.LittleEndian`
