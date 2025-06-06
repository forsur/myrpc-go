# PowerShell script to test RPC services
Write-Host "Starting RPC services test..." -ForegroundColor Green

# Function to kill processes on specific ports
function Kill-ProcessOnPort {
    param([int]$Port)
    try {
        $process = Get-NetTCPConnection -LocalPort $Port -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess
        if ($process) {
            Stop-Process -Id $process -Force -ErrorAction SilentlyContinue
            Write-Host "Killed process on port $Port" -ForegroundColor Yellow
        }
    } catch {
        # Ignore errors
    }
}

# Clean up any existing processes
Write-Host "Cleaning up existing processes..." -ForegroundColor Yellow
Kill-ProcessOnPort 8088

# Wait a moment
Start-Sleep -Seconds 2

# Start registry
Write-Host "Starting registry..." -ForegroundColor Blue
$registryJob = Start-Job -ScriptBlock {
    Set-Location "d:\myrpc-go"
    go run registry_app/registry_app.go
}

# Wait for registry to start
Start-Sleep -Seconds 3

# Start server
Write-Host "Starting server..." -ForegroundColor Blue
$serverJob = Start-Job -ScriptBlock {
    Set-Location "d:\myrpc-go"
    go run server_app/server_app.go
}

# Wait for server to start and register
Start-Sleep -Seconds 3

# Run client
Write-Host "Running client..." -ForegroundColor Blue
Set-Location "d:\myrpc-go"
go run client_app/client_app.go

# Clean up
Write-Host "Cleaning up..." -ForegroundColor Yellow
Stop-Job $registryJob, $serverJob -ErrorAction SilentlyContinue
Remove-Job $registryJob, $serverJob -ErrorAction SilentlyContinue

Write-Host "Test completed!" -ForegroundColor Green
