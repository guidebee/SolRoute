package woofi

import (
	"context"
	"fmt"

	cosmath "cosmossdk.io/math"
	"github.com/gagliardetto/solana-go"
	"soltrading/pkg"
	"soltrading/pkg/sol"
)

type WooFiPool struct {
	TokenMintA solana.PublicKey
	TokenMintB solana.PublicKey
	PoolId     solana.PublicKey
}

func (p *WooFiPool) ProtocolName() pkg.ProtocolName {
	return pkg.ProtocolName("woofi")
}

func (p *WooFiPool) GetProgramID() solana.PublicKey {
	return WooFiProgramID
}

func (p *WooFiPool) GetID() string {
	return p.PoolId.String()
}

func (p *WooFiPool) GetTokens() (string, string) {
	return p.TokenMintA.String(), p.TokenMintB.String()
}

func (p *WooFiPool) Decode(data []byte) error {
	return fmt.Errorf("woofi decode not yet implemented")
}

func (p *WooFiPool) Quote(ctx context.Context, solClient *sol.Client, inputMint string, amount cosmath.Int) (cosmath.Int, error) {
	return cosmath.ZeroInt(), fmt.Errorf("woofi quote not yet implemented")
}

func (p *WooFiPool) BuildSwapInstructions(ctx context.Context, solClient *sol.Client, user solana.PublicKey, inputMint string, inputAmount cosmath.Int, minOutputAmount cosmath.Int, userBaseAccount solana.PublicKey, userQuoteAccount solana.PublicKey) ([]solana.Instruction, error) {
	return nil, fmt.Errorf("woofi swap not yet implemented")
}
