package aldrin

import "github.com/gagliardetto/solana-go"

// Aldrin AMM Program ID
const (
	ALDRIN_AMM_PROGRAM_ID = "AMM55ShdkoGRB5jVYPjWziwk8m5MpwyDgsMWHaMSQWH6"
)

var (
	AldrinAmmProgramID = solana.MustPublicKeyFromBase58(ALDRIN_AMM_PROGRAM_ID)
)
