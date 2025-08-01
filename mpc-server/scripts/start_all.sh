#!/bin/bash

# 启动所有MPC服务器的脚本

echo "=== 启动MPC服务器集群 ==="

# 创建日志目录
mkdir -p logs

# 启动third-party服务器
echo "启动 third-party 服务器 (端口 8081)..."
nohup ./bin/mpc-server -server third-party > logs/third-party.log 2>&1 &
THIRD_PARTY_PID=$!
echo "third-party PID: $THIRD_PARTY_PID"

# 等待一秒
sleep 1

# 启动enterprise服务器
echo "启动 enterprise 服务器 (端口 8082)..."
nohup ./bin/mpc-server -server enterprise > logs/enterprise.log 2>&1 &
ENTERPRISE_PID=$!
echo "enterprise PID: $ENTERPRISE_PID"

# 等待一秒
sleep 1

# 启动mobile-app服务器
echo "启动 mobile-app 服务器 (端口 8083)..."
nohup ./bin/mpc-server -server mobile-app > logs/mobile-app.log 2>&1 &
MOBILE_APP_PID=$!
echo "mobile-app PID: $MOBILE_APP_PID"

# 保存PID到文件
echo $THIRD_PARTY_PID > logs/third-party.pid
echo $ENTERPRISE_PID > logs/enterprise.pid
echo $MOBILE_APP_PID > logs/mobile-app.pid

echo ""
echo "所有服务器已启动！"
echo "PID文件保存在 logs/ 目录中"
echo ""
echo "使用以下命令查看日志："
echo "  tail -f logs/third-party.log"
echo "  tail -f logs/enterprise.log"
echo "  tail -f logs/mobile-app.log"
echo ""
echo "使用 ./scripts/stop_servers.sh 停止所有服务器"
echo "使用 ./scripts/test_servers.sh 测试服务器"

# 等待几秒让服务器启动
echo "等待服务器启动..."
sleep 3

# 检查服务器是否启动成功
echo "检查服务器状态..."
./scripts/test_servers.sh