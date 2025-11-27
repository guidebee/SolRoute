@echo off
REM Script to build and run the SolRoute Quote Service

echo ========================================
echo SolRoute Quote Service Builder
echo ========================================
echo.

REM Check if .env file exists
if not exist ".env" (
    echo WARNING: .env file not found!
    echo Please copy .env.example to .env and configure your RPC endpoints.
    echo.
    echo Example:
    echo   copy .env.example .env
    echo   notepad .env
    echo.
    pause
    exit /b 1
)

echo Building quote-service...
go build -o quote-service.exe ./cmd/quote-service
if errorlevel 1 (
    echo.
    echo ERROR: Build failed!
    pause
    exit /b 1
)

echo.
echo Build successful!
echo.

REM Parse command line arguments or use defaults
set PORT=8080
set REFRESH=30
set SLIPPAGE=50
set RATELIMIT=20

if not "%1"=="" set PORT=%1
if not "%2"=="" set REFRESH=%2
if not "%3"=="" set SLIPPAGE=%3
if not "%4"=="" set RATELIMIT=%4

echo Starting quote-service with configuration:
echo   Port: %PORT%
echo   Refresh interval: %REFRESH%s
echo   Slippage: %SLIPPAGE% bps
echo   Rate limit: %RATELIMIT% req/s
echo.
echo Press Ctrl+C to stop the service
echo ========================================
echo.

REM Run the service
quote-service.exe -port %PORT% -refresh %REFRESH% -slippage %SLIPPAGE% -ratelimit %RATELIMIT%
