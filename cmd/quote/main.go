package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"cosmossdk.io/math"
	"github.com/gagliardetto/solana-go"
	"soltrading/pkg/config"
	"soltrading/pkg/protocol"
	"soltrading/pkg/router"
	"soltrading/pkg/sol"
)

type QuoteResponse struct {
	InputMint            string      `json:"inputMint"`
	OutputMint           string      `json:"outputMint"`
	InAmount             string      `json:"inAmount"`
	OutAmount            string      `json:"outAmount"`
	PriceImpact          string      `json:"priceImpact,omitempty"`
	RoutePlan            []RoutePlan `json:"routePlan"`
	SlippageBps          int         `json:"slippageBps"`
	OtherAmountThreshold string      `json:"otherAmountThreshold"`
}

type RoutePlan struct {
	Protocol   string `json:"protocol"`
	PoolID     string `json:"poolId"`
	InputMint  string `json:"inputMint"`
	OutputMint string `json:"outputMint"`
	InAmount   string `json:"inAmount"`
	OutAmount  string `json:"outAmount"`
}

type QuoteError struct {
	Error string `json:"error"`
}

var (
	rpcEndpoints = flag.String("rpc", "", "Comma-separated Solana RPC endpoints (reads from .env if not specified)")
	inputMint    = flag.String("input", "", "Input token mint address (required)")
	outputMint   = flag.String("output", "", "Output token mint address (required)")
	amount       = flag.String("amount", "", "Input amount in smallest units (required)")
	slippageBps  = flag.Int("slippage", 50, "Slippage tolerance in basis points (default: 50 = 0.5%)")
	rateLimit    = flag.Int("ratelimit", 20, "RPC requests per second limit per endpoint (default: 20)")
	jsonOutput   = flag.Bool("json", true, "Output as JSON (default: true)")
	useRpcPool   = flag.Bool("use-pool", true, "Use RPC pool for load balancing (default: true)")
)

