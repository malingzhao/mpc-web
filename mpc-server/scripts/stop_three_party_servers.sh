#!/bin/bash

# 停止三方MPC服务器脚本

echo "停止三方MPC服务器..."

# 查找并停止MPC服务器进程
stop_mpc_servers() {
    echo "查找MPC服务器进程..."
    
    # 查找端口8082-8084的进程
    for port in 8082 8083 8084; do
        pid=$(lsof -ti:$port 2>/dev/null)
        if [ ! -z "$pid" ]; then
            echo "停止端口 $port 上的进程 (PID: $pid)..."
            kill $pid
            sleep 1
            
            # 如果进程仍在运行，强制杀死
            if kill -0 $pid 2>/dev/null; then
                echo "强制停止进程 $pid..."
                kill -9 $pid
            fi
        else
            echo "端口 $port 上没有运行的进程"
        fi
    done
}

# 通过进程名停止
stop_by_name() {
    echo "通过进程名停止MPC服务器..."
    pkill -f "mpc-server" 2>/dev/null
    sleep 1
}

# 主函数
main() {
    echo "=" * 40
    echo "停止MPC服务器集群"
    echo "=" * 40
    
    # 停止服务器
    stop_mpc_servers
    stop_by_name
    
    # 验证是否停止成功
    echo "验证服务器是否已停止..."
    sleep 2
    
    running_processes=0
    for port in 8082 8083 8084; do
        if lsof -ti:$port >/dev/null 2>&1; then
            echo "警告: 端口 $port 上仍有进程在运行"
            running_processes=$((running_processes + 1))
        else
            echo "端口 $port 已释放"
        fi
    done
    
    if [ $running_processes -eq 0 ]; then
        echo "=" * 40
        echo "所有MPC服务器已成功停止"
        echo "=" * 40
    else
        echo "=" * 40
        echo "警告: 仍有 $running_processes 个服务器进程在运行"
        echo "请手动检查并停止相关进程"
        echo "=" * 40
    fi
}

# 运行主函数
main "$@"