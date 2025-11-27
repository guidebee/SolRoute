package test

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"soltrading/pkg/config"
	"soltrading/pkg/sol"
)

func TestWhirlpoolDebugStructure(t *testing.T) {
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

	// Fetch the specific SOL/USDC pool
	poolID := "FpCMFDFGYotvufJ7HrFHsWEiiQCGbkLCtwHiDnh7o28Q"
	poolPubkey, err := solana.PublicKeyFromBase58(poolID)
	if err != nil {
		t.Fatalf("Invalid pool ID: %v", err)
	}

	account, err := solClient.GetAccountInfoWithOpts(ctx, poolPubkey)
	if err != nil {
		t.Fatalf("Failed to get pool account: %v", err)
	}

	data := account.Value.Data.GetBinary()
	fmt.Printf("Account data size: %d bytes\n", len(data))
	fmt.Printf("\n=== Account Data Analysis ===\n\n")

	// Known addresses we're looking for:
	wsol := "So11111111111111111111111111111111111111112"
	usdc := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

	wsolPubkey, _ := solana.PublicKeyFromBase58(wsol)
	usdcPubkey, _ := solana.PublicKeyFromBase58(usdc)

	fmt.Printf("Looking for WSOL: %s\n", wsol)
	fmt.Printf("  Bytes: %s\n", hex.EncodeToString(wsolPubkey.Bytes()))
	fmt.Printf("Looking for USDC: %s\n", usdc)
	fmt.Printf("  Bytes: %s\n\n", hex.EncodeToString(usdcPubkey.Bytes()))

	// Search for these addresses in the account data
	wsolBytes := wsolPubkey.Bytes()
	usdcBytes := usdcPubkey.Bytes()

	for i := 0; i < len(data)-32; i++ {
		// Check if this position matches WSOL
		if string(data[i:i+32]) == string(wsolBytes) {
			fmt.Printf("Found WSOL at offset %d (0x%X)\n", i, i)
		}
		// Check if this position matches USDC
		if string(data[i:i+32]) == string(usdcBytes) {
			fmt.Printf("Found USDC at offset %d (0x%X)\n", i, i)
		}
	}

	// Print first 200 bytes in hex for manual inspection
	fmt.Printf("\n=== First 200 bytes (hex) ===\n")
	for i := 0; i < 200 && i < len(data); i += 32 {
		end := i + 32
		if end > len(data) {
			end = len(data)
		}
		fmt.Printf("Offset %3d (0x%02X): %s\n", i, i, hex.EncodeToString(data[i:end]))
	}

	// Try to decode what we think are at offsets 41 and 73
	if len(data) >= 105 {
		fmt.Printf("\n=== Current Decode Offsets ===\n")
		fmt.Printf("Offset 41 (TokenMintA in protocol filters):\n")
		pubkey41 := solana.PublicKeyFromBytes(data[41:73])
		fmt.Printf("  Address: %s\n", pubkey41.String())

		fmt.Printf("Offset 73 (TokenMintB in protocol filters):\n")
		pubkey73 := solana.PublicKeyFromBytes(data[73:105])
		fmt.Printf("  Address: %s\n", pubkey73.String())
	}
}
