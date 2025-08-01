#!/bin/bash

# MPC功能调试脚本

echo "=== MPC功能调试脚本 ==="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_status() {
    local status=$1
    local message=$2
    case $status in
        "SUCCESS") echo -e "${GREEN}✅ $message${NC}" ;;
        "ERROR") echo -e "${RED}❌ $message${NC}" ;;
        "WARNING") echo -e "${YELLOW}⚠️  $message${NC}" ;;
        "INFO") echo -e "${BLUE}ℹ️  $message${NC}" ;;
    esac
}

# 检查服务器响应
check_response() {
    local response=$1
    local expected_field=$2
    
    if echo "$response" | jq -e ".$expected_field" > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# 等待会话状态变化
wait_for_session_status() {
    local server_port=$1
    local session_id=$2
    local expected_status=$3
    local max_wait=${4:-30}
    
    print_status "INFO" "等待会话 $session_id 状态变为 $expected_status..."
    
    for i in $(seq 1 $max_wait); do
        local response=$(curl -s "http://localhost:$server_port/api/v1/sessions/$session_id")
        local current_status=$(echo "$response" | jq -r '.status // "unknown"')
        
        echo "第 $i 次检查: 当前状态 = $current_status"
        
        if [ "$current_status" = "$expected_status" ]; then
            print_status "SUCCESS" "会话状态已变为 $expected_status"
            return 0
        fi
        
        sleep 1
    done
    
    print_status "ERROR" "等待超时，会话状态未变为 $expected_status"
    return 1
}

# 测试密钥生成
test_keygen() {
    echo ""
    echo "=== 测试密钥生成 (Keygen) ==="
    
    # 清理之前的会话
    print_status "INFO" "清理之前的会话..."
    
    # 发起密钥生成请求
    print_status "INFO" "向enterprise服务器发起密钥生成请求..."
    local keygen_response=$(curl -s -X POST http://localhost:8082/api/v1/keygen \
        -H "Content-Type: application/json" \
        -d '{
            "threshold": 2,
            "participants": ["third-party", "enterprise", "mobile-app"]
        }')
    
    echo "Keygen响应: $keygen_response"
    
    if check_response "$keygen_response" "session_id"; then
        local session_id=$(echo "$keygen_response" | jq -r '.session_id')
        print_status "SUCCESS" "密钥生成会话创建成功，会话ID: $session_id"
        
        # 检查所有服务器是否都有这个会话
        echo ""
        print_status "INFO" "检查会话同步状态..."
        
        for port in 8081 8082 8083; do
            local server_name=""
            case $port in
                8081) server_name="third-party" ;;
                8082) server_name="enterprise" ;;
                8083) server_name="mobile-app" ;;
            esac
            
            local session_response=$(curl -s "http://localhost:$port/api/v1/sessions/$session_id")
            if check_response "$session_response" "id"; then
                local status=$(echo "$session_response" | jq -r '.status')
                local round=$(echo "$session_response" | jq -r '.current_round')
                print_status "SUCCESS" "$server_name 服务器有会话记录，状态: $status, 轮次: $round"
            else
                print_status "ERROR" "$server_name 服务器没有会话记录"
            fi
        done
        
        # 等待密钥生成完成
        wait_for_session_status 8082 "$session_id" "completed" 60
        
        return 0
    else
        print_status "ERROR" "密钥生成请求失败"
        return 1
    fi
}

# 测试重分享
test_reshare() {
    echo ""
    echo "=== 测试密钥重分享 (Reshare) ==="
    
    # 首先需要一个完成的密钥生成会话
    print_status "INFO" "检查是否有可用的密钥..."
    
    local sessions_response=$(curl -s "http://localhost:8082/api/v1/sessions")
    local completed_keygen=$(echo "$sessions_response" | jq -r '.sessions[] | select(.type == "keygen" and .status == "completed") | .id' | head -1)
    
    if [ -z "$completed_keygen" ] || [ "$completed_keygen" = "null" ]; then
        print_status "WARNING" "没有找到完成的密钥生成会话，先执行密钥生成..."
        if ! test_keygen; then
            print_status "ERROR" "无法完成密钥生成，跳过重分享测试"
            return 1
        fi
        # 重新获取完成的会话
        sessions_response=$(curl -s "http://localhost:8082/api/v1/sessions")
        completed_keygen=$(echo "$sessions_response" | jq -r '.sessions[] | select(.type == "keygen" and .status == "completed") | .id' | head -1)
    fi
    
    if [ -n "$completed_keygen" ] && [ "$completed_keygen" != "null" ]; then
        print_status "INFO" "使用密钥会话: $completed_keygen"
        
        # 发起重分享请求
        print_status "INFO" "发起密钥重分享请求..."
        local reshare_response=$(curl -s -X POST http://localhost:8082/api/v1/reshare \
            -H "Content-Type: application/json" \
            -d "{
                \"session_id\": \"$completed_keygen\",
                \"new_threshold\": 2,
                \"new_participants\": [\"third-party\", \"enterprise\", \"mobile-app\"]
            }")
        
        echo "Reshare响应: $reshare_response"
        
        if check_response "$reshare_response" "session_id"; then
            local session_id=$(echo "$reshare_response" | jq -r '.session_id')
            print_status "SUCCESS" "重分享会话创建成功，会话ID: $session_id"
            
            # 等待重分享完成
            wait_for_session_status 8082 "$session_id" "completed" 60
            
            return 0
        else
            print_status "ERROR" "重分享请求失败"
            return 1
        fi
    else
        print_status "ERROR" "没有可用的密钥进行重分享"
        return 1
    fi
}

