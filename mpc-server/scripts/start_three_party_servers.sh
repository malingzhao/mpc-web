#!/bin/bash

# 三方MPC服务器启动脚本
# 启动Enterprise、Mobile和Third-party三个服务器实例

echo "启动三方MPC服务器演示..."

# 检查是否已有服务器在运行
check_port() {
    local port=$1
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null ; then
        echo "端口 $port 已被占用"
        return 1
    fi
    return 0
}

# 停止现有服务器
stop_servers() {
    echo "停止现有服务器..."
    pkill -f "mpc-server.*8082" 2>/dev/null
    pkill -f "mpc-server.*8083" 2>/dev/null  
    pkill -f "mpc-server.*8084" 2>/dev/null
    sleep 2
}

# 启动服务器
start_server() {
    local server_type=$1
    local port=$2
    local log_file=$3
    
    echo "启动 $server_type 服务器 (端口: $port)..."
    
    # 设置环境变量
    export SERVER_TYPE=$server_type
    export SERVER_PORT=$port
    export LOG_LEVEL=info
    
    # 启动服务器
    ./mpc-server -port=$port -type=$server_type > logs/$log_file 2>&1 &
    local pid=$!
    
    echo "$server_type 服务器已启动 (PID: $pid, 端口: $port)"
    
    # 等待服务器启动
    sleep 2
    
    # 检查服务器是否成功启动
    if curl -s http://localhost:$port/api/v1/health >/dev/null 2>&1; then
        echo "$server_type 服务器启动成功"
    else
        echo "$server_type 服务器启动失败"
        return 1
    fi
    
    return 0
}

# 主函数
main() {
    # 检查mpc-server可执行文件
    if [ ! -f "./mpc-server" ]; then
        echo "错误: 找不到 mpc-server 可执行文件"
        echo "请先运行: go build -o mpc-server cmd/server/main.go"
        exit 1
    fi
    
    # 创建日志目录
    mkdir -p logs
    
    # 停止现有服务器
    stop_servers
    
    # 启动三个服务器实例
    echo "=" * 50
    echo "启动MPC服务器集群..."
    echo "=" * 50
    
    # Enterprise服务器 (协调节点)
    start_server "enterprise" 8082 "enterprise.log"
    if [ $? -ne 0 ]; then
        echo "Enterprise服务器启动失败"
        exit 1
    fi
    
    # Mobile服务器
    start_server "mobile" 8083 "mobile-app.log"
    if [ $? -ne 0 ]; then
        echo "Mobile服务器启动失败"
        exit 1
    fi
    
    # Third-party服务器
    start_server "third_party" 8084 "third-party.log"
    if [ $? -ne 0 ]; then
        echo "Third-party服务器启动失败"
        exit 1
    fi
    
    echo "=" * 50
    echo "所有服务器启动完成！"
    echo "=" * 50
    echo "Enterprise服务器:   http://localhost:8082"
    echo "Mobile服务器:       http://localhost:8083"
    echo "Third-party服务器:  http://localhost:8084"
    echo "=" * 50
    echo ""
    echo "现在可以运行演示程序:"
    echo "python3 demo/three_party_demo.py"
    echo ""
    echo "查看日志:"
    echo "tail -f logs/enterprise.log"
    echo "tail -f logs/mobile-app.log"
    echo "tail -f logs/third-party.log"
    echo ""
    echo "停止服务器:"
    echo "./scripts/stop_three_party_servers.sh"
}

# 运行主函数
main "$@"