func main() {
	// Load .env file
	if err := config.LoadEnv(".env"); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	flag.Parse()

	// Validate required flags
	if *inputMint == "" || *outputMint == "" || *amount == "" {
		fmt.Fprintln(os.Stderr, "Error: Missing required arguments")
		fmt.Fprintln(os.Stderr, "\nUsage:")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nExample:")
		fmt.Fprintln(os.Stderr, "  quote -input So11111111111111111111111111111111111111112 \\")
		fmt.Fprintln(os.Stderr, "        -output EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v \\")
		fmt.Fprintln(os.Stderr, "        -amount 1000000000")
		fmt.Fprintln(os.Stderr, "\nNote: Uses default Helius RPC pool if -rpc not specified")
		os.Exit(1)
	}

	// Parse and validate addresses
	inTokenAddr, err := solana.PublicKeyFromBase58(*inputMint)
	if err != nil {
		outputError(fmt.Sprintf("Invalid input mint address: %v", err))
		os.Exit(1)
	}

	outTokenAddr, err := solana.PublicKeyFromBase58(*outputMint)
	if err != nil {
		outputError(fmt.Sprintf("Invalid output mint address: %v", err))
		os.Exit(1)
	}

	// Parse amount
	amountIn, ok := math.NewIntFromString(*amount)
	if !ok || amountIn.LTE(math.ZeroInt()) {
		outputError("Invalid amount: must be a positive integer")
		os.Exit(1)
	}

	ctx := context.Background()

	// Parse RPC endpoints
	var endpoints []string
	if *rpcEndpoints != "" {
		// User provided endpoints via command line
		endpoints = strings.Split(*rpcEndpoints, ",")
		for i := range endpoints {
			endpoints[i] = strings.TrimSpace(endpoints[i])
		}
	} else {
		// Try to load from .env file
		endpoints = config.GetRPCEndpoints()
		if len(endpoints) == 0 {
			outputError("No RPC endpoints configured. Set RPC_ENDPOINTS in .env or use -rpc flag")
			os.Exit(1)
		}
	}

	// Initialize RPC pool or single client
	var rpcPool *sol.RPCPool
	var solClient *sol.Client

	if *useRpcPool && len(endpoints) > 1 {
		// Use RPC pool for load balancing
		rpcPool, err = sol.NewRPCPool(ctx, endpoints, "", *rateLimit)
		if err != nil {
			outputError(fmt.Sprintf("Failed to create RPC pool: %v", err))
			os.Exit(1)
		}
		solClient = rpcPool.GetClient()
		if !*jsonOutput {
			log.Printf("Using RPC pool with %d endpoints", rpcPool.Size())
		}
	} else {
		// Use single client
		solClient, err = sol.NewClient(ctx, endpoints[0], "", *rateLimit)
		if err != nil {
			outputError(fmt.Sprintf("Failed to create Solana client: %v", err))
			os.Exit(1)
		}
	}

	// Initialize router with all protocols
	r := router.NewSimpleRouter(
		protocol.NewPumpAmm(solClient),
		protocol.NewRaydiumAmm(solClient),
		protocol.NewRaydiumClmm(solClient),
		protocol.NewRaydiumCpmm(solClient),
		protocol.NewMeteoraDlmm(solClient),
	)

	// Query available pools
	if !*jsonOutput {
		log.Printf("Querying available pools for %s -> %s...", *inputMint, *outputMint)
	}

	err = r.QueryAllPools(ctx, inTokenAddr.String(), outTokenAddr.String())
	if err != nil {
		outputError(fmt.Sprintf("Failed to query pools: %v", err))
		os.Exit(1)
	}

	if len(r.Pools) == 0 {
		outputError("No pools found for this token pair")
		os.Exit(1)
	}

	if !*jsonOutput {
		log.Printf("Found %d pools", len(r.Pools))
	}

	// Get best pool and quote
	bestPool, amountOut, err := r.GetBestPool(ctx, solClient, inTokenAddr.String(), amountIn)
	if err != nil {
		outputError(fmt.Sprintf("Failed to get best pool: %v", err))
		os.Exit(1)
	}

	// Calculate minimum amount out with slippage
	minAmountOut := amountOut.Mul(math.NewInt(int64(10000 - *slippageBps))).Quo(math.NewInt(10000))

	// Get protocol name from pool
	protocolName := "unknown"
	for _, p := range r.Protocols {
		for _, pool := range r.Pools {
			if pool.GetID() == bestPool.GetID() {
				protocolName = string(p.ProtocolName())
				break
			}
		}
	}

	// Build response
	response := QuoteResponse{
		InputMint:            inTokenAddr.String(),
		OutputMint:           outTokenAddr.String(),
		InAmount:             amountIn.String(),
		OutAmount:            amountOut.String(),
		SlippageBps:          *slippageBps,
		OtherAmountThreshold: minAmountOut.String(),
		RoutePlan: []RoutePlan{
			{
				Protocol:   protocolName,
				PoolID:     bestPool.GetID(),
				InputMint:  inTokenAddr.String(),
				OutputMint: outTokenAddr.String(),
				InAmount:   amountIn.String(),
				OutAmount:  amountOut.String(),
			},
		},
	}

	// Output result
	if *jsonOutput {
		jsonData, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			outputError(fmt.Sprintf("Failed to marshal JSON: %v", err))
			os.Exit(1)
		}
		fmt.Println(string(jsonData))
	} else {
		fmt.Printf("\n=== Quote Results ===\n")
		fmt.Printf("Route: %s\n", protocolName)
		fmt.Printf("Pool ID: %s\n", bestPool.GetID())
		fmt.Printf("Input: %s %s\n", amountIn.String(), *inputMint)
		fmt.Printf("Output: %s %s\n", amountOut.String(), *outputMint)
		fmt.Printf("Minimum Output (with %d bps slippage): %s\n", *slippageBps, minAmountOut.String())
	}
}

func outputError(msg string) {
	if *jsonOutput {
		errResp := QuoteError{Error: msg}
		jsonData, _ := json.MarshalIndent(errResp, "", "  ")
		fmt.Fprintln(os.Stderr, string(jsonData))
	} else {
		log.Println("Error:", msg)
	}
}
