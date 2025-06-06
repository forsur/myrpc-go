# PowerShell script to test performance with 3 servers
Write-Host "启动3个Server进程的性能测试..." -ForegroundColor Green

# Function to kill processes on specific ports
function Kill-ProcessOnPort {
    param([int]$Port)
    try {
        $process = Get-NetTCPConnection -LocalPort $Port -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess
        if ($process) {
            Stop-Process -Id $process -Force -ErrorAction SilentlyContinue
            Write-Host "已结束端口 $Port 上的进程" -ForegroundColor Yellow
        }
    } catch {
        # Ignore errors
    }
}

# Clean up any existing processes
Write-Host "清理现有进程..." -ForegroundColor Yellow
Kill-ProcessOnPort 8088

# Wait a moment
Start-Sleep -Seconds 2

# Start registry
Write-Host "启动注册中心..." -ForegroundColor Blue
$registryJob = Start-Job -ScriptBlock {
    Set-Location "d:\myrpc-go"
    go run registry_app/registry_app.go
}

# Wait for registry to start
Start-Sleep -Seconds 3

# Start 3 benchmark servers
Write-Host "启动第1个Server..." -ForegroundColor Blue
$server1Job = Start-Job -ScriptBlock {
    Set-Location "d:\myrpc-go"
    go run bench_server/server.go
}

Write-Host "启动第2个Server..." -ForegroundColor Blue
$server2Job = Start-Job -ScriptBlock {
    Set-Location "d:\myrpc-go"
    go run bench_server/server.go
}

Write-Host "启动第3个Server..." -ForegroundColor Blue
$server3Job = Start-Job -ScriptBlock {
    Set-Location "d:\myrpc-go"
    go run bench_server/server.go
}

# Wait for servers to start and register
Write-Host "等待所有服务器启动并注册..." -ForegroundColor Yellow
Start-Sleep -Seconds 5

# Run benchmark client
Write-Host "开始性能测试..." -ForegroundColor Green
Set-Location "d:\myrpc-go"
go run bench_client/client.go -c 100 -d 1s

# Clean up
Write-Host "清理资源..." -ForegroundColor Yellow
Stop-Job $registryJob, $server1Job, $server2Job, $server3Job -ErrorAction SilentlyContinue
Remove-Job $registryJob, $server1Job, $server2Job, $server3Job -ErrorAction SilentlyContinue

Write-Host "3个Server性能测试完成!" -ForegroundColor Green
