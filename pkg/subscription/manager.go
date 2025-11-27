package subscription

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"sync"
	"time"

	"soltrading/pkg"
)

// PoolUpdateHandler is called when a pool's state is updated
type PoolUpdateHandler func(poolID string, data []byte, slot uint64)

// SubscriptionManager manages pool account subscriptions
type SubscriptionManager struct {
	wsClient      *WebSocketClient
	poolCache     *PoolCache
	subscriptions map[string]uint64 // poolID -> subscription ID
	handlers      map[string]PoolUpdateHandler
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewSubscriptionManager creates a new subscription manager
func NewSubscriptionManager(ctx context.Context, wsURL string) (*SubscriptionManager, error) {
	managerCtx, cancel := context.WithCancel(ctx)

	// Create WebSocket client
	wsClient, err := NewWebSocketClient(managerCtx, wsURL)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create WebSocket client: %w", err)
	}

	// Create pool cache
	poolCache := NewPoolCache()

	manager := &SubscriptionManager{
		wsClient:      wsClient,
		poolCache:     poolCache,
		subscriptions: make(map[string]uint64),
		handlers:      make(map[string]PoolUpdateHandler),
		ctx:           managerCtx,
		cancel:        cancel,
	}

	return manager, nil
}

// SubscribePool subscribes to updates for a specific pool
func (sm *SubscriptionManager) SubscribePool(pool pkg.Pool) error {
	poolID := pool.GetID()

	sm.mu.Lock()
	// Check if already subscribed
	if _, exists := sm.subscriptions[poolID]; exists {
		sm.mu.Unlock()
		return nil
	}
	sm.mu.Unlock()

	// Get pool account addresses to subscribe to
	accounts := sm.getPoolAccounts(pool)
	if len(accounts) == 0 {
		return fmt.Errorf("no accounts to subscribe for pool %s", poolID)
	}

	log.Printf("Subscribing to %d accounts for pool %s", len(accounts), poolID)

	// Subscribe to each account
	for _, account := range accounts {
		handler := func(accountID string, data []byte, slot uint64) {
			sm.handleAccountUpdate(poolID, accountID, data, slot)
		}

		subID, err := sm.wsClient.SubscribeAccount(account, handler)
		if err != nil {
			log.Printf("Failed to subscribe to account %s for pool %s: %v", account, poolID, err)
			continue
		}

		sm.mu.Lock()
		sm.subscriptions[account] = subID
		sm.mu.Unlock()

		log.Printf("Subscribed to account %s (subID: %d) for pool %s", account, subID, poolID)
	}

	// Initialize pool in cache
	sm.poolCache.SetPool(poolID, pool)

	return nil
}

// UnsubscribePool unsubscribes from a pool's updates
func (sm *SubscriptionManager) UnsubscribePool(poolID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Find all subscriptions for this pool
	var subsToRemove []string
	for account, subID := range sm.subscriptions {
		// Unsubscribe
		if err := sm.wsClient.Unsubscribe(subID); err != nil {
			log.Printf("Failed to unsubscribe from %s: %v", account, err)
		}
		subsToRemove = append(subsToRemove, account)
	}

	// Remove from tracking
	for _, account := range subsToRemove {
		delete(sm.subscriptions, account)
	}

	// Remove from cache
	sm.poolCache.RemovePool(poolID)

	return nil
}

// handleAccountUpdate processes account updates from WebSocket
func (sm *SubscriptionManager) handleAccountUpdate(poolID, accountID string, base64Data []byte, slot uint64) {
	// Decode base64 data
	data, err := base64.StdEncoding.DecodeString(string(base64Data))
	if err != nil {
		log.Printf("Failed to decode account data for %s: %v", accountID, err)
		return
	}

	// Update pool cache with new data
	if err := sm.poolCache.UpdatePoolAccount(poolID, accountID, data, slot); err != nil {
		log.Printf("Failed to update pool %s account %s: %v", poolID, accountID, err)
		return
	}

	// Call custom handler if registered
	sm.mu.RLock()
	if handler, exists := sm.handlers[poolID]; exists {
		handler(poolID, data, slot)
	}
	sm.mu.RUnlock()
}

// RegisterHandler registers a custom handler for pool updates
func (sm *SubscriptionManager) RegisterHandler(poolID string, handler PoolUpdateHandler) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.handlers[poolID] = handler
}

// GetPool returns a pool from the cache
func (sm *SubscriptionManager) GetPool(poolID string) (pkg.Pool, bool) {
	return sm.poolCache.GetPool(poolID)
}

// GetAllPools returns all cached pools
func (sm *SubscriptionManager) GetAllPools() []pkg.Pool {
	return sm.poolCache.GetAllPools()
}

// IsConnected returns whether the WebSocket is connected
func (sm *SubscriptionManager) IsConnected() bool {
	return sm.wsClient.IsConnected()
}

// Close closes the subscription manager
func (sm *SubscriptionManager) Close() error {
	sm.cancel()

	// Unsubscribe from all pools
	sm.mu.RLock()
	poolIDs := make([]string, 0, len(sm.subscriptions))
	for account := range sm.subscriptions {
		poolIDs = append(poolIDs, account)
	}
	sm.mu.RUnlock()

	for _, poolID := range poolIDs {
		sm.UnsubscribePool(poolID)
	}

	// Close WebSocket client
	return sm.wsClient.Close()
}

// getPoolAccounts extracts account addresses from a pool that need to be monitored
func (sm *SubscriptionManager) getPoolAccounts(pool pkg.Pool) []string {
	accounts := []string{pool.GetID()}

	// Type-specific account extraction
	// Check if pool has vault accounts (e.g., Raydium AMM pools)
	type VaultPool interface {
		GetBaseVault() string
		GetQuoteVault() string
	}

	if vaultPool, ok := pool.(VaultPool); ok {
		baseVault := vaultPool.GetBaseVault()
		quoteVault := vaultPool.GetQuoteVault()
		if baseVault != "" {
			accounts = append(accounts, baseVault)
		}
		if quoteVault != "" {
			accounts = append(accounts, quoteVault)
		}
	}

	return accounts
}

// Stats returns subscription statistics
func (sm *SubscriptionManager) Stats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return map[string]interface{}{
		"subscriptions": len(sm.subscriptions),
		"cachedPools":   sm.poolCache.Size(),
		"connected":     sm.wsClient.IsConnected(),
		"timestamp":     time.Now().Format(time.RFC3339),
	}
}
