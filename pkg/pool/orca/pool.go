package orca

import (
	"context"
	"encoding/binary"
	"fmt"

	cosmath "cosmossdk.io/math"
	"github.com/gagliardetto/solana-go"
	"soltrading/pkg"
	"soltrading/pkg/sol"
)

// OrcaPool represents an Orca legacy AMM pool
type OrcaPool struct {
	TokenAccountA  solana.PublicKey
	TokenAccountB  solana.PublicKey
	TokenMintA     solana.PublicKey
	TokenMintB     solana.PublicKey
	FeeNumerator   uint64
	FeeDenominator uint64

	PoolId solana.PublicKey
}

func (p *OrcaPool) ProtocolName() pkg.ProtocolName {
	return pkg.ProtocolName("orca")
}

func (p *OrcaPool) GetProgramID() solana.PublicKey {
	return OrcaAmmProgramID
}

func (p *OrcaPool) GetID() string {
	return p.PoolId.String()
}

func (p *OrcaPool) GetTokens() (string, string) {
	return p.TokenMintA.String(), p.TokenMintB.String()
}

func (p *OrcaPool) Decode(data []byte) error {
	if len(data) < 256 {
		return fmt.Errorf("data too short for Orca pool: got %d bytes", len(data))
	}

	offset := 8 // Skip discriminator

	// Token accounts (vaults)
	copy(p.TokenAccountA[:], data[offset:offset+32])
	offset += 32
	copy(p.TokenAccountB[:], data[offset:offset+32])
	offset += 32

	// Skip pool token mint (32 bytes)
	offset += 32

	// Token mints
	copy(p.TokenMintA[:], data[offset:offset+32])
	offset += 32
	copy(p.TokenMintB[:], data[offset:offset+32])
	offset += 32

	// Skip to fees (may vary by implementation)
	// Default Orca fee: 0.3% = 30 bps
	p.FeeNumerator = 30
	p.FeeDenominator = 10000

	return nil
}

func (p *OrcaPool) Quote(ctx context.Context, solClient *sol.Client, inputMint string, amount cosmath.Int) (cosmath.Int, error) {
	// Fetch vault balances
	accounts := []solana.PublicKey{p.TokenAccountA, p.TokenAccountB}
	results, err := solClient.GetMultipleAccountsWithOpts(ctx, accounts)
	if err != nil {
		return cosmath.ZeroInt(), fmt.Errorf("failed to fetch vault balances: %w", err)
	}

	var reserveA, reserveB cosmath.Int
	for i, result := range results.Value {
		if result == nil {
			return cosmath.ZeroInt(), fmt.Errorf("vault account %s not found", accounts[i])
		}

		amountBytes := result.Data.GetBinary()[64:72]
		balance := binary.LittleEndian.Uint64(amountBytes)

		if accounts[i].Equals(p.TokenAccountA) {
			reserveA = cosmath.NewIntFromUint64(balance)
		} else {
			reserveB = cosmath.NewIntFromUint64(balance)
		}
	}

	// Determine swap direction
	var reserveIn, reserveOut cosmath.Int
	if inputMint == p.TokenMintA.String() {
		reserveIn = reserveA
		reserveOut = reserveB
	} else {
		reserveIn = reserveB
		reserveOut = reserveA
	}

	if amount.IsZero() {
		return cosmath.ZeroInt(), nil
	}

	// Calculate fee
	feeNumerator := cosmath.NewInt(int64(p.FeeNumerator))
	feeDenominator := cosmath.NewInt(int64(p.FeeDenominator))
	fee := amount.Mul(feeNumerator).Quo(feeDenominator)

	// Amount after fee
	amountInWithFee := amount.Sub(fee)

	// Constant product formula
	denominator := reserveIn.Add(amountInWithFee)
	amountOut := reserveOut.Mul(amountInWithFee).Quo(denominator)

	return amountOut, nil
}

func (p *OrcaPool) BuildSwapInstructions(
	ctx context.Context,
	solClient *sol.Client,
	user solana.PublicKey,
	inputMint string,
	inputAmount cosmath.Int,
	minOutputAmount cosmath.Int,
	userBaseAccount solana.PublicKey,
	userQuoteAccount solana.PublicKey,
) ([]solana.Instruction, error) {
	return nil, fmt.Errorf("orca swap instructions not yet implemented")
}
