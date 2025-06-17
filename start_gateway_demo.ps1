# Gateway 演示启动脚本
# 依次启动：注册中心 -> 服务器 -> 网关 -> 测试

Write-Host "Starting RPC Gateway Demo..." -ForegroundColor Green

# 设置变量
$RegistryPort = "8088"
$GatewayPort = "8080"
$RegistryUrl = "http://127.0.0.1:$RegistryPort/myrpc/registry"

# 检查端口是否被占用
function Test-Port {
    param($Port)
    try {
        $connection = New-Object System.Net.Sockets.TcpClient("127.0.0.1", $Port)
        $connection.Close()
        return $true
    } catch {
        return $false
    }
}

if (Test-Port $RegistryPort) {
    Write-Host "Port $RegistryPort is already in use. Please stop the service or use another port." -ForegroundColor Red
    exit 1
}

if (Test-Port $GatewayPort) {
    Write-Host "Port $GatewayPort is already in use. Please stop the service or use another port." -ForegroundColor Red
    exit 1
}

Write-Host "`nStep 1: Starting Registry..." -ForegroundColor Yellow
$registryJob = Start-Job -ScriptBlock {
    Set-Location "d:\_MyProject\myrpc-go"
    go run registry_app/registry_app.go
}

# 等待注册中心启动
Start-Sleep -Seconds 3
Write-Host "Registry should be running on http://127.0.0.1:$RegistryPort/myrpc/registry" -ForegroundColor Green

Write-Host "`nStep 2: Starting Server..." -ForegroundColor Yellow
$serverJob = Start-Job -ScriptBlock {
    Set-Location "d:\_MyProject\myrpc-go"
    go run server_app/server_app.go
}

# 等待服务器启动并注册
Start-Sleep -Seconds 3
Write-Host "Server should be running and registered" -ForegroundColor Green

Write-Host "`nStep 3: Starting Gateway..." -ForegroundColor Yellow
$gatewayJob = Start-Job -ScriptBlock {
    param($RegistryUrl, $GatewayPort)
    Set-Location "d:\_MyProject\myrpc-go"
    go run gateway_app/gateway_app.go $RegistryUrl $GatewayPort "random"
} -ArgumentList $RegistryUrl, $GatewayPort

# 等待网关启动
Start-Sleep -Seconds 3
Write-Host "Gateway should be running on http://localhost:$GatewayPort" -ForegroundColor Green

Write-Host "`nStep 4: Testing Gateway..." -ForegroundColor Yellow

# 测试健康检查
Write-Host "`nTesting health check..." -ForegroundColor Cyan
try {
    $healthResponse = Invoke-RestMethod -Uri "http://localhost:$GatewayPort/health" -Method GET -TimeoutSec 10
    Write-Host "✓ Health check successful!" -ForegroundColor Green
    Write-Host ($healthResponse | ConvertTo-Json -Depth 3) -ForegroundColor Gray
} catch {
    Write-Host "✗ Health check failed: $($_.Exception.Message)" -ForegroundColor Red
}

# 测试 RPC 调用
Write-Host "`nTesting RPC call..." -ForegroundColor Cyan
$testArgs = @{
    Num1 = 15
    Num2 = 25
} | ConvertTo-Json

try {
    $rpcResponse = Invoke-RestMethod -Uri "http://localhost:$GatewayPort/rpc/AddServiceImpl.Sum" `
                                   -Method POST `
                                   -Body $testArgs `
                                   -ContentType "application/json" `
                                   -TimeoutSec 10
    Write-Host "✓ RPC call successful!" -ForegroundColor Green
    Write-Host ($rpcResponse | ConvertTo-Json -Depth 3) -ForegroundColor Gray
} catch {
    Write-Host "✗ RPC call failed: $($_.Exception.Message)" -ForegroundColor Red
    if ($_.Exception.Response) {
        $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $responseBody = $reader.ReadToEnd()
        Write-Host "Response: $responseBody" -ForegroundColor Red
    }
}

Write-Host "`n" + "="*60 -ForegroundColor Blue
Write-Host "Demo is running! You can:" -ForegroundColor Green
Write-Host "1. Open your browser to http://localhost:$GatewayPort/health" -ForegroundColor White
Write-Host "2. Open d:\_MyProject\myrpc-go\gateway\test.html in your browser" -ForegroundColor White
Write-Host "3. Use the test script: .\test_gateway.ps1" -ForegroundColor White
Write-Host "4. Press Ctrl+C to stop all services" -ForegroundColor White
Write-Host "="*60 -ForegroundColor Blue

try {
    Write-Host "`nPress Ctrl+C to stop all services..." -ForegroundColor Yellow
    while ($true) {
        Start-Sleep -Seconds 1
        
        # 检查作业状态
        $jobs = @($registryJob, $serverJob, $gatewayJob) | Where-Object { $_.State -eq "Running" }
        if ($jobs.Count -eq 0) {
            Write-Host "All services have stopped unexpectedly." -ForegroundColor Red
            break
        }
    }
} finally {
    Write-Host "`nStopping all services..." -ForegroundColor Yellow
    
    # 停止所有作业
    @($registryJob, $serverJob, $gatewayJob) | ForEach-Object {
        if ($_.State -eq "Running") {
            Stop-Job $_
        }
        Remove-Job $_ -Force
    }
    
    Write-Host "All services stopped." -ForegroundColor Green
}
