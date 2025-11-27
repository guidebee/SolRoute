package lifinity

import "github.com/gagliardetto/solana-go"

const (
	LIFINITY_PROGRAM_ID = "EewxydAPCCVuNEyrVN68PuSYdQ7wKn27V9Gjeoi8dy3S"
)

var (
	LifinityProgramID = solana.MustPublicKeyFromBase58(LIFINITY_PROGRAM_ID)
)

// Lifinity uses a proactive market maker model with oracle prices
