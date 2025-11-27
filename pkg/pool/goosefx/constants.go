package goosefx

import "github.com/gagliardetto/solana-go"

const (
	GOOSEFX_PROGRAM_ID = "7WduLbRfYhTJktjLw5FDEyrqoEv61aTTCuGAetgLjzN5"
)

var (
	GooseFXProgramID = solana.MustPublicKeyFromBase58(GOOSEFX_PROGRAM_ID)
)
