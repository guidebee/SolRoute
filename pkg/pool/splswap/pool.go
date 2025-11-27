package splswap

import (
	"context"
	"encoding/binary"
	"fmt"

	cosmath "cosmossdk.io/math"
	"github.com/gagliardetto/solana-go"
	"soltrading/pkg"
	"soltrading/pkg/sol"
)

// SplSwapPool represents an SPL Token Swap pool
type SplSwapPool struct {
	Version                     uint8
	IsInitialized               bool
	Nonce                       uint8
	TokenProgramId              solana.PublicKey
	TokenAccountA               solana.PublicKey
	TokenAccountB               solana.PublicKey
	TokenPool                   solana.PublicKey
	MintA                       solana.PublicKey
	MintB                       solana.PublicKey
	FeeAccount                  solana.PublicKey
	TradeFeeNumerator           uint64
	TradeFeeDenominator         uint64
	OwnerTradeFeeNumerator      uint64
	OwnerTradeFeeDenominator    uint64
	OwnerWithdrawFeeNumerator   uint64
	OwnerWithdrawFeeDenominator uint64
	HostFeeNumerator            uint64
	HostFeeDenominator          uint64
	CurveType                   uint8

	PoolId solana.PublicKey

	// Pool reserves (fetched from token accounts)
	ReserveA cosmath.Int
	ReserveB cosmath.Int
}

func (p *SplSwapPool) ProtocolName() pkg.ProtocolName {
	return pkg.ProtocolName("spl_token_swap")
}

func (p *SplSwapPool) GetProgramID() solana.PublicKey {
	return SplTokenSwapProgramID
}

func (p *SplSwapPool) GetID() string {
	return p.PoolId.String()
}

func (p *SplSwapPool) GetTokens() (string, string) {
	return p.MintA.String(), p.MintB.String()
}

func (p *SplSwapPool) Decode(data []byte) error {
	if len(data) < 324 {
		return fmt.Errorf("data too short for SPL Token Swap pool: got %d bytes", len(data))
	}

	offset := 0

	// Version, IsInitialized, Nonce
	p.Version = data[offset]
	offset++
	if data[offset] == 1 {
		p.IsInitialized = true
	}
	offset++
	p.Nonce = data[offset]
	offset++

	// Token program ID
	copy(p.TokenProgramId[:], data[offset:offset+32])
	offset += 32

	// Token accounts (vaults)
	copy(p.TokenAccountA[:], data[offset:offset+32])
	offset += 32
	copy(p.TokenAccountB[:], data[offset:offset+32])
	offset += 32

	// Pool token mint
	copy(p.TokenPool[:], data[offset:offset+32])
	offset += 32

	// Token mints
	copy(p.MintA[:], data[offset:offset+32])
	offset += 32
	copy(p.MintB[:], data[offset:offset+32])
	offset += 32

	// Pool fee account
	copy(p.FeeAccount[:], data[offset:offset+32])
	offset += 32

	// Fees
	p.TradeFeeNumerator = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	p.TradeFeeDenominator = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	p.OwnerTradeFeeNumerator = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	p.OwnerTradeFeeDenominator = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	p.OwnerWithdrawFeeNumerator = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	p.OwnerWithdrawFeeDenominator = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	p.HostFeeNumerator = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	p.HostFeeDenominator = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	// Curve type
	p.CurveType = data[offset]

	return nil
}

func (p *SplSwapPool) Quote(ctx context.Context, solClient *sol.Client, inputMint string, amount cosmath.Int) (cosmath.Int, error) {
	// Fetch vault balances
	accounts := []solana.PublicKey{p.TokenAccountA, p.TokenAccountB}
	results, err := solClient.GetMultipleAccountsWithOpts(ctx, accounts)
	if err != nil {
		return cosmath.ZeroInt(), fmt.Errorf("failed to fetch vault balances: %w", err)
	}

	for i, result := range results.Value {
		if result == nil {
			return cosmath.ZeroInt(), fmt.Errorf("vault account %s not found", accounts[i])
		}

		// Extract balance from token account (offset 64, 8 bytes)
		amountBytes := result.Data.GetBinary()[64:72]
		balance := binary.LittleEndian.Uint64(amountBytes)

		if accounts[i].Equals(p.TokenAccountA) {
			p.ReserveA = cosmath.NewIntFromUint64(balance)
		} else {
			p.ReserveB = cosmath.NewIntFromUint64(balance)
		}
	}

	// Determine swap direction
	var reserveIn, reserveOut cosmath.Int
	if inputMint == p.MintA.String() {
		reserveIn = p.ReserveA
		reserveOut = p.ReserveB
	} else {
		reserveIn = p.ReserveB
		reserveOut = p.ReserveA
	}

	if amount.IsZero() {
		return cosmath.ZeroInt(), nil
	}

	// Calculate fee
	feeNumerator := cosmath.NewInt(int64(p.TradeFeeNumerator))
	feeDenominator := cosmath.NewInt(int64(p.TradeFeeDenominator))
	fee := amount.Mul(feeNumerator).Quo(feeDenominator)

	// Amount after fee
	amountInWithFee := amount.Sub(fee)

	// Constant product formula: amountOut = (reserveOut * amountInWithFee) / (reserveIn + amountInWithFee)
	denominator := reserveIn.Add(amountInWithFee)
	amountOut := reserveOut.Mul(amountInWithFee).Quo(denominator)

	return amountOut, nil
}

func (p *SplSwapPool) BuildSwapInstructions(
	ctx context.Context,
	solClient *sol.Client,
	user solana.PublicKey,
	inputMint string,
	inputAmount cosmath.Int,
	minOutputAmount cosmath.Int,
	userBaseAccount solana.PublicKey,
	userQuoteAccount solana.PublicKey,
) ([]solana.Instruction, error) {
	return nil, fmt.Errorf("spl token swap instructions not yet implemented")
}
