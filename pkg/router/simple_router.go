package router

import (
	"context"
	"fmt"
	"log"
	"sync"

	"cosmossdk.io/math"
	"soltrading/pkg"
	"soltrading/pkg/pool/pump"
	"soltrading/pkg/pool/raydium"
	"soltrading/pkg/sol"
)

type SimpleRouter struct {
	Protocols []pkg.Protocol
	Pools     []pkg.Pool
}

func NewSimpleRouter(protocols ...pkg.Protocol) *SimpleRouter {
	return &SimpleRouter{
		Protocols: protocols,
		Pools:     []pkg.Pool{},
	}
}

func (r *SimpleRouter) QueryAllPools(ctx context.Context, baseMint, quoteMint string) error {
	var allPools []pkg.Pool

	// Loop through each protocol sequentially
	for _, proto := range r.Protocols {
		log.Printf("😈Fetching pools from protocol: %v", proto.ProtocolName())
		pools, err := proto.FetchPoolsByPair(ctx, baseMint, quoteMint)
		if err != nil {
			log.Printf("error fetching pools from protocol: %v", err)
			continue
		}
		allPools = append(allPools, pools...)
	}

	r.Pools = allPools
	return nil
}

func (r *SimpleRouter) GetBestPool(ctx context.Context, solClient *sol.Client, tokenIn string, amountIn math.Int) (pkg.Pool, math.Int, error) {
	return r.GetBestPoolWithFilter(ctx, solClient, tokenIn, amountIn, nil, nil, 0)
}

func (r *SimpleRouter) GetBestPoolWithFilter(ctx context.Context, solClient *sol.Client, tokenIn string, amountIn math.Int, dexes, excludeDexes []string, minLiquidityUSD float64) (pkg.Pool, math.Int, error) {
	// Filter pools based on protocol names and liquidity
	filteredPools := r.filterPools(dexes, excludeDexes, minLiquidityUSD, tokenIn)

	if len(filteredPools) == 0 {
		return nil, math.ZeroInt(), fmt.Errorf("no pools found after filtering")
	}

	type quoteResult struct {
		pool      pkg.Pool
		outAmount math.Int
		err       error
	}

	// Create a channel to collect results
	resultChan := make(chan quoteResult, len(filteredPools))
	var wg sync.WaitGroup

	// Launch goroutines for each pool
	for _, pool := range filteredPools {
		wg.Add(1)
		go func(p pkg.Pool) {
			defer wg.Done()
			outAmount, err := p.Quote(ctx, solClient, tokenIn, amountIn)
			resultChan <- quoteResult{
				pool:      p,
				outAmount: outAmount,
				err:       err,
			}
		}(pool)
	}

	// Close the channel when all goroutines are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results and find the best one
	var best pkg.Pool
	maxOut := math.NewInt(0)

	for result := range resultChan {
		if result.err != nil {
			log.Printf("error quoting pool %s: %v", result.pool.GetID(), result.err)
			continue
		}
		if result.outAmount.GT(maxOut) {
			maxOut = result.outAmount
			best = result.pool
		}
	}

	if best == nil {
		return nil, math.ZeroInt(), fmt.Errorf("no route found")
	}
	return best, maxOut, nil
}

// getPoolLiquidity estimates the pool liquidity in USD based on reserves
// For simplicity, we assume the output token (non-input) reserve represents USD value
// This works well for WSOL/USDC pairs where USDC ≈ $1
func getPoolLiquidity(pool pkg.Pool, tokenIn string) float64 {
	tokenA, _ := pool.GetTokens()

	// Determine which reserve to check (the output token side)
	var liquidityRaw math.Int

	// Try to extract reserves based on pool type
	switch p := pool.(type) {
	case *raydium.AMMPool:
		// Raydium AMM pools
		if tokenA == tokenIn {
			// Input is tokenA, check tokenB reserve (quote)
			liquidityRaw = p.QuoteReserve
		} else {
			// Input is tokenB, check tokenA reserve (base)
			liquidityRaw = p.BaseReserve
		}
	case *raydium.CPMMPool:
		// Raydium CPMM pools (same structure as AMM)
		if tokenA == tokenIn {
			liquidityRaw = p.QuoteReserve
		} else {
			liquidityRaw = p.BaseReserve
		}
	case *pump.PumpAMMPool:
		// Pump AMM pools
		if tokenA == tokenIn {
			// Input is tokenA (base), check tokenB (quote)
			liquidityRaw = p.QuoteAmount
		} else {
			// Input is tokenB (quote), check tokenA (base)
			liquidityRaw = p.BaseAmount
		}
	default:
		// For CLMM/DLMM/Whirlpool pools, we can't easily extract reserves without context
		// Return a high value to not filter them out for now
		return 1000000.0
	}

	if liquidityRaw.IsNil() || liquidityRaw.IsZero() {
		return 0
	}

	// Convert to float with decimals adjustment (assume 6 decimals for stables/SOL)
	liquidityFloat := float64(liquidityRaw.Int64()) / float64(1e6)
	return liquidityFloat
}

// filterPools filters the pools based on dexes, excludeDexes, and minimum liquidity
func (r *SimpleRouter) filterPools(dexes, excludeDexes []string, minLiquidityUSD float64, tokenIn string) []pkg.Pool {
	// If no filters provided, return all pools
	if len(dexes) == 0 && len(excludeDexes) == 0 && minLiquidityUSD == 0 {
		return r.Pools
	}

	var filtered []pkg.Pool

	for _, pool := range r.Pools {
		protocolName := string(pool.ProtocolName())

		// If dexes is specified, only include matching protocols
		if len(dexes) > 0 {
			found := false
			for _, dex := range dexes {
				if protocolName == dex {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// If excludeDexes is specified, skip matching protocols
		if len(excludeDexes) > 0 {
			excluded := false
			for _, excludeDex := range excludeDexes {
				if protocolName == excludeDex {
					excluded = true
					break
				}
			}
			if excluded {
				continue
			}
		}

		// If minLiquidity is specified, check pool liquidity
		if minLiquidityUSD > 0 {
			liquidity := getPoolLiquidity(pool, tokenIn)
			if liquidity < minLiquidityUSD {
				log.Printf("Filtering out pool %s with low liquidity: $%.2f < $%.2f", pool.GetID()[:8], liquidity, minLiquidityUSD)
				continue
			}
		}

		filtered = append(filtered, pool)
	}

	return filtered
}
