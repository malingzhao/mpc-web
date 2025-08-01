#!/bin/bash

# 停止所有MPC服务器的脚本

echo "=== 停止MPC服务器集群 ==="

# 停止服务器函数
stop_server() {
    local name=$1
    local pid_file="logs/$name.pid"
    
    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        echo "停止 $name 服务器 (PID: $pid)..."
        
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid"
            sleep 2
            
            # 如果进程还在运行，强制杀死
            if kill -0 "$pid" 2>/dev/null; then
                echo "强制停止 $name 服务器..."
                kill -9 "$pid"
            fi
            
            echo "✅ $name 服务器已停止"
        else
            echo "⚠️  $name 服务器进程不存在"
        fi
        
        rm -f "$pid_file"
    else
        echo "⚠️  未找到 $name 服务器的PID文件"
    fi
}

# 停止所有服务器
stop_server "third-party"
stop_server "enterprise" 
stop_server "mobile-app"

echo ""
echo "所有服务器已停止！"