# 测试签名
test_sign() {
    echo ""
    echo "=== 测试签名 (Sign) ==="
    
    # 检查是否有可用的密钥
    print_status "INFO" "检查是否有可用的密钥..."
    
    local sessions_response=$(curl -s "http://localhost:8082/api/v1/sessions")
    local completed_keygen=$(echo "$sessions_response" | jq -r '.sessions[] | select(.type == "keygen" and .status == "completed") | .id' | head -1)
    
    if [ -z "$completed_keygen" ] || [ "$completed_keygen" = "null" ]; then
        print_status "WARNING" "没有找到完成的密钥生成会话，先执行密钥生成..."
        if ! test_keygen; then
            print_status "ERROR" "无法完成密钥生成，跳过签名测试"
            return 1
        fi
        # 重新获取完成的会话
        sessions_response=$(curl -s "http://localhost:8082/api/v1/sessions")
        completed_keygen=$(echo "$sessions_response" | jq -r '.sessions[] | select(.type == "keygen" and .status == "completed") | .id' | head -1)
    fi
    
    if [ -n "$completed_keygen" ] && [ "$completed_keygen" != "null" ]; then
        print_status "INFO" "使用密钥会话: $completed_keygen"
        
        # 发起签名请求
        print_status "INFO" "发起签名请求..."
        local sign_response=$(curl -s -X POST http://localhost:8082/api/v1/sign \
            -H "Content-Type: application/json" \
            -d "{
                \"session_id\": \"$completed_keygen\",
                \"message\": \"Hello, MPC World!\",
                \"signers\": [\"third-party\", \"enterprise\"]
            }")
        
        echo "Sign响应: $sign_response"
        
        if check_response "$sign_response" "session_id"; then
            local session_id=$(echo "$sign_response" | jq -r '.session_id')
            print_status "SUCCESS" "签名会话创建成功，会话ID: $session_id"
            
            # 等待签名完成
            wait_for_session_status 8082 "$session_id" "completed" 60
            
            return 0
        else
            print_status "ERROR" "签名请求失败"
            return 1
        fi
    else
        print_status "ERROR" "没有可用的密钥进行签名"
        return 1
    fi
}

# 检查服务器日志
check_server_logs() {
    echo ""
    echo "=== 检查服务器日志 ==="
    
    for server in third-party enterprise mobile-app; do
        local log_file="logs/$server.log"
        if [ -f "$log_file" ]; then
            print_status "INFO" "检查 $server 服务器日志..."
            echo "最近的错误日志:"
            tail -20 "$log_file" | grep -i "error\|panic\|fatal" || echo "没有发现错误"
            echo ""
        else
            print_status "WARNING" "未找到 $server 服务器日志文件"
        fi
    done
}

# 主函数
main() {
    print_status "INFO" "开始MPC功能调试..."
    
    # 检查服务器状态
    print_status "INFO" "检查服务器状态..."
    if ! ./scripts/test_servers.sh > /dev/null 2>&1; then
        print_status "ERROR" "服务器状态检查失败，请先启动服务器"
        exit 1
    fi
    
    # 测试各个功能
    local keygen_result=0
    local reshare_result=0
    local sign_result=0
    
    test_keygen
    keygen_result=$?
    
    test_reshare
    reshare_result=$?
    
    test_sign
    sign_result=$?
    
    # 检查日志
    check_server_logs
    
    # 总结
    echo ""
    echo "=== 调试结果总结 ==="
    
    if [ $keygen_result -eq 0 ]; then
        print_status "SUCCESS" "密钥生成 (Keygen) 测试通过"
    else
        print_status "ERROR" "密钥生成 (Keygen) 测试失败"
    fi
    
    if [ $reshare_result -eq 0 ]; then
        print_status "SUCCESS" "密钥重分享 (Reshare) 测试通过"
    else
        print_status "ERROR" "密钥重分享 (Reshare) 测试失败"
    fi
    
    if [ $sign_result -eq 0 ]; then
        print_status "SUCCESS" "签名 (Sign) 测试通过"
    else
        print_status "ERROR" "签名 (Sign) 测试失败"
    fi
    
    local total_failed=$((keygen_result + reshare_result + sign_result))
    if [ $total_failed -eq 0 ]; then
        print_status "SUCCESS" "所有MPC功能测试通过！"
    else
        print_status "ERROR" "有 $total_failed 个功能测试失败"
    fi
}

# 运行主函数
main