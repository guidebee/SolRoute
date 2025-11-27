package woofi

import "github.com/gagliardetto/solana-go"

const (
	WOOFI_PROGRAM_ID = "WooU7pPnMoJZvr8dFQxY8YXBjZh7tkkqyTEFu4vUFpd"
)

var (
	WooFiProgramID = solana.MustPublicKeyFromBase58(WOOFI_PROGRAM_ID)
)
