package whirlpool

import "github.com/gagliardetto/solana-go"

// Whirlpool (Orca) program IDs
const (
	// WHIRLPOOL_PROGRAM_ID is the Orca Whirlpool CLMM program
	WHIRLPOOL_PROGRAM_ID = "whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc"
)

var (
	WhirlpoolProgramID = solana.MustPublicKeyFromBase58(WHIRLPOOL_PROGRAM_ID)
)

// Whirlpool account discriminators
const (
	WHIRLPOOL_ACCOUNT_DISCRIMINATOR = "63M5OOj1XoGJ2nM" // First 8 bytes of account in base58
)

// Fee tiers (basis points)
const (
	FEE_RATE_BPS_0_01 = 1   // 0.01%
	FEE_RATE_BPS_0_05 = 5   // 0.05%
	FEE_RATE_BPS_0_25 = 25  // 0.25%
	FEE_RATE_BPS_1_00 = 100 // 1.00%
)

// Tick constants
const (
	TICK_ARRAY_SIZE     = 88
	MIN_TICK            = -443636
	MAX_TICK            = 443636
	TICK_SPACING_STABLE = 1
	TICK_SPACING_LOW    = 64
	TICK_SPACING_MED    = 128
)
