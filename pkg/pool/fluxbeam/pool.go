package fluxbeam

import (
	"context"
	"encoding/binary"
	"fmt"

	cosmath "cosmossdk.io/math"
	"github.com/gagliardetto/solana-go"
	"soltrading/pkg"
	"soltrading/pkg/sol"
)

type FluxbeamPool struct {
	TokenMintA     solana.PublicKey
	TokenMintB     solana.PublicKey
	TokenVaultA    solana.PublicKey
	TokenVaultB    solana.PublicKey
	FeeNumerator   uint64
	FeeDenominator uint64
	PoolId         solana.PublicKey
}

func (p *FluxbeamPool) ProtocolName() pkg.ProtocolName {
	return pkg.ProtocolName("fluxbeam")
}

func (p *FluxbeamPool) GetProgramID() solana.PublicKey {
	return FluxbeamProgramID
}

func (p *FluxbeamPool) GetID() string {
	return p.PoolId.String()
}

func (p *FluxbeamPool) GetTokens() (string, string) {
	return p.TokenMintA.String(), p.TokenMintB.String()
}

func (p *FluxbeamPool) Decode(data []byte) error {
	if len(data) < 200 {
		return fmt.Errorf("data too short for Fluxbeam pool: got %d bytes", len(data))
	}

	offset := 8 // Skip discriminator

	// Token mints
	copy(p.TokenMintA[:], data[offset:offset+32])
	offset += 32
	copy(p.TokenMintB[:], data[offset:offset+32])
	offset += 32

	// Token vaults
	copy(p.TokenVaultA[:], data[offset:offset+32])
	offset += 32
	copy(p.TokenVaultB[:], data[offset:offset+32])
	offset += 32

	// Default fee: 0.3%
	p.FeeNumerator = 30
	p.FeeDenominator = 10000

	return nil
}

func (p *FluxbeamPool) Quote(ctx context.Context, solClient *sol.Client, inputMint string, amount cosmath.Int) (cosmath.Int, error) {
	// Fetch vault balances
	accounts := []solana.PublicKey{p.TokenVaultA, p.TokenVaultB}
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

		if accounts[i].Equals(p.TokenVaultA) {
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

func (p *FluxbeamPool) BuildSwapInstructions(ctx context.Context, solClient *sol.Client, user solana.PublicKey, inputMint string, inputAmount cosmath.Int, minOutputAmount cosmath.Int, userBaseAccount solana.PublicKey, userQuoteAccount solana.PublicKey) ([]solana.Instruction, error) {
	return nil, fmt.Errorf("fluxbeam swap not yet implemented")
}
