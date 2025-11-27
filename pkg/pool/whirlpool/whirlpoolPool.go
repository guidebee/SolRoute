package whirlpool

import (
	"context"
	"fmt"
	"math/big"
	"time"

	cosmath "cosmossdk.io/math"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"lukechampine.com/uint128"
	"soltrading/pkg"
	"soltrading/pkg/sol"
)

// WhirlpoolPool represents an Orca Whirlpool CLMM pool
type WhirlpoolPool struct {
	// Account discriminator (8 bytes)
	Discriminator [8]uint8

	// Whirlpool config
	WhirlpoolsConfig solana.PublicKey // 32
	WhirlpoolBump    [1]uint8         // 1

	// Token info
	TokenMintA      solana.PublicKey // 32
	TokenMintB      solana.PublicKey // 32
	TokenVaultA     solana.PublicKey // 32
	TokenVaultB     solana.PublicKey // 32
	TickSpacing     uint16           // 2
	TickSpacingSeed [2]uint8         // 2

	// Price and liquidity
	FeeRate          uint16          // 2
	ProtocolFeeRate  uint16          // 2
	Liquidity        uint128.Uint128 // 16
	SqrtPrice        uint128.Uint128 // 16
	TickCurrentIndex int32           // 4
	ProtocolFeeOwedA uint64          // 8
	ProtocolFeeOwedB uint64          // 8
	FeeGrowthGlobalA uint128.Uint128 // 16
	FeeGrowthGlobalB uint128.Uint128 // 16

	// Reward info (3 rewards)
	RewardLastUpdatedTimestamp uint64        // 8
	RewardInfos                [3]RewardInfo // 3 * 128 = 384

	// Pool metadata
	PoolId         solana.PublicKey
	TickArrayCache map[string]*TickArray

	// Cache tracking for WebSocket updates
	lastCacheUpdate time.Time
	cacheDataFresh  bool
}

type RewardInfo struct {
	Mint                  solana.PublicKey // 32
	Vault                 solana.PublicKey // 32
	Authority             solana.PublicKey // 32
	EmissionsPerSecondX64 uint128.Uint128  // 16
	GrowthGlobalX64       uint128.Uint128  // 16
}

type TickArray struct {
	StartTickIndex   int32            // 4
	Ticks            [88]Tick         // 88 * 137 bytes
	WhirlpoolAddress solana.PublicKey // 32
}

type Tick struct {
	Initialized          bool               // 1
	LiquidityNet         big.Int            // 16 (i128)
	LiquidityGross       uint128.Uint128    // 16
	FeeGrowthOutsideA    uint128.Uint128    // 16
	FeeGrowthOutsideB    uint128.Uint128    // 16
	RewardGrowthsOutside [3]uint128.Uint128 // 48
}

func (pool *WhirlpoolPool) ProtocolName() pkg.ProtocolName {
	return pkg.ProtocolName("whirlpool")
}

func (pool *WhirlpoolPool) GetProgramID() solana.PublicKey {
	return WhirlpoolProgramID
}

func (pool *WhirlpoolPool) GetID() string {
	return pool.PoolId.String()
}

func (pool *WhirlpoolPool) GetTokens() (string, string) {
	return pool.TokenMintA.String(), pool.TokenMintB.String()
}

// GetBaseVault returns the base vault address (TokenVaultA)
func (pool *WhirlpoolPool) GetBaseVault() string {
	return pool.TokenVaultA.String()
}

// GetQuoteVault returns the quote vault address (TokenVaultB)
func (pool *WhirlpoolPool) GetQuoteVault() string {
	return pool.TokenVaultB.String()
}

// UpdateFromAccountData implements the PoolStateUpdater interface
func (pool *WhirlpoolPool) UpdateFromAccountData(accountID string, data []byte) error {
	// Check if this is a vault update
	if accountID == pool.TokenVaultA.String() || accountID == pool.TokenVaultB.String() {
		// Mark cache as fresh - Whirlpool uses tick arrays
		pool.lastCacheUpdate = time.Now()
		pool.cacheDataFresh = true
		return nil
	}

	// Check if this is a pool state update
	if accountID == pool.PoolId.String() {
		return pool.Decode(data)
	}

	// Could be tick array update
	pool.lastCacheUpdate = time.Now()
	pool.cacheDataFresh = true
	return nil
}

