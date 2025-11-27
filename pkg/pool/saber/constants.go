package saber

import "github.com/gagliardetto/solana-go"

// Saber StableSwap Program ID
const (
	SABER_SWAP_PROGRAM_ID = "SSwpkEEcbUqx4vtoEByFjSkhKdCT862DNVb52nZg1UZ"
)

var (
	SaberSwapProgramID = solana.MustPublicKeyFromBase58(SABER_SWAP_PROGRAM_ID)
)

// Saber uses Curve's StableSwap algorithm
const (
	DEFAULT_AMP_FACTOR = 100
	FEE_DENOMINATOR    = 10000
)
