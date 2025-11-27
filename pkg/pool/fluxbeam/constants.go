package fluxbeam

import "github.com/gagliardetto/solana-go"

const (
	FLUXBEAM_PROGRAM_ID = "FLUXubRmkEi2q6K3Y9kBPg9248ggaZVsoSFhtJHSrm1X"
)

var (
	FluxbeamProgramID = solana.MustPublicKeyFromBase58(FLUXBEAM_PROGRAM_ID)
)
