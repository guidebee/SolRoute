package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"cosmossdk.io/math"
	"github.com/gagliardetto/solana-go"
	"soltrading/pkg/config"
)

var (
	rpcEndpoints    = flag.String("rpc", "", "Comma-separated Solana RPC endpoints (uses default pool if empty)")
	port            = flag.Int("port", 8080, "HTTP server port")
	refreshInterval = flag.Int("refresh", 30, "Quote refresh interval in seconds")
	rateLimit       = flag.Int("ratelimit", 20, "RPC requests per second per endpoint")
	slippageBps     = flag.Int("slippage", 50, "Slippage tolerance in basis points")
)

var (
	// SOL (wrapped)
	WSOL = solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	// USDC
	USDC = solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")

	// Default amounts to quote
	ONE_SOL  = "1000000000" // 1 SOL (9 decimals)
	TEN_USDC = "10000000"   // 10 USDC (6 decimals)
)

var (
	quoteCache *QuoteCache
	startTime  time.Time
)

func main() {
	// Load .env file
	if err := config.LoadEnv(".env"); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	flag.Parse()

	startTime = time.Now()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Parse RPC endpoints
	var endpoints []string
	if *rpcEndpoints != "" {
		// User provided endpoints via command line
		endpoints = strings.Split(*rpcEndpoints, ",")
		for i := range endpoints {
			endpoints[i] = strings.TrimSpace(endpoints[i])
		}
	} else {
		// Load from .env file
		endpoints = config.GetRPCEndpoints()
		if len(endpoints) == 0 {
			log.Fatalf("No RPC endpoints configured. Set RPC_ENDPOINTS in .env or use -rpc flag")
		}
	}

	log.Printf("Starting SolRoute Quote Service")
	log.Printf("Port: %d", *port)
	log.Printf("Refresh interval: %d seconds", *refreshInterval)
	log.Printf("RPC endpoints: %d", len(endpoints))
	log.Printf("Slippage: %d bps", *slippageBps)

	// Initialize quote cache
	var err error
	quoteCache, err = NewQuoteCache(
		ctx,
		endpoints,
		*rateLimit,
		time.Duration(*refreshInterval)*time.Second,
		*slippageBps,
	)
	if err != nil {
		log.Fatalf("Failed to create quote cache: %v", err)
	}

	// Define quote pairs to monitor
	quotePairs := []QuotePair{
		{
			InputMint:  WSOL.String(),
			OutputMint: USDC.String(),
			Amount:     ONE_SOL,
			Label:      "SOL->USDC (1 SOL)",
		},
		{
			InputMint:  USDC.String(),
			OutputMint: WSOL.String(),
			Amount:     TEN_USDC,
			Label:      "USDC->SOL (10 USDC)",
		},
	}

	// Start periodic refresh in background
	go quoteCache.StartPeriodicRefresh(ctx, quotePairs)

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/quote", handleQuote)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/", handleRoot)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: corsMiddleware(mux),
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
		cancel()
	}()

	log.Printf("Server listening on http://localhost:%d", *port)
	log.Printf("Endpoints:")
	log.Printf("  GET  /quote?input=<mint>&output=<mint>&amount=<amount>&slippageBps=<bps>&dexes=<comma-separated>&excludeDexes=<comma-separated>&minLiquidity=<usd>")
	log.Printf("  GET  /health")
	log.Printf("  GET  /")

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}

	log.Println("Server stopped")
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	allQuotes := quoteCache.GetAllCached()
	response := map[string]interface{}{
		"service":      "SolRoute Quote Service",
		"status":       "running",
		"cachedQuotes": len(allQuotes),
		"quotes":       allQuotes,
		"endpoints": map[string]string{
			"quote":  "/quote?input=<mint>&output=<mint>&amount=<amount>",
			"health": "/health",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleQuote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	inputMint := r.URL.Query().Get("input")
	outputMint := r.URL.Query().Get("output")
	amount := r.URL.Query().Get("amount")
	slippageParam := r.URL.Query().Get("slippageBps")
	dexesParam := r.URL.Query().Get("dexes")
	excludeDexesParam := r.URL.Query().Get("excludeDexes")
	minLiquidityParam := r.URL.Query().Get("minLiquidity")

	if inputMint == "" || outputMint == "" || amount == "" {
		writeError(w, "Missing required parameters: input, output, amount", http.StatusBadRequest)
		return
	}

	// Parse DEX filters
	var dexes, excludeDexes []string
	if dexesParam != "" {
		dexes = strings.Split(dexesParam, ",")
		for i := range dexes {
			dexes[i] = strings.TrimSpace(dexes[i])
		}
	}
	if excludeDexesParam != "" {
		excludeDexes = strings.Split(excludeDexesParam, ",")
		for i := range excludeDexes {
			excludeDexes[i] = strings.TrimSpace(excludeDexes[i])
		}
	}

	// Parse minimum liquidity filter (in USD)
	var minLiquidityUSD float64
	if minLiquidityParam != "" {
		parsedLiquidity, err := strconv.ParseFloat(minLiquidityParam, 64)
		if err != nil || parsedLiquidity < 0 {
			writeError(w, "Invalid minLiquidity parameter (must be positive number)", http.StatusBadRequest)
			return
		}
		minLiquidityUSD = parsedLiquidity
	}

	// Try to get from cache first (only if no filters applied)
	var quote *CachedQuote
	var exists bool
	if len(dexes) == 0 && len(excludeDexes) == 0 && minLiquidityUSD == 0 {
		quote, exists = quoteCache.GetQuote(inputMint, outputMint, amount)
	}

	// If not in cache or filters applied, calculate on-demand using pool data
	if !exists {
		var err error
		quote, err = quoteCache.GetOrCalculateQuote(r.Context(), inputMint, outputMint, amount, dexes, excludeDexes, minLiquidityUSD)
		if err != nil {
			writeError(w, fmt.Sprintf("Failed to calculate quote: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Apply custom slippage if provided
	if slippageParam != "" {
		customSlippage, err := strconv.Atoi(slippageParam)
		if err != nil || customSlippage < 0 || customSlippage > 10000 {
			writeError(w, "Invalid slippageBps parameter (must be 0-10000)", http.StatusBadRequest)
			return
		}

		// Recalculate threshold with custom slippage
		outAmount, ok := math.NewIntFromString(quote.OutAmount)
		if ok {
			minAmountOut := outAmount.Mul(math.NewInt(int64(10000 - customSlippage))).Quo(math.NewInt(10000))

			// Create a copy of the quote with updated slippage
			modifiedQuote := *quote
			modifiedQuote.SlippageBps = customSlippage
			modifiedQuote.OtherAmountThreshold = minAmountOut.String()
			quote = &modifiedQuote
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(quote)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	allQuotes := quoteCache.GetAllCached()

	var lastUpdate time.Time
	for _, quote := range allQuotes {
		if quote.LastUpdate.After(lastUpdate) {
			lastUpdate = quote.LastUpdate
		}
	}

	health := HealthResponse{
		Status:       "healthy",
		LastUpdate:   lastUpdate,
		CachedRoutes: len(allQuotes),
		Uptime:       time.Since(startTime).Round(time.Second).String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func writeError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(QuoteError{Error: message})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
