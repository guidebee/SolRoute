package saber

import (
	"context"
	"fmt"

	cosmath "cosmossdk.io/math"
	"github.com/gagliardetto/solana-go"
	"soltrading/pkg"
	"soltrading/pkg/sol"
)

// SaberPool represents a Saber StableSwap pool
type SaberPool struct {
	IsInitialized    bool
	IsPaused         bool
	Nonce            uint8
	AmpFactor        uint64 // Amplification coefficient
	Fees             Fees
	TokenAccountA    solana.PublicKey
	TokenAccountB    solana.PublicKey
	TokenMintA       solana.PublicKey
	TokenMintB       solana.PublicKey
	AdminFeeAccountA solana.PublicKey
	AdminFeeAccountB solana.PublicKey

	PoolId solana.PublicKey
}

type Fees struct {
	AdminTradeFeeNumerator      uint64
	AdminTradeFeeDenominator    uint64
	AdminWithdrawFeeNumerator   uint64
	AdminWithdrawFeeDenominator uint64
	TradeFeeNumerator           uint64
	TradeFeeDenominator         uint64
	WithdrawFeeNumerator        uint64
	WithdrawFeeDenominator      uint64
}

func (p *SaberPool) ProtocolName() pkg.ProtocolName {
	return pkg.ProtocolName("saber")
}

func (p *SaberPool) GetProgramID() solana.PublicKey {
	return SaberSwapProgramID
}

func (p *SaberPool) GetID() string {
	return p.PoolId.String()
}

func (p *SaberPool) GetTokens() (string, string) {
	return p.TokenMintA.String(), p.TokenMintB.String()
}

func (p *SaberPool) Decode(data []byte) error {
	// TODO: Implement Saber StableSwap account decoding
	return fmt.Errorf("saber decode not yet implemented")
}

func (p *SaberPool) Quote(ctx context.Context, solClient *sol.Client, inputMint string, amount cosmath.Int) (cosmath.Int, error) {
	// TODO: Implement Curve StableSwap formula
	// Uses: A * sum(x_i) * n^n + D = A * D * n^n + D^(n+1) / (n^n * prod(x_i))
	return cosmath.ZeroInt(), fmt.Errorf("saber stableswap quote not yet implemented")
}

func (p *SaberPool) BuildSwapInstructions(
	ctx context.Context,
	solClient *sol.Client,
	user solana.PublicKey,
	inputMint string,
	inputAmount cosmath.Int,
	minOutputAmount cosmath.Int,
	userBaseAccount solana.PublicKey,
	userQuoteAccount solana.PublicKey,
) ([]solana.Instruction, error) {
	return nil, fmt.Errorf("saber swap instructions not yet implemented")
}
