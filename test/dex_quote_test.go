package test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/gagliardetto/solana-go"
	"soltrading/pkg"
	"soltrading/pkg/config"
	"soltrading/pkg/protocol"
	"soltrading/pkg/sol"
)

var (
	// SOL (wrapped)
	WSOL = solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	// USDC
	USDC = solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")

	// Test amounts
	ONE_SOL      = math.NewInt(1_000_000_000) // 1 SOL (9 decimals)
	HUNDRED_USDC = math.NewInt(100_000_000)   // 100 USDC (6 decimals)
)

func TestNewDEXQuotes(t *testing.T) {
	// Load .env file from parent directory
	if err := config.LoadEnv("../.env"); err != nil {
		t.Logf("Warning: Could not load .env file: %v", err)
	}

	// Get RPC endpoints
	endpoints := config.GetRPCEndpoints()
	if len(endpoints) == 0 {
		t.Skip("No RPC endpoints configured. Set RPC_ENDPOINTS in .env")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create Solana client
	solClient, err := sol.NewClient(ctx, endpoints[0], "", 20)
	if err != nil {
		t.Fatalf("Failed to create Solana client: %v", err)
	}

	// Initialize all new protocols
	protocols := []struct {
		name     string
		protocol pkg.Protocol
	}{
		{"SPL Token Swap", protocol.NewSplTokenSwap(solClient)},
		{"Aldrin", protocol.NewAldrin(solClient)},
		{"Orca", protocol.NewOrca(solClient)},
		{"GooseFX", protocol.NewGooseFX(solClient)},
		{"Saros", protocol.NewSaros(solClient)},
		{"Fluxbeam", protocol.NewFluxbeam(solClient)},
	}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("Testing New DEX Quote Capabilities")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("\nTest 1: 1 SOL → USDC\n")
	fmt.Printf("Test 2: 100 USDC → SOL\n")
	fmt.Printf("Slippage: 0 bps\n\n")

	for _, p := range protocols {
		fmt.Println(strings.Repeat("-", 80))
		fmt.Printf("Protocol: %s\n", p.name)
		fmt.Println(strings.Repeat("-", 80))

		// Test 1: SOL to USDC
		testQuote(ctx, t, solClient, p.name, p.protocol, WSOL.String(), USDC.String(), ONE_SOL, "1 SOL → USDC")

		// Test 2: USDC to SOL
		testQuote(ctx, t, solClient, p.name, p.protocol, USDC.String(), WSOL.String(), HUNDRED_USDC, "100 USDC → SOL")

		fmt.Println()
	}

	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("Testing Complete")
	fmt.Println(strings.Repeat("=", 80))
}

func testQuote(ctx context.Context, t *testing.T, solClient *sol.Client, protocolName string, protocol pkg.Protocol, inputMint, outputMint string, amount math.Int, label string) {

	// Fetch pools for the pair
	pools, err := protocol.FetchPoolsByPair(ctx, inputMint, outputMint)
	if err != nil {
		fmt.Printf("  ❌ %s: Failed to fetch pools: %v\n", label, err)
		return
	}

	if len(pools) == 0 {
		fmt.Printf("  ⚠️  %s: No pools found\n", label)
		return
	}

	fmt.Printf("  📊 %s: Found %d pool(s)\n", label, len(pools))

	// Test quote on each pool
	bestOutput := math.ZeroInt()
	bestPoolID := ""
	successCount := 0

	for i, pool := range pools {
		output, err := pool.Quote(ctx, solClient, inputMint, amount)
		if err != nil {
			fmt.Printf("     Pool %d [%s...]: Failed - %v\n", i+1, pool.GetID()[:8], err)
			continue
		}

		successCount++

		// Track best output
		if output.GT(bestOutput) {
			bestOutput = output
			bestPoolID = pool.GetID()
		}

		// Format output based on direction
		var outputFormatted string
		if inputMint == WSOL.String() {
			// SOL to USDC: output is in USDC (6 decimals)
			outputFloat := float64(output.Int64()) / 1_000_000
			outputFormatted = fmt.Sprintf("%.6f USDC", outputFloat)
		} else {
			// USDC to SOL: output is in SOL (9 decimals)
			outputFloat := float64(output.Int64()) / 1_000_000_000
			outputFormatted = fmt.Sprintf("%.9f SOL", outputFloat)
		}

		fmt.Printf("     Pool %d [%s...]: %s\n", i+1, pool.GetID()[:8], outputFormatted)
	}

	if successCount > 0 {
		var bestFormatted string
		if inputMint == WSOL.String() {
			outputFloat := float64(bestOutput.Int64()) / 1_000_000
			bestFormatted = fmt.Sprintf("%.6f USDC", outputFloat)
		} else {
			outputFloat := float64(bestOutput.Int64()) / 1_000_000_000
			bestFormatted = fmt.Sprintf("%.9f SOL", outputFloat)
		}
		fmt.Printf("  ✅ Best quote: %s (Pool: %s...)\n", bestFormatted, bestPoolID[:8])
	} else {
		fmt.Printf("  ❌ No successful quotes\n")
	}
}
