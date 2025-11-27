package byreal

import "github.com/gagliardetto/solana-go"

const (
	BYREAL_PROGRAM_ID = "BYREuLZYdT7pxDKtvkMoBE4DGAG2XdhnGmhWzgBXSFnb"
)

var (
	ByrealProgramID = solana.MustPublicKeyFromBase58(BYREAL_PROGRAM_ID)
)
