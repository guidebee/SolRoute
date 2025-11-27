package aldrin

import (
	"context"
	"encoding/binary"
	"fmt"

	cosmath "cosmossdk.io/math"
	"github.com/gagliardetto/solana-go"
	"soltrading/pkg"
	"soltrading/pkg/sol"
)

// AldrinPool represents an Aldrin AMM pool
type AldrinPool struct {
	TokenA         solana.PublicKey
	TokenB         solana.PublicKey
	TokenVaultA    solana.PublicKey
	TokenVaultB    solana.PublicKey
	ReserveA       uint64
	ReserveB       uint64
	FeeNumerator   uint64
	FeeDenominator uint64

	PoolId solana.PublicKey
}

func (p *AldrinPool) ProtocolName() pkg.ProtocolName {
	return pkg.ProtocolName("aldrin")
}

func (p *AldrinPool) GetProgramID() solana.PublicKey {
	return AldrinAmmProgramID
}

func (p *AldrinPool) GetID() string {
	return p.PoolId.String()
}

func (p *AldrinPool) GetTokens() (string, string) {
	return p.TokenA.String(), p.TokenB.String()
}

func (p *AldrinPool) Decode(data []byte) error {
	if len(data) < 200 {
		return fmt.Errorf("data too short for Aldrin pool: got %d bytes", len(data))
	}

	offset := 8 // Skip discriminator

	// Token mints
	copy(p.TokenA[:], data[offset:offset+32])
	offset += 32
	copy(p.TokenB[:], data[offset:offset+32])
	offset += 32

	// Token vaults
	copy(p.TokenVaultA[:], data[offset:offset+32])
	offset += 32
	copy(p.TokenVaultB[:], data[offset:offset+32])
	offset += 32

	// Reserves are stored in the pool state
	p.ReserveA = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	p.ReserveB = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	// Fee (assuming 0.25% = 25 bps)
	p.FeeNumerator = 25
	p.FeeDenominator = 10000

	return nil
}

func (p *AldrinPool) Quote(ctx context.Context, solClient *sol.Client, inputMint string, amount cosmath.Int) (cosmath.Int, error) {
	// Fetch vault balances to get current reserves
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

		// Extract balance from token account (offset 64, 8 bytes)
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
	if inputMint == p.TokenA.String() {
		reserveIn = reserveA
		reserveOut = reserveB
	} else {
		reserveIn = reserveB
		reserveOut = reserveA
	}

	if amount.IsZero() {
		return cosmath.ZeroInt(), nil
	}

	// Calculate fee (0.25%)
	feeNumerator := cosmath.NewInt(int64(p.FeeNumerator))
	feeDenominator := cosmath.NewInt(int64(p.FeeDenominator))
	fee := amount.Mul(feeNumerator).Quo(feeDenominator)

	// Amount after fee
	amountInWithFee := amount.Sub(fee)

	// Constant product formula: amountOut = (reserveOut * amountInWithFee) / (reserveIn + amountInWithFee)
	denominator := reserveIn.Add(amountInWithFee)
	amountOut := reserveOut.Mul(amountInWithFee).Quo(denominator)

	return amountOut, nil
}

func (p *AldrinPool) BuildSwapInstructions(
	ctx context.Context,
	solClient *sol.Client,
	user solana.PublicKey,
	inputMint string,
	inputAmount cosmath.Int,
	minOutputAmount cosmath.Int,
	userBaseAccount solana.PublicKey,
	userQuoteAccount solana.PublicKey,
) ([]solana.Instruction, error) {
	return nil, fmt.Errorf("aldrin swap instructions not yet implemented")
}
