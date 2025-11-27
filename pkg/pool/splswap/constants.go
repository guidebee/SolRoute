package splswap

import "github.com/gagliardetto/solana-go"

// SPL Token Swap Program ID (official Solana program)
const (
	SPL_TOKEN_SWAP_PROGRAM_ID = "SwaPpA9LAaLfeLi3a68M4DjnLqgtticKg6CnyNwgAC8"
)

var (
	SplTokenSwapProgramID = solana.MustPublicKeyFromBase58(SPL_TOKEN_SWAP_PROGRAM_ID)
)

// Fee structure (basis points)
const (
	DEFAULT_FEE_NUMERATOR   = 25
	DEFAULT_FEE_DENOMINATOR = 10000
)
