package meteoradbc

import "github.com/gagliardetto/solana-go"

const (
	METEORA_DBC_PROGRAM_ID = "Eo7WjKq67rjJQSZxS6z3YkapzY3eMj6Xy8X5EQVn5UaB"
)

var (
	MeteoraDBCProgramID = solana.MustPublicKeyFromBase58(METEORA_DBC_PROGRAM_ID)
)