func (pool *WhirlpoolPool) Decode(data []byte) error {
	if len(data) < 653 {
		return fmt.Errorf("insufficient data: expected 653 bytes, got %d", len(data))
	}

	// Based on official Orca Whirlpool structure from:
	// https://github.com/orca-so/whirlpools/blob/main/programs/whirlpool/src/state/whirlpool.rs

	// Read discriminator (8 bytes at offset 0)
	copy(pool.Discriminator[:], data[0:8])

	// Read WhirlpoolsConfig (32 bytes at offset 8)
	pool.WhirlpoolsConfig = solana.PublicKeyFromBytes(data[8:40])

	// Read WhirlpoolBump (1 byte at offset 40)
	pool.WhirlpoolBump[0] = data[40]

	// Read TickSpacing (2 bytes at offset 41)
	decoder := bin.NewBinDecoder(data[41:43])
	decoder.Decode(&pool.TickSpacing)

	// Read TickSpacingSeed (2 bytes at offset 43)
	decoder = bin.NewBinDecoder(data[43:45])
	decoder.Decode(&pool.TickSpacingSeed)

	// Read FeeRate (2 bytes at offset 45)
	decoder = bin.NewBinDecoder(data[45:47])
	decoder.Decode(&pool.FeeRate)

	// Read ProtocolFeeRate (2 bytes at offset 47)
	decoder = bin.NewBinDecoder(data[47:49])
	decoder.Decode(&pool.ProtocolFeeRate)

	// Read Liquidity (16 bytes at offset 49)
	decoder = bin.NewBinDecoder(data[49:65])
	decoder.Decode(&pool.Liquidity)

	// Read SqrtPrice (16 bytes at offset 65)
	decoder = bin.NewBinDecoder(data[65:81])
	decoder.Decode(&pool.SqrtPrice)

	// Read TickCurrentIndex (4 bytes at offset 81)
	decoder = bin.NewBinDecoder(data[81:85])
	decoder.Decode(&pool.TickCurrentIndex)

	// Read ProtocolFeeOwedA (8 bytes at offset 85)
	decoder = bin.NewBinDecoder(data[85:93])
	decoder.Decode(&pool.ProtocolFeeOwedA)

	// Read ProtocolFeeOwedB (8 bytes at offset 93)
	decoder = bin.NewBinDecoder(data[93:101])
	decoder.Decode(&pool.ProtocolFeeOwedB)

	// Read TokenMintA (32 bytes at offset 101)
	pool.TokenMintA = solana.PublicKeyFromBytes(data[101:133])

	// Read TokenVaultA (32 bytes at offset 133)
	pool.TokenVaultA = solana.PublicKeyFromBytes(data[133:165])

	// Read FeeGrowthGlobalA (16 bytes at offset 165)
	decoder = bin.NewBinDecoder(data[165:181])
	decoder.Decode(&pool.FeeGrowthGlobalA)

	// Read TokenMintB (32 bytes at offset 181)
	pool.TokenMintB = solana.PublicKeyFromBytes(data[181:213])

	// Read TokenVaultB (32 bytes at offset 213)
	pool.TokenVaultB = solana.PublicKeyFromBytes(data[213:245])

	// Read FeeGrowthGlobalB (16 bytes at offset 245)
	decoder = bin.NewBinDecoder(data[245:261])
	decoder.Decode(&pool.FeeGrowthGlobalB)

	// Read RewardLastUpdatedTimestamp (8 bytes at offset 261)
	decoder = bin.NewBinDecoder(data[261:269])
	decoder.Decode(&pool.RewardLastUpdatedTimestamp)

	// Read RewardInfos (384 bytes at offset 269)
	decoder = bin.NewBinDecoder(data[269:653])
	decoder.Decode(&pool.RewardInfos)

	pool.TickArrayCache = make(map[string]*TickArray)

	return nil
}

