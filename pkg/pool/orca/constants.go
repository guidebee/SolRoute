package orca

import "github.com/gagliardetto/solana-go"

// Orca (Legacy) AMM Program ID
const (
	ORCA_AMM_PROGRAM_ID = "9W959DqEETiGZocYWCQPaJ6sBmUzgfxXfqGeTEdp3aQP"
)

var (
	OrcaAmmProgramID = solana.MustPublicKeyFromBase58(ORCA_AMM_PROGRAM_ID)
)

// Note: Orca's new CLMM is "Whirlpool" - separate implementation
