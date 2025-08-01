#!/bin/bash

# MPC Server 启动脚本

echo "Starting MPC Servers..."

# 检查是否已有进程在运行
check_port() {
    local port=$1
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null ; then
        echo "Port $port is already in use"
        return 1
    fi
    return 0
}

# 停止所有MPC服务器
stop_servers() {
    echo "Stopping all MPC servers..."
    pkill -f "mpc-server.*-server"
    sleep 2
}

# 启动单个服务器
start_server() {
    local server_id=$1
    local port=$2
    
    echo "Starting $server_id server on port $port..."
    go run cmd/server/main.go -server $server_id > logs/${server_id}.log 2>&1 &
    local pid=$!
    echo "Started $server_id server with PID $pid"
    sleep 1
}

# 创建日志目录
mkdir -p logs

# 检查命令行参数
case "$1" in
    "start")
        echo "Starting all MPC servers..."
        
        # 检查端口是否可用
        check_port 8081 || exit 1
        check_port 8082 || exit 1
        check_port 8083 || exit 1
        
        # 启动三个服务器
        start_server "third-party" 8081
        start_server "enterprise" 8082
        start_server "mobile-app" 8083
        
        echo "All servers started successfully!"
        echo "Third-party server: http://localhost:8081"
        echo "Enterprise server: http://localhost:8082"
        echo "Mobile-app server: http://localhost:8083"
        echo ""
        echo "Check logs in the logs/ directory"
        echo "Use './scripts/start.sh stop' to stop all servers"
        ;;
        
    "stop")
        stop_servers
        echo "All servers stopped"
        ;;
        
    "restart")
        stop_servers
        echo "Restarting servers..."
        sleep 2
        $0 start
        ;;
        
    "status")
        echo "Checking server status..."
        echo ""
        
        # 检查第三方服务器
        if curl -s http://localhost:8081/health >/dev/null 2>&1; then
            echo "✓ Third-party server (8081): Running"
        else
            echo "✗ Third-party server (8081): Not running"
        fi
        
        # 检查企业服务器
        if curl -s http://localhost:8082/health >/dev/null 2>&1; then
            echo "✓ Enterprise server (8082): Running"
        else
            echo "✗ Enterprise server (8082): Not running"
        fi
        
        # 检查移动应用服务器
        if curl -s http://localhost:8083/health >/dev/null 2>&1; then
            echo "✓ Mobile-app server (8083): Running"
        else
            echo "✗ Mobile-app server (8083): Not running"
        fi
        ;;
        
    "logs")
        echo "Showing recent logs..."
        echo ""
        echo "=== Third-party server logs ==="
        tail -n 20 logs/third-party.log 2>/dev/null || echo "No logs found"
        echo ""
        echo "=== Enterprise server logs ==="
        tail -n 20 logs/enterprise.log 2>/dev/null || echo "No logs found"
        echo ""
        echo "=== Mobile-app server logs ==="
        tail -n 20 logs/mobile-app.log 2>/dev/null || echo "No logs found"
        ;;
        
    *)
        echo "Usage: $0 {start|stop|restart|status|logs}"
        echo ""
        echo "Commands:"
        echo "  start   - Start all MPC servers"
        echo "  stop    - Stop all MPC servers"
        echo "  restart - Restart all MPC servers"
        echo "  status  - Check server status"
        echo "  logs    - Show recent logs"
        exit 1
        ;;
esac