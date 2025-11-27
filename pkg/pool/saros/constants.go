package saros

import "github.com/gagliardetto/solana-go"

const (
	SAROS_PROGRAM_ID = "SSwapUtytfBdBn1b9NUGG6foMVPtcWgpRU32HToDUZr"
)

var (
	SarosProgramID = solana.MustPublicKeyFromBase58(SAROS_PROGRAM_ID)
)
