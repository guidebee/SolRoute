# examples/quote-service/run_example.ps1
# Build, run, query endpoints, stop service
Set-StrictMode -Version Latest

# Build the binary
go build -o quote-service.exe ./cmd/quote-service

# Start service (background)
$proc = Start-Process -FilePath .\quote-service.exe -ArgumentList '-port','8080','-refresh','30','-slippage','50','-ratelimit','20' -PassThru
Write-Host "quote-service started (PID=$($proc.Id))"

Start-Sleep -Seconds 3

# Query a quote (1 SOL -> USDC)
Invoke-RestMethod -Uri 'http://localhost:8080/quote?input=So11111111111111111111111111111111111111112&output=EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v&amount=1000000000' | ConvertTo-Json -Depth 5

# Query health
Invoke-RestMethod -Uri 'http://localhost:8080/health' | ConvertTo-Json -Depth 5

# Stop the process
Stop-Process -Id $proc.Id -Force
Write-Host "quote-service stopped"

