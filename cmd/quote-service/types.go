package main

import (
	"time"
)

type CachedQuote struct {
	InputMint            string      `json:"inputMint"`
	OutputMint           string      `json:"outputMint"`
	InAmount             string      `json:"inAmount"`
	OutAmount            string      `json:"outAmount"`
	PriceImpact          string      `json:"priceImpact,omitempty"`
	RoutePlan            []RoutePlan `json:"routePlan"`
	SlippageBps          int         `json:"slippageBps"`
	OtherAmountThreshold string      `json:"otherAmountThreshold"`
	LastUpdate           time.Time   `json:"lastUpdate"`
	TimeTaken            string      `json:"timeTaken"`
}

type RoutePlan struct {
	Protocol     string `json:"protocol"`
	PoolID       string `json:"poolId"`
	PoolAddress  string `json:"poolAddress"`
	InputMint    string `json:"inputMint"`
	OutputMint   string `json:"outputMint"`
	InAmount     string `json:"inAmount"`
	OutAmount    string `json:"outAmount"`
	Fee          string `json:"fee,omitempty"`
	ProgramID    string `json:"programId"`
	TokenASymbol string `json:"tokenASymbol,omitempty"`
	TokenBSymbol string `json:"tokenBSymbol,omitempty"`
}

type QuoteRequest struct {
	InputMint   string `json:"inputMint"`
	OutputMint  string `json:"outputMint"`
	Amount      string `json:"amount"`
	SlippageBps int    `json:"slippageBps,omitempty"`
}

type QuoteError struct {
	Error string `json:"error"`
}

type HealthResponse struct {
	Status       string    `json:"status"`
	LastUpdate   time.Time `json:"lastUpdate"`
	CachedRoutes int       `json:"cachedRoutes"`
	Uptime       string    `json:"uptime"`
}
