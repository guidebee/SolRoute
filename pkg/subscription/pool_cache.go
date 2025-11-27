package subscription

import (
	"fmt"
	"log"
	"sync"
	"time"

	"soltrading/pkg"
)

// PoolCacheEntry represents a cached pool with metadata
type PoolCacheEntry struct {
	Pool        pkg.Pool
	LastUpdate  time.Time
	LastSlot    uint64
	AccountData map[string][]byte // account address -> raw data
}

// PoolCache manages cached pool state
type PoolCache struct {
	pools map[string]*PoolCacheEntry
	mu    sync.RWMutex
}

// NewPoolCache creates a new pool cache
func NewPoolCache() *PoolCache {
	return &PoolCache{
		pools: make(map[string]*PoolCacheEntry),
	}
}

// SetPool adds or updates a pool in the cache
func (pc *PoolCache) SetPool(poolID string, pool pkg.Pool) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if entry, exists := pc.pools[poolID]; exists {
		// Update existing entry
		entry.Pool = pool
		entry.LastUpdate = time.Now()
	} else {
		// Create new entry
		pc.pools[poolID] = &PoolCacheEntry{
			Pool:        pool,
			LastUpdate:  time.Now(),
			AccountData: make(map[string][]byte),
		}
	}
}

// GetPool retrieves a pool from the cache
func (pc *PoolCache) GetPool(poolID string) (pkg.Pool, bool) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if entry, exists := pc.pools[poolID]; exists {
		return entry.Pool, true
	}
	return nil, false
}

// GetAllPools returns all pools in the cache
func (pc *PoolCache) GetAllPools() []pkg.Pool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	pools := make([]pkg.Pool, 0, len(pc.pools))
	for _, entry := range pc.pools {
		pools = append(pools, entry.Pool)
	}
	return pools
}

// RemovePool removes a pool from the cache
func (pc *PoolCache) RemovePool(poolID string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	delete(pc.pools, poolID)
}

// UpdatePoolAccount updates account data for a pool
func (pc *PoolCache) UpdatePoolAccount(poolID, accountID string, data []byte, slot uint64) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	entry, exists := pc.pools[poolID]
	if !exists {
		return fmt.Errorf("pool %s not found in cache", poolID)
	}

	// Store raw account data
	entry.AccountData[accountID] = data
	entry.LastUpdate = time.Now()
	entry.LastSlot = slot

	// Try to update the pool with the new data
	if updater, ok := entry.Pool.(PoolStateUpdater); ok {
		if err := updater.UpdateFromAccountData(accountID, data); err != nil {
			log.Printf("Failed to update pool %s state from account %s: %v", poolID, accountID, err)
			return err
		}
		log.Printf("Updated pool %s from account %s at slot %d", poolID, accountID, slot)
	} else {
		log.Printf("Pool %s does not implement PoolStateUpdater interface", poolID)
	}

	return nil
}

// GetPoolEntry returns the full cache entry for a pool
func (pc *PoolCache) GetPoolEntry(poolID string) (*PoolCacheEntry, bool) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	entry, exists := pc.pools[poolID]
	return entry, exists
}

// Size returns the number of cached pools
func (pc *PoolCache) Size() int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	return len(pc.pools)
}

// Clear removes all pools from the cache
func (pc *PoolCache) Clear() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.pools = make(map[string]*PoolCacheEntry)
}

// GetStalePoolIDs returns pool IDs that haven't been updated recently
func (pc *PoolCache) GetStalePoolIDs(maxAge time.Duration) []string {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	now := time.Now()
	stalePools := make([]string, 0)

	for poolID, entry := range pc.pools {
		if now.Sub(entry.LastUpdate) > maxAge {
			stalePools = append(stalePools, poolID)
		}
	}

	return stalePools
}

// PoolStateUpdater is an interface for pools that can update their state from account data
type PoolStateUpdater interface {
	UpdateFromAccountData(accountID string, data []byte) error
}
