package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"cosmossdk.io/math"
	"github.com/gagliardetto/solana-go"
	"soltrading/pkg/protocol"
	"soltrading/pkg/router"
	"soltrading/pkg/sol"
	"soltrading/pkg/subscription"
)

type QuoteCache struct {
	cache           map[string]*CachedQuote
	poolToQuotes    map[string][]QuotePair // Maps poolID to quote pairs that use it
	mu              sync.RWMutex
	solClient       *sol.Client
	rpcPool         *sol.RPCPool
	router          *router.SimpleRouter
	subscriptionMgr *subscription.SubscriptionManager
	refreshInterval time.Duration
	slippageBps     int
	useWebSocket    bool
	ctx             context.Context
}

type QuotePair struct {
	InputMint  string
	OutputMint string
	Amount     string
	Label      string
}

// httpToWsURL converts an HTTP(S) RPC URL to a WebSocket URL
func httpToWsURL(httpURL string) string {
	wsURL := strings.Replace(httpURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	return wsURL
}

func NewQuoteCache(ctx context.Context, endpoints []string, rateLimit int, refreshInterval time.Duration, slippageBps int) (*QuoteCache, error) {
	var rpcPool *sol.RPCPool
	var solClient *sol.Client
	var subscriptionMgr *subscription.SubscriptionManager
	var err error

	if len(endpoints) > 1 {
		rpcPool, err = sol.NewRPCPool(ctx, endpoints, "", rateLimit)
		if err != nil {
			return nil, fmt.Errorf("failed to create RPC pool: %w", err)
		}
		solClient = rpcPool.GetClient()
		log.Printf("Initialized RPC pool with %d endpoints", rpcPool.Size())
	} else {
		solClient, err = sol.NewClient(ctx, endpoints[0], "", rateLimit)
		if err != nil {
			return nil, fmt.Errorf("failed to create Solana client: %w", err)
		}
	}

	// Initialize WebSocket subscription manager using first endpoint
	wsURL := httpToWsURL(endpoints[0])
	log.Printf("Initializing WebSocket connection to %s", wsURL)
	subscriptionMgr, err = subscription.NewSubscriptionManager(ctx, wsURL)
	if err != nil {
		log.Printf("Warning: Failed to create WebSocket subscription manager: %v", err)
		log.Printf("Falling back to RPC-only mode")
		subscriptionMgr = nil
	} else {
		log.Printf("WebSocket subscription manager initialized successfully")
	}

	// Initialize router with all protocols (only DEXs with SOL/USDC pairs)
	r := router.NewSimpleRouter(
		protocol.NewPumpAmm(solClient),
		protocol.NewRaydiumAmm(solClient),
		protocol.NewRaydiumClmm(solClient),
		protocol.NewRaydiumCpmm(solClient),
		protocol.NewMeteoraDlmm(solClient),
		protocol.NewWhirlpool(solClient),
	)

	qc := &QuoteCache{
		cache:           make(map[string]*CachedQuote),
		poolToQuotes:    make(map[string][]QuotePair),
		solClient:       solClient,
		rpcPool:         rpcPool,
		router:          r,
		subscriptionMgr: subscriptionMgr,
		refreshInterval: refreshInterval,
		slippageBps:     slippageBps,
		useWebSocket:    subscriptionMgr != nil,
		ctx:             ctx,
	}

	return qc, nil
}

func (qc *QuoteCache) getCacheKey(inputMint, outputMint, amount string) string {
	return fmt.Sprintf("%s-%s-%s", inputMint, outputMint, amount)
}

func (qc *QuoteCache) GetQuote(inputMint, outputMint, amount string) (*CachedQuote, bool) {
	qc.mu.RLock()
	defer qc.mu.RUnlock()

	key := qc.getCacheKey(inputMint, outputMint, amount)
	quote, exists := qc.cache[key]
	return quote, exists
}

// GetOrCalculateQuote gets a quote from cache or calculates it on-demand
func (qc *QuoteCache) GetOrCalculateQuote(ctx context.Context, inputMint, outputMint, amount string, dexes, excludeDexes []string, minLiquidityUSD float64) (*CachedQuote, error) {
	// Generate cache key
	key := qc.getCacheKey(inputMint, outputMint, amount)

	// Check cache again with lock (only if no filters applied)
	if len(dexes) == 0 && len(excludeDexes) == 0 && minLiquidityUSD == 0 {
		qc.mu.RLock()
		if quote, exists := qc.cache[key]; exists {
			qc.mu.RUnlock()
			return quote, nil
		}
		qc.mu.RUnlock()
	}

	// Parse inputs
	inTokenAddr, err := solana.PublicKeyFromBase58(inputMint)
	if err != nil {
		return nil, fmt.Errorf("invalid input mint: %w", err)
	}

	outTokenAddr, err := solana.PublicKeyFromBase58(outputMint)
	if err != nil {
		return nil, fmt.Errorf("invalid output mint: %w", err)
	}

	amountIn, ok := math.NewIntFromString(amount)
	if !ok || amountIn.LTE(math.ZeroInt()) {
		return nil, fmt.Errorf("invalid amount")
	}

	startTime := time.Now()
	log.Printf("💡 Calculating on-demand quote: %s -> %s, amount: %s", inputMint[:8], outputMint[:8], amount)

	// Check if we already have pools for this pair
	hasPool := false
	qc.mu.RLock()
	for _, quotes := range qc.poolToQuotes {
		for _, q := range quotes {
			if q.InputMint == inputMint && q.OutputMint == outputMint {
				hasPool = true
				break
			}
		}
		if hasPool {
			break
		}
	}
	qc.mu.RUnlock()

	// If no pools for this pair, query them
	if !hasPool {
		err = qc.router.QueryAllPools(ctx, inTokenAddr.String(), outTokenAddr.String())
		if err != nil {
			return nil, fmt.Errorf("failed to query pools: %w", err)
		}

		if len(qc.router.Pools) == 0 {
			return nil, fmt.Errorf("no pools found for this pair")
		}

		// Subscribe to pools via WebSocket if enabled
		if qc.useWebSocket && qc.subscriptionMgr != nil {
			for _, pool := range qc.router.Pools {
				poolID := pool.GetID()
				if _, exists := qc.subscriptionMgr.GetPool(poolID); !exists {
					if err := qc.subscriptionMgr.SubscribePool(pool); err != nil {
						log.Printf("Warning: Failed to subscribe to pool %s: %v", poolID, err)
					} else {
						// Register handler for this pool
						qc.subscriptionMgr.RegisterHandler(poolID, func(updatedPoolID string, data []byte, slot uint64) {
							qc.handlePoolUpdate(updatedPoolID, slot)
						})
					}
				}
			}
			log.Printf("Subscribed to %d pools via WebSocket", len(qc.router.Pools))
		}
	}

	// Get best pool with optional filtering
	bestPool, amountOut, err := qc.router.GetBestPoolWithFilter(ctx, qc.solClient, inTokenAddr.String(), amountIn, dexes, excludeDexes, minLiquidityUSD)
	if err != nil {
		return nil, fmt.Errorf("failed to get best pool: %w", err)
	}

	// Calculate minimum amount out with slippage
	minAmountOut := amountOut.Mul(math.NewInt(int64(10000 - qc.slippageBps))).Quo(math.NewInt(10000))

	// Get protocol name directly from the pool
	protocolName := string(bestPool.ProtocolName())

	// Get pool tokens info
	tokenA, _ := bestPool.GetTokens()
	tokenASymbol := "TokenA"
	tokenBSymbol := "TokenB"

	if tokenA == inputMint {
		tokenASymbol = "Input"
		tokenBSymbol = "Output"
	} else {
		tokenASymbol = "Output"
		tokenBSymbol = "Input"
	}

	// Build cached quote
	quote := &CachedQuote{
		InputMint:            inTokenAddr.String(),
		OutputMint:           outTokenAddr.String(),
		InAmount:             amountIn.String(),
		OutAmount:            amountOut.String(),
		SlippageBps:          qc.slippageBps,
		OtherAmountThreshold: minAmountOut.String(),
		LastUpdate:           time.Now(),
		TimeTaken:            time.Since(startTime).String(),
		RoutePlan: []RoutePlan{
			{
				Protocol:     protocolName,
				PoolID:       bestPool.GetID(),
				PoolAddress:  bestPool.GetID(),
				InputMint:    inTokenAddr.String(),
				OutputMint:   outTokenAddr.String(),
				InAmount:     amountIn.String(),
				OutAmount:    amountOut.String(),
				ProgramID:    bestPool.GetProgramID().String(),
				TokenASymbol: tokenASymbol,
				TokenBSymbol: tokenBSymbol,
			},
		},
	}

	// Store in cache and track pool-to-quote mapping
	qc.mu.Lock()
	qc.cache[key] = quote

	// Track which pool is used for this quote (for WebSocket updates)
	bestPoolID := bestPool.GetID()
	pair := QuotePair{
		InputMint:  inputMint,
		OutputMint: outputMint,
		Amount:     amount,
		Label:      fmt.Sprintf("%s->%s (%s)", inputMint[:8], outputMint[:8], amount),
	}
	found := false
	for _, existingPair := range qc.poolToQuotes[bestPoolID] {
		if existingPair.InputMint == pair.InputMint &&
			existingPair.OutputMint == pair.OutputMint &&
			existingPair.Amount == pair.Amount {
			found = true
			break
		}
	}
	if !found {
		qc.poolToQuotes[bestPoolID] = append(qc.poolToQuotes[bestPoolID], pair)
	}
	qc.mu.Unlock()

	log.Printf("✓ Calculated on-demand quote: %s -> %s (took %s)",
		amountIn.String(),
		amountOut.String(),
		time.Since(startTime).Round(time.Millisecond))

	return quote, nil
}

func (qc *QuoteCache) UpdateQuote(ctx context.Context, pair QuotePair) error {
	startTime := time.Now()

	log.Printf("Updating quote for %s (%s -> %s, amount: %s)", pair.Label, pair.InputMint, pair.OutputMint, pair.Amount)

	inTokenAddr, err := solana.PublicKeyFromBase58(pair.InputMint)
	if err != nil {
		return fmt.Errorf("invalid input mint: %w", err)
	}

	outTokenAddr, err := solana.PublicKeyFromBase58(pair.OutputMint)
	if err != nil {
		return fmt.Errorf("invalid output mint: %w", err)
	}

	amountIn, ok := math.NewIntFromString(pair.Amount)
	if !ok || amountIn.LTE(math.ZeroInt()) {
		return fmt.Errorf("invalid amount")
	}

	// Query pools
	err = qc.router.QueryAllPools(ctx, inTokenAddr.String(), outTokenAddr.String())
	if err != nil {
		return fmt.Errorf("failed to query pools: %w", err)
	}

	if len(qc.router.Pools) == 0 {
		return fmt.Errorf("no pools found")
	}

	// Subscribe to pools via WebSocket if enabled
	if qc.useWebSocket && qc.subscriptionMgr != nil {
		for _, pool := range qc.router.Pools {
			poolID := pool.GetID()
			// Check if already subscribed
			if _, exists := qc.subscriptionMgr.GetPool(poolID); !exists {
				if err := qc.subscriptionMgr.SubscribePool(pool); err != nil {
					log.Printf("Warning: Failed to subscribe to pool %s: %v", poolID, err)
				} else {
					// Register handler for this pool that triggers quote recalculation
					qc.subscriptionMgr.RegisterHandler(poolID, func(updatedPoolID string, data []byte, slot uint64) {
						qc.handlePoolUpdate(updatedPoolID, slot)
					})
				}
			}
		}
		log.Printf("Subscribed to %d pools via WebSocket", len(qc.router.Pools))
	}

	// Get best pool
	bestPool, amountOut, err := qc.router.GetBestPool(ctx, qc.solClient, inTokenAddr.String(), amountIn)
	if err != nil {
		return fmt.Errorf("failed to get best pool: %w", err)
	}

	// Calculate minimum amount out with slippage
	minAmountOut := amountOut.Mul(math.NewInt(int64(10000 - qc.slippageBps))).Quo(math.NewInt(10000))

	// Get protocol name directly from the pool
	protocolName := string(bestPool.ProtocolName())

	// Get pool tokens info
	tokenA, _ := bestPool.GetTokens()
	tokenASymbol := "TokenA"
	tokenBSymbol := "TokenB"

	if tokenA == pair.InputMint {
		tokenASymbol = "Input"
		tokenBSymbol = "Output"
	} else {
		tokenASymbol = "Output"
		tokenBSymbol = "Input"
	}

	// Build cached quote
	quote := &CachedQuote{
		InputMint:            inTokenAddr.String(),
		OutputMint:           outTokenAddr.String(),
		InAmount:             amountIn.String(),
		OutAmount:            amountOut.String(),
		SlippageBps:          qc.slippageBps,
		OtherAmountThreshold: minAmountOut.String(),
		LastUpdate:           time.Now(),
		TimeTaken:            time.Since(startTime).String(),
		RoutePlan: []RoutePlan{
			{
				Protocol:     protocolName,
				PoolID:       bestPool.GetID(),
				PoolAddress:  bestPool.GetID(),
				InputMint:    inTokenAddr.String(),
				OutputMint:   outTokenAddr.String(),
				InAmount:     amountIn.String(),
				OutAmount:    amountOut.String(),
				ProgramID:    bestPool.GetProgramID().String(),
				TokenASymbol: tokenASymbol,
				TokenBSymbol: tokenBSymbol,
			},
		},
	}

	// Store in cache and track pool-to-quote mapping
	qc.mu.Lock()
	key := qc.getCacheKey(pair.InputMint, pair.OutputMint, pair.Amount)
	qc.cache[key] = quote

	// Track which pool is used for this quote pair (for WebSocket updates)
	bestPoolID := bestPool.GetID()
	found := false
	for _, existingPair := range qc.poolToQuotes[bestPoolID] {
		if existingPair.InputMint == pair.InputMint &&
			existingPair.OutputMint == pair.OutputMint &&
			existingPair.Amount == pair.Amount {
			found = true
			break
		}
	}
	if !found {
		qc.poolToQuotes[bestPoolID] = append(qc.poolToQuotes[bestPoolID], pair)
	}
	qc.mu.Unlock()

	log.Printf("✓ Updated %s: %s %s -> %s %s (took %s)",
		pair.Label,
		amountIn.String(),
		pair.InputMint[:8],
		amountOut.String(),
		pair.OutputMint[:8],
		time.Since(startTime).Round(time.Millisecond))

	return nil
}

// handlePoolUpdate is called when a pool is updated via WebSocket
func (qc *QuoteCache) handlePoolUpdate(poolID string, slot uint64) {
	qc.mu.RLock()
	quotePairs, exists := qc.poolToQuotes[poolID]
	qc.mu.RUnlock()

	if !exists || len(quotePairs) == 0 {
		return
	}

	log.Printf("🔄 Pool %s updated (slot %d), recalculating %d quotes", poolID[:8], slot, len(quotePairs))

	// Recalculate all quotes that use this pool
	for _, pair := range quotePairs {
		if err := qc.recalculateQuote(qc.ctx, pair, poolID); err != nil {
			log.Printf("Error recalculating quote for %s: %v", pair.Label, err)
		}
	}
}

// recalculateQuote recalculates a single quote using the updated pool data from cache
func (qc *QuoteCache) recalculateQuote(ctx context.Context, pair QuotePair, poolID string) error {
	startTime := time.Now()

	inTokenAddr, err := solana.PublicKeyFromBase58(pair.InputMint)
	if err != nil {
		return fmt.Errorf("invalid input mint: %w", err)
	}

	outTokenAddr, err := solana.PublicKeyFromBase58(pair.OutputMint)
	if err != nil {
		return fmt.Errorf("invalid output mint: %w", err)
	}

	amountIn, ok := math.NewIntFromString(pair.Amount)
	if !ok || amountIn.LTE(math.ZeroInt()) {
		return fmt.Errorf("invalid amount")
	}

	// Get the updated pool from subscription manager cache
	pool, exists := qc.subscriptionMgr.GetPool(poolID)
	if !exists {
		return fmt.Errorf("pool %s not found in cache", poolID)
	}

	// Get old quote for comparison
	qc.mu.RLock()
	key := qc.getCacheKey(pair.InputMint, pair.OutputMint, pair.Amount)
	oldQuote, hadOldQuote := qc.cache[key]
	qc.mu.RUnlock()

	// Quote using the cached pool data (no RPC call needed!)
	amountOut, err := pool.Quote(ctx, qc.solClient, inTokenAddr.String(), amountIn)
	if err != nil {
		return fmt.Errorf("failed to quote: %w", err)
	}

	// Calculate minimum amount out with slippage
	minAmountOut := amountOut.Mul(math.NewInt(int64(10000 - qc.slippageBps))).Quo(math.NewInt(10000))

	// Get protocol name directly from the pool
	protocolName := string(pool.ProtocolName())

	// Get pool tokens info
	tokenA, _ := pool.GetTokens()
	tokenASymbol := "TokenA"
	tokenBSymbol := "TokenB"

	if tokenA == pair.InputMint {
		tokenASymbol = "Input"
		tokenBSymbol = "Output"
	} else {
		tokenASymbol = "Output"
		tokenBSymbol = "Input"
	}

	// Build cached quote
	quote := &CachedQuote{
		InputMint:            inTokenAddr.String(),
		OutputMint:           outTokenAddr.String(),
		InAmount:             amountIn.String(),
		OutAmount:            amountOut.String(),
		SlippageBps:          qc.slippageBps,
		OtherAmountThreshold: minAmountOut.String(),
		LastUpdate:           time.Now(),
		TimeTaken:            time.Since(startTime).String(),
		RoutePlan: []RoutePlan{
			{
				Protocol:     protocolName,
				PoolID:       pool.GetID(),
				PoolAddress:  pool.GetID(),
				InputMint:    inTokenAddr.String(),
				OutputMint:   outTokenAddr.String(),
				InAmount:     amountIn.String(),
				OutAmount:    amountOut.String(),
				ProgramID:    pool.GetProgramID().String(),
				TokenASymbol: tokenASymbol,
				TokenBSymbol: tokenBSymbol,
			},
		},
	}

	// Store in cache
	qc.mu.Lock()
	qc.cache[key] = quote
	qc.mu.Unlock()

	// Log with price change comparison
	if hadOldQuote {
		oldAmount, ok1 := math.NewIntFromString(oldQuote.OutAmount)
		newAmount := amountOut

		if ok1 && !oldAmount.IsZero() {
			// Calculate absolute and percentage change
			diff := newAmount.Sub(oldAmount)
			// Calculate percentage: (diff / oldAmount) * 100
			// To avoid precision loss, multiply by 10000 first, then divide
			percentChange := diff.Mul(math.NewInt(10000)).Quo(oldAmount)
			percentChangeFloat := float64(percentChange.Int64()) / 100.0

			changeSymbol := "→"
			if diff.IsPositive() {
				changeSymbol = "↑"
			} else if diff.IsNegative() {
				changeSymbol = "↓"
			}

			log.Printf("💰 %s %s: %s → %s (%s%+.4f%%, %s%s) [%s] %s",
				changeSymbol,
				pair.Label,
				oldAmount.String(),
				newAmount.String(),
				changeSymbol,
				percentChangeFloat,
				changeSymbol,
				diff.String(),
				protocolName,
				time.Since(startTime).Round(time.Millisecond))
		} else {
			log.Printf("✓ Recalculated %s: %s -> %s [%s] (took %s)",
				pair.Label,
				amountIn.String(),
				amountOut.String(),
				protocolName,
				time.Since(startTime).Round(time.Millisecond))
		}
	} else {
		log.Printf("✓ First calculation %s: %s -> %s [%s] (took %s)",
			pair.Label,
			amountIn.String(),
			amountOut.String(),
			protocolName,
			time.Since(startTime).Round(time.Millisecond))
	}

	return nil
}

func (qc *QuoteCache) RefreshAll(ctx context.Context, pairs []QuotePair) {
	for _, pair := range pairs {
		if err := qc.UpdateQuote(ctx, pair); err != nil {
			log.Printf("Error updating quote for %s: %v", pair.Label, err)
		}
	}
}

func (qc *QuoteCache) StartPeriodicRefresh(ctx context.Context, pairs []QuotePair) {
	// Initial refresh (always needed to populate cache and subscribe to pools)
	log.Printf("Starting initial quote refresh...")
	qc.RefreshAll(ctx, pairs)
	log.Printf("Initial refresh complete")

	// Set up periodic refresh as fallback
	var fallbackInterval time.Duration
	if qc.useWebSocket {
		// When WebSocket is enabled, use much longer interval as fallback
		fallbackInterval = qc.refreshInterval * 10 // e.g., 30s * 10 = 5 minutes
		log.Printf("WebSocket enabled: Using %v fallback refresh interval", fallbackInterval)
	} else {
		// When WebSocket is disabled, use the configured interval
		fallbackInterval = qc.refreshInterval
		log.Printf("WebSocket disabled: Using %v refresh interval", fallbackInterval)
	}

	ticker := time.NewTicker(fallbackInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Stopping periodic refresh")
			return
		case <-ticker.C:
			if qc.useWebSocket {
				log.Printf("Running fallback refresh (WebSocket primary)...")
			} else {
				log.Printf("Starting periodic refresh...")
			}
			qc.RefreshAll(ctx, pairs)
			if qc.useWebSocket {
				log.Printf("Fallback refresh complete")
			} else {
				log.Printf("Periodic refresh complete")
			}
		}
	}
}

func (qc *QuoteCache) GetAllCached() map[string]*CachedQuote {
	qc.mu.RLock()
	defer qc.mu.RUnlock()

	result := make(map[string]*CachedQuote)
	for k, v := range qc.cache {
		result[k] = v
	}
	return result
}
