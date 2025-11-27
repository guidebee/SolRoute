package sol

import (
	"context"
	"sync"
	"sync/atomic"
)

// RPCPool manages multiple RPC endpoints and distributes requests across them
type RPCPool struct {
	endpoints []string
	clients   []*Client
	index     uint64
	mu        sync.RWMutex
}

// NewRPCPool creates a new RPC pool with the given endpoints
func NewRPCPool(ctx context.Context, endpoints []string, jitoRpc string, reqLimitPerSecond int) (*RPCPool, error) {
	if len(endpoints) == 0 {
		return nil, nil
	}

	pool := &RPCPool{
		endpoints: endpoints,
		clients:   make([]*Client, 0, len(endpoints)),
	}

	// Create a client for each endpoint
	for _, endpoint := range endpoints {
		client, err := NewClient(ctx, endpoint, jitoRpc, reqLimitPerSecond)
		if err != nil {
			return nil, err
		}
		pool.clients = append(pool.clients, client)
	}

	return pool, nil
}

// GetClient returns the next client in round-robin fashion
func (p *RPCPool) GetClient() *Client {
	if len(p.clients) == 0 {
		return nil
	}
	if len(p.clients) == 1 {
		return p.clients[0]
	}

	// Atomic round-robin selection
	idx := atomic.AddUint64(&p.index, 1) % uint64(len(p.clients))
	return p.clients[idx]
}

// GetAllClients returns all clients in the pool
func (p *RPCPool) GetAllClients() []*Client {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.clients
}

// Size returns the number of clients in the pool
func (p *RPCPool) Size() int {
	return len(p.clients)
}
