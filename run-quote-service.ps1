#!/usr/bin/env pwsh
# Script to build and run the SolRoute Quote Service

param(
    [int]$Port = 8080,
    [int]$Refresh = 30,
    [int]$Slippage = 50,
    [int]$RateLimit = 20
)

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "SolRoute Quote Service Builder" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Check if .env file exists
if (-not (Test-Path ".env")) {
    Write-Host "WARNING: .env file not found!" -ForegroundColor Yellow
    Write-Host "Please copy .env.example to .env and configure your RPC endpoints." -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Example:" -ForegroundColor White
    Write-Host "  Copy-Item .env.example .env" -ForegroundColor Gray
    Write-Host "  notepad .env" -ForegroundColor Gray
    Write-Host ""
    Read-Host "Press Enter to exit"
    exit 1
}

Write-Host "Building quote-service..." -ForegroundColor Green
$buildResult = go build -o quote-service.exe ./cmd/quote-service
if ($LASTEXITCODE -ne 0) {
    Write-Host ""
    Write-Host "ERROR: Build failed!" -ForegroundColor Red
    Read-Host "Press Enter to exit"
    exit 1
}

Write-Host ""
Write-Host "Build successful!" -ForegroundColor Green
Write-Host ""

Write-Host "Starting quote-service with configuration:" -ForegroundColor Cyan
Write-Host "  Port: $Port" -ForegroundColor White
Write-Host "  Refresh interval: $Refresh`s" -ForegroundColor White
Write-Host "  Slippage: $Slippage bps" -ForegroundColor White
Write-Host "  Rate limit: $RateLimit req/s" -ForegroundColor White
Write-Host ""
Write-Host "Press Ctrl+C to stop the service" -ForegroundColor Yellow
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Run the service
& ./quote-service.exe -port $Port -refresh $Refresh -slippage $Slippage -ratelimit $RateLimit
