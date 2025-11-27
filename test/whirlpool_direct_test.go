package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"soltrading/pkg/config"
	"soltrading/pkg/pool/whirlpool"
	"soltrading/pkg/protocol"
	"soltrading/pkg/sol"
)

func TestWhirlpoolDirectFetch(t *testing.T) {
	// Load .env file
	if err := config.LoadEnv("../.env"); err != nil {
		t.Logf("Warning: Could not load .env file: %v", err)
	}

	// Get RPC endpoints
	endpoints := config.GetRPCEndpoints()
	if len(endpoints) == 0 {
		t.Skip("No RPC endpoints configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	solClient, err := sol.NewClient(ctx, endpoints[0], "", 20)
	if err != nil {
		t.Fatalf("Failed to create Solana client: %v", err)
	}

	whirlpoolProtocol := protocol.NewWhirlpool(solClient)

	// Fetch the specific SOL/USDC pool
	poolID := "FpCMFDFGYotvufJ7HrFHsWEiiQCGbkLCtwHiDnh7o28Q"
	fmt.Printf("Fetching Whirlpool pool: %s\n", poolID)

	pool, err := whirlpoolProtocol.FetchPoolByID(ctx, poolID)
	if err != nil {
		t.Fatalf("Failed to fetch pool: %v", err)
	}

	tokenA, tokenB := pool.GetTokens()
	whirlpoolPool := pool.(*whirlpool.WhirlpoolPool)
	fmt.Printf("Pool ID: %s\n", pool.GetID())
	fmt.Printf("Token A: %s\n", tokenA)
	fmt.Printf("Token B: %s\n", tokenB)
	fmt.Printf("Protocol: %s\n", pool.ProtocolName())
	fmt.Printf("SqrtPrice: %s\n", whirlpoolPool.SqrtPrice.Big().String())
	fmt.Printf("Liquidity: %s\n", whirlpoolPool.Liquidity.Big().String())
	fmt.Printf("FeeRate: %d\n", whirlpoolPool.FeeRate)
	fmt.Printf("TickSpacing: %d\n", whirlpoolPool.TickSpacing)

	// Verify it's SOL/USDC
	if tokenA == WSOL.String() || tokenB == WSOL.String() {
		fmt.Println("✅ Pool contains WSOL")
	} else {
		t.Errorf("Pool doesn't contain WSOL. TokenA: %s, TokenB: %s", tokenA, tokenB)
	}

	if tokenA == USDC.String() || tokenB == USDC.String() {
		fmt.Println("✅ Pool contains USDC")
	} else {
		t.Errorf("Pool doesn't contain USDC. TokenA: %s, TokenB: %s", tokenA, tokenB)
	}

	// Try to get a quote
	fmt.Println("\nTesting quote for 1 SOL...")
	testAmount := ONE_SOL

	inputMint := ""
	if tokenA == WSOL.String() {
		inputMint = WSOL.String()
	} else {
		inputMint = tokenB
	}

	quote, err := pool.Quote(ctx, solClient, inputMint, testAmount)
	if err != nil {
		t.Fatalf("Failed to get quote: %v", err)
	}

	outputUSDC := float64(quote.Int64()) / 1_000_000
	fmt.Printf("Quote: 1 SOL = %.6f USDC\n", outputUSDC)

	if outputUSDC > 0 && outputUSDC < 1000 {
		fmt.Println("✅ Quote looks reasonable")
	} else {
		t.Errorf("Quote looks unreasonable: %.6f USDC", outputUSDC)
	}
}
