package pancakeswapv3

import "github.com/gagliardetto/solana-go"

const (
	PANCAKESWAP_V3_PROGRAM_ID = "PSwapMdSP4y7IzFEiEEaBMz3rwx8bT5DMYnyBNMYYBo"
)

var (
	PancakeSwapV3ProgramID = solana.MustPublicKeyFromBase58(PANCAKESWAP_V3_PROGRAM_ID)
)
