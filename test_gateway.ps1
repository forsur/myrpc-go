# 网关测试脚本
# 使用方法: .\test_gateway.ps1

Write-Host "Starting Gateway Test..." -ForegroundColor Green

# 测试健康检查
Write-Host "`nTesting health check..." -ForegroundColor Yellow
try {
    $healthResponse = Invoke-RestMethod -Uri "http://localhost:8080/health" -Method GET
    Write-Host "Health check response:" -ForegroundColor Green
    $healthResponse | ConvertTo-Json -Depth 3
} catch {
    Write-Host "Health check failed: $($_.Exception.Message)" -ForegroundColor Red
}

# 测试 RPC 调用
Write-Host "`nTesting RPC call..." -ForegroundColor Yellow

# 准备测试数据
$testArgs = @{
    username = "testuser"
    password = "testpass"
}

$body = $testArgs | ConvertTo-Json

try {
    # 调用 AuthService.Login 方法
    $rpcResponse = Invoke-RestMethod -Uri "http://localhost:8080/rpc/AuthService.Login" `
                                   -Method POST `
                                   -Body $body `
                                   -ContentType "application/json"
    
    Write-Host "RPC call response:" -ForegroundColor Green
    $rpcResponse | ConvertTo-Json -Depth 3
} catch {
    Write-Host "RPC call failed: $($_.Exception.Message)" -ForegroundColor Red
    if ($_.Exception.Response) {
        $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $responseBody = $reader.ReadToEnd()
        Write-Host "Response body: $responseBody" -ForegroundColor Red
    }
}

# 测试无效的服务方法
Write-Host "`nTesting invalid service method..." -ForegroundColor Yellow
try {
    $invalidResponse = Invoke-RestMethod -Uri "http://localhost:8080/rpc/InvalidService" `
                                        -Method POST `
                                        -Body "{}" `
                                        -ContentType "application/json"
    Write-Host "Invalid method response:" -ForegroundColor Green
    $invalidResponse | ConvertTo-Json -Depth 3
} catch {
    Write-Host "Invalid method call failed as expected: $($_.Exception.Message)" -ForegroundColor Yellow
    if ($_.Exception.Response) {
        $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $responseBody = $reader.ReadToEnd()
        Write-Host "Response body: $responseBody" -ForegroundColor Yellow
    }
}

Write-Host "`nGateway test completed!" -ForegroundColor Green