func (pool *WhirlpoolPool) Quote(ctx context.Context, solClient *sol.Client, inputMint string, amount cosmath.Int) (cosmath.Int, error) {
	// Simplified CLMM quote - uses current pool liquidity without tick array traversal
	// Good approximation for swaps that don't cross many ticks

	if amount.IsZero() {
		return cosmath.ZeroInt(), nil
	}

	// Determine swap direction
	zeroForOne := inputMint == pool.TokenMintA.String()

	// Convert uint128 to cosmath.Int
	sqrtPriceX64 := cosmath.NewIntFromBigInt(pool.SqrtPrice.Big())
	liquidity := cosmath.NewIntFromBigInt(pool.Liquidity.Big())

	// Apply fee (Whirlpool fee rate is in hundredths of a basis point)
	// Fee rate of 200 = 0.02% = 2 basis points
	feeAmount := amount.Mul(cosmath.NewInt(int64(pool.FeeRate))).Quo(cosmath.NewInt(1000000))
	amountAfterFee := amount.Sub(feeAmount)

	// Q64 constant (2^64)
	q64BigInt := new(big.Int).Lsh(big.NewInt(1), 64)

	if liquidity.IsZero() {
		return cosmath.ZeroInt(), fmt.Errorf("pool has zero liquidity")
	}

	if zeroForOne {
		// Swapping token A for token B
		// price = (sqrtPrice / 2^64)^2 = sqrtPrice^2 / 2^128
		// To avoid precision loss, we calculate: (amountIn * sqrtPrice^2) / 2^128
		// This is equivalent to: (amountIn * sqrtPrice * sqrtPrice) / (2^64 * 2^64)

		// Calculate sqrtPrice^2
		sqrtPriceSquared := new(big.Int).Mul(sqrtPriceX64.BigInt(), sqrtPriceX64.BigInt())

		// Calculate amountAfterFee * sqrtPrice^2
		numerator := new(big.Int).Mul(amountAfterFee.BigInt(), sqrtPriceSquared)

		// Calculate 2^128 (Q64 * Q64)
		q128 := new(big.Int).Mul(q64BigInt, q64BigInt)

		// Calculate result
		result := new(big.Int).Div(numerator, q128)

		return cosmath.NewIntFromBigInt(result), nil
	} else {
		// Swapping token B for token A
		// amountOut = amountIn / price = amountIn * 2^128 / sqrtPrice^2

		// Calculate sqrtPrice^2
		sqrtPriceSquared := new(big.Int).Mul(sqrtPriceX64.BigInt(), sqrtPriceX64.BigInt())

		if sqrtPriceSquared.Sign() == 0 {
			return cosmath.ZeroInt(), fmt.Errorf("sqrt price is zero")
		}

		// Calculate 2^128
		q128 := new(big.Int).Mul(q64BigInt, q64BigInt)

		// Calculate amountAfterFee * 2^128
		numerator := new(big.Int).Mul(amountAfterFee.BigInt(), q128)

		// Calculate result
		result := new(big.Int).Div(numerator, sqrtPriceSquared)

		return cosmath.NewIntFromBigInt(result), nil
	}
}

func (pool *WhirlpoolPool) BuildSwapInstructions(
	ctx context.Context,
	solClient *sol.Client,
	user solana.PublicKey,
	inputMint string,
	inputAmount cosmath.Int,
	minOutputAmount cosmath.Int,
	userBaseAccount solana.PublicKey,
	userQuoteAccount solana.PublicKey,
) ([]solana.Instruction, error) {
	// TODO: Implement Whirlpool swap instruction building
	return nil, fmt.Errorf("whirlpool swap instructions not yet implemented - coming soon")
}

// Helper functions for tick math (similar to Raydium CLMM)

func sqrtPriceX64ToPrice(sqrtPriceX64 uint128.Uint128, decimalsA, decimalsB uint8) float64 {
	// Convert sqrt price to actual price
	sqrtPrice := new(big.Float).SetInt(sqrtPriceX64.Big())
	q64 := new(big.Float).SetInt(new(big.Int).Lsh(big.NewInt(1), 64))

	// price = (sqrt_price / 2^64) ^ 2
	price := new(big.Float).Quo(sqrtPrice, q64)
	price.Mul(price, price)

	// Adjust for decimals
	decimalAdjust := new(big.Float).SetFloat64(1e10) // placeholder
	price.Mul(price, decimalAdjust)

	result, _ := price.Float64()
	return result
}
