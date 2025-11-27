package test

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	cosmath "cosmossdk.io/math"
	"soltrading/pkg/config"
	"soltrading/pkg/protocol"
	"soltrading/pkg/sol"
)

func TestWhirlpoolVsRaydiumCLMM(t *testing.T) {
	// Load .env file
	if err := config.LoadEnv("../.env"); err != nil {
		t.Logf("Warning: Could not load .env file: %v", err)
	}

	// Get RPC endpoints
	endpoints := config.GetRPCEndpoints()
	if len(endpoints) == 0 {
		t.Skip("No RPC endpoints configured. Set RPC_ENDPOINTS in .env")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Create Solana client
	solClient, err := sol.NewClient(ctx, endpoints[0], "", 20)
	if err != nil {
		t.Fatalf("Failed to create Solana client: %v", err)
	}

	// Initialize protocols
	whirlpoolProtocol := protocol.NewWhirlpool(solClient)
	raydiumCLMMProtocol := protocol.NewRaydiumClmm(solClient)

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("Whirlpool vs Raydium CLMM Comparison Test")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("\nTest: SOL/USDC Quote Comparison")
	fmt.Println("Amount: 1 SOL (1,000,000,000 lamports)")

	// Test amount: 1 SOL
	testAmount := cosmath.NewInt(1_000_000_000)

	// Use specific known pools to avoid RPC pagination issues
	// These are the largest/most liquid pools for SOL/USDC on each protocol
	whirlpoolPoolID := "FpCMFDFGYotvufJ7HrFHsWEiiQCGbkLCtwHiDnh7o28Q" // Whirlpool SOL/USDC
	raydiumPoolID := "61R1ndXxvsWXXkWSyNkCxnzwd3zUNB8Q2ibmkiLPC8ht"   // Raydium CLMM SOL/USDC

	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Println("Fetching Whirlpool pool...")
	whirlpoolPool, err := whirlpoolProtocol.FetchPoolByID(ctx, whirlpoolPoolID)
	if err != nil {
		t.Fatalf("Failed to fetch Whirlpool pool: %v", err)
	}
	fmt.Printf("Whirlpool pool: %s\n", whirlpoolPoolID[:16])

	fmt.Println("\nFetching Raydium CLMM pool...")
	raydiumPool, err := raydiumCLMMProtocol.FetchPoolByID(ctx, raydiumPoolID)
	if err != nil {
		t.Fatalf("Failed to fetch Raydium CLMM pool: %v", err)
	}
	fmt.Printf("Raydium CLMM pool: %s\n", raydiumPoolID[:16])

	// Get quotes from both
	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Println("Calculating quotes...")

	// Get Whirlpool quote
	whirlpoolQuote, err := whirlpoolPool.Quote(ctx, solClient, WSOL.String(), testAmount)
	if err != nil {
		t.Fatalf("Failed to get Whirlpool quote: %v", err)
	}

	// Get Raydium CLMM quote
	raydiumQuote, err := raydiumPool.Quote(ctx, solClient, WSOL.String(), testAmount)
	if err != nil {
		t.Fatalf("Failed to get Raydium CLMM quote: %v", err)
	}

	// Format results
	whirlpoolUSDC := float64(whirlpoolQuote.Int64()) / 1_000_000
	raydiumUSDC := float64(raydiumQuote.Int64()) / 1_000_000

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("Results:")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Whirlpool:    %.6f USDC (Pool: %s...)\n", whirlpoolUSDC, whirlpoolPoolID[:8])
	fmt.Printf("Raydium CLMM: %.6f USDC (Pool: %s...)\n", raydiumUSDC, raydiumPoolID[:8])

	// Calculate deviation
	deviation := math.Abs(whirlpoolUSDC-raydiumUSDC) / raydiumUSDC * 100
	fmt.Printf("\nDeviation: %.2f%%\n", deviation)

	// Determine if deviation is acceptable
	const maxAcceptableDeviation = 5.0 // 5%

	if deviation > maxAcceptableDeviation {
		fmt.Printf("\n⚠️  WARNING: Deviation (%.2f%%) exceeds acceptable threshold (%.2f%%)\n",
			deviation, maxAcceptableDeviation)
		fmt.Println("This may indicate implementation issues with Whirlpool quote calculation")
		t.Errorf("Quote deviation too high: %.2f%% > %.2f%%", deviation, maxAcceptableDeviation)
	} else {
		fmt.Printf("\n✅ PASS: Deviation (%.2f%%) within acceptable range (%.2f%%)\n",
			deviation, maxAcceptableDeviation)
		fmt.Println("Whirlpool implementation validated!")
	}

	fmt.Println(strings.Repeat("=", 80))
}
