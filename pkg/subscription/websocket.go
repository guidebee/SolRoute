package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketClient manages WebSocket connection to Solana
type WebSocketClient struct {
	url            string
	conn           *websocket.Conn
	mu             sync.RWMutex
	subscriptions  map[uint64]*Subscription
	nextID         uint64
	handlers       map[uint64]AccountUpdateHandler
	reconnectDelay time.Duration
	ctx            context.Context
	cancel         context.CancelFunc
	connected      bool
}

// Subscription represents an account subscription
type Subscription struct {
	ID        uint64
	AccountID string
	SubID     uint64 // Solana subscription ID
}

// AccountUpdateHandler is called when an account is updated
type AccountUpdateHandler func(accountID string, data []byte, slot uint64)

// RPCRequest represents a JSON-RPC request
type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      uint64        `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

// RPCResponse represents a JSON-RPC response
type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NotificationMessage represents a subscription notification
type NotificationMessage struct {
	JSONRPC string             `json:"jsonrpc"`
	Method  string             `json:"method"`
	Params  NotificationParams `json:"params"`
}

// NotificationParams contains subscription notification data
type NotificationParams struct {
	Result       AccountNotification `json:"result"`
	Subscription uint64              `json:"subscription"`
}

// AccountNotification contains account update data
type AccountNotification struct {
	Context Context      `json:"context"`
	Value   AccountValue `json:"value"`
}

// Context contains slot information
type Context struct {
	Slot uint64 `json:"slot"`
}

// AccountValue contains account data
type AccountValue struct {
	Data       []interface{} `json:"data"` // [base64_data, encoding]
	Executable bool          `json:"executable"`
	Lamports   uint64        `json:"lamports"`
	Owner      string        `json:"owner"`
	RentEpoch  uint64        `json:"rentEpoch"`
}

// NewWebSocketClient creates a new WebSocket client
func NewWebSocketClient(ctx context.Context, wsURL string) (*WebSocketClient, error) {
	clientCtx, cancel := context.WithCancel(ctx)

	client := &WebSocketClient{
		url:            wsURL,
		subscriptions:  make(map[uint64]*Subscription),
		handlers:       make(map[uint64]AccountUpdateHandler),
		reconnectDelay: 5 * time.Second,
		ctx:            clientCtx,
		cancel:         cancel,
		nextID:         1,
	}

	if err := client.connect(); err != nil {
		cancel()
		return nil, err
	}

	// Start message reader
	go client.readMessages()

	// Start reconnection handler
	go client.handleReconnection()

	return client, nil
}

// connect establishes WebSocket connection
func (c *WebSocketClient) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, _, err := websocket.DefaultDialer.Dial(c.url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	c.conn = conn
	c.connected = true
	log.Printf("WebSocket connected to %s", c.url)

	return nil
}

// SubscribeAccount subscribes to account updates
func (c *WebSocketClient) SubscribeAccount(accountID string, handler AccountUpdateHandler) (uint64, error) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	c.mu.Unlock()

	// Send subscription request
	req := RPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "accountSubscribe",
		Params: []interface{}{
			accountID,
			map[string]interface{}{
				"encoding":   "base64",
				"commitment": "confirmed",
			},
		},
	}

	if err := c.sendRequest(req); err != nil {
		return 0, err
	}

	// Store handler
	c.mu.Lock()
	c.handlers[id] = handler
	c.subscriptions[id] = &Subscription{
		ID:        id,
		AccountID: accountID,
	}
	c.mu.Unlock()

	return id, nil
}

// Unsubscribe removes an account subscription
func (c *WebSocketClient) Unsubscribe(subID uint64) error {
	c.mu.Lock()
	sub, exists := c.subscriptions[subID]
	if !exists {
		c.mu.Unlock()
		return fmt.Errorf("subscription not found: %d", subID)
	}

	if sub.SubID == 0 {
		// Subscription not yet confirmed
		delete(c.subscriptions, subID)
		delete(c.handlers, subID)
		c.mu.Unlock()
		return nil
	}

	solanaSubID := sub.SubID
	c.mu.Unlock()

	// Send unsubscribe request
	req := RPCRequest{
		JSONRPC: "2.0",
		ID:      subID,
		Method:  "accountUnsubscribe",
		Params:  []interface{}{solanaSubID},
	}

	if err := c.sendRequest(req); err != nil {
		return err
	}

	c.mu.Lock()
	delete(c.subscriptions, subID)
	delete(c.handlers, subID)
	c.mu.Unlock()

	return nil
}

// sendRequest sends a JSON-RPC request
func (c *WebSocketClient) sendRequest(req RPCRequest) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	return conn.WriteMessage(websocket.TextMessage, data)
}

// readMessages reads incoming messages
func (c *WebSocketClient) readMessages() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		if conn == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			c.mu.Lock()
			c.connected = false
			c.mu.Unlock()
			continue
		}

		c.handleMessage(message)
	}
}

// handleMessage processes incoming messages
func (c *WebSocketClient) handleMessage(data []byte) {
	// Try to parse as notification first
	var notification NotificationMessage
	if err := json.Unmarshal(data, &notification); err == nil && notification.Method == "accountNotification" {
		c.handleAccountNotification(notification)
		return
	}

	// Parse as response
	var response RPCResponse
	if err := json.Unmarshal(data, &response); err != nil {
		log.Printf("Failed to parse WebSocket message: %v", err)
		return
	}

	c.handleResponse(response)
}

// handleResponse processes RPC responses
func (c *WebSocketClient) handleResponse(response RPCResponse) {
	if response.Error != nil {
		log.Printf("RPC error: %s", response.Error.Message)
		return
	}

	// Parse subscription ID from result
	var subID uint64
	if err := json.Unmarshal(response.Result, &subID); err != nil {
		return
	}

	// Update subscription with Solana subscription ID
	c.mu.Lock()
	if sub, exists := c.subscriptions[response.ID]; exists {
		sub.SubID = subID
	}
	c.mu.Unlock()
}

// handleAccountNotification processes account notifications
func (c *WebSocketClient) handleAccountNotification(notification NotificationMessage) {
	// Find handler by Solana subscription ID
	c.mu.RLock()
	var handler AccountUpdateHandler
	var accountID string

	for _, sub := range c.subscriptions {
		if sub.SubID == notification.Params.Subscription {
			if h, exists := c.handlers[sub.ID]; exists {
				handler = h
				accountID = sub.AccountID
			}
			break
		}
	}
	c.mu.RUnlock()

	if handler == nil {
		return
	}

	// Decode account data
	if len(notification.Params.Result.Value.Data) < 1 {
		return
	}

	dataStr, ok := notification.Params.Result.Value.Data[0].(string)
	if !ok {
		return
	}

	// Call handler with decoded data
	handler(accountID, []byte(dataStr), notification.Params.Result.Context.Slot)
}

// handleReconnection manages reconnection logic
func (c *WebSocketClient) handleReconnection() {
	ticker := time.NewTicker(c.reconnectDelay)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.mu.RLock()
			connected := c.connected
			c.mu.RUnlock()

			if !connected {
				log.Printf("Attempting to reconnect WebSocket...")
				if err := c.reconnect(); err != nil {
					log.Printf("Reconnection failed: %v", err)
				} else {
					log.Printf("WebSocket reconnected successfully")
				}
			}
		}
	}
}

// reconnect attempts to reconnect and resubscribe
func (c *WebSocketClient) reconnect() error {
	if err := c.connect(); err != nil {
		return err
	}

	// Resubscribe to all accounts
	c.mu.Lock()
	subs := make([]*Subscription, 0, len(c.subscriptions))
	handlers := make(map[uint64]AccountUpdateHandler)

	for id, sub := range c.subscriptions {
		subs = append(subs, sub)
		handlers[id] = c.handlers[id]
	}
	c.mu.Unlock()

	for _, sub := range subs {
		req := RPCRequest{
			JSONRPC: "2.0",
			ID:      sub.ID,
			Method:  "accountSubscribe",
			Params: []interface{}{
				sub.AccountID,
				map[string]interface{}{
					"encoding":   "base64",
					"commitment": "confirmed",
				},
			},
		}

		if err := c.sendRequest(req); err != nil {
			log.Printf("Failed to resubscribe to %s: %v", sub.AccountID, err)
		}
	}

	return nil
}

// Close closes the WebSocket connection
func (c *WebSocketClient) Close() error {
	c.cancel()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// IsConnected returns whether the client is connected
func (c *WebSocketClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}
