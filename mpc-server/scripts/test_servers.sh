#!/bin/bash

# 测试MPC服务器的脚本

echo "=== MPC服务器测试脚本 ==="

# 检查服务器是否运行
check_server() {
    local port=$1
    local name=$2
    
    echo "检查 $name 服务器 (端口 $port)..."
    
    # 测试健康检查
    response=$(curl -s -w "%{http_code}" http://localhost:$port/health)
    http_code="${response: -3}"
    
    if [ "$http_code" = "200" ]; then
        echo "✅ $name 服务器运行正常"
        echo "响应: ${response%???}"
        echo ""
    else
        echo "❌ $name 服务器无法访问 (HTTP $http_code)"
        echo ""
        return 1
    fi
}

# 测试API端点
test_api() {
    local port=$1
    local name=$2
    
    echo "测试 $name 服务器API..."
    
    # 测试服务器信息
    echo "获取服务器信息:"
    curl -s http://localhost:$port/api/v1/info | jq . 2>/dev/null || curl -s http://localhost:$port/api/v1/info
    echo ""
    
    # 测试会话列表
    echo "获取会话列表:"
    curl -s http://localhost:$port/api/v1/sessions | jq . 2>/dev/null || curl -s http://localhost:$port/api/v1/sessions
    echo ""
    echo "---"
}

# 启动密钥生成测试
test_keygen() {
    echo "=== 测试密钥生成 ==="
    
    # 向enterprise服务器发起密钥生成请求
    echo "向enterprise服务器发起密钥生成请求..."
    response=$(curl -s -X POST http://localhost:8082/api/v1/keygen \
        -H "Content-Type: application/json" \
        -d '{
            "threshold": 2,
            "participants": ["third-party", "enterprise", "mobile-app"]
        }')
    
    echo "响应: $response"
    echo ""
}

# 主测试流程
main() {
    echo "开始测试所有服务器..."
    echo ""
    
    # 检查所有服务器
    check_server 8081 "third-party" || exit 1
    check_server 8082 "enterprise" || exit 1  
    check_server 8083 "mobile-app" || exit 1
    
    echo "=== 所有服务器运行正常 ==="
    echo ""
    
    # 测试API
    test_api 8081 "third-party"
    test_api 8082 "enterprise"
    test_api 8083 "mobile-app"
    
    # 测试密钥生成
    test_keygen
    
    echo "=== 测试完成 ==="
}

# 运行主函数
main