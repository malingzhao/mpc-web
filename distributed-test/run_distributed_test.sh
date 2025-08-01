#!/bin/bash

# 分布式MPC测试脚本
# 启动Java客户端和两个Go客户端进行协作密钥生成测试

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 项目根目录
PROJECT_ROOT="/Users/malltony/mpc/threshold-lib"
DISTRIBUTED_TEST_DIR="$PROJECT_ROOT/distributed-test"

echo -e "${BLUE}🚀 开始分布式MPC测试${NC}"
echo "=================================================="

# 检查依赖
check_dependencies() {
    echo -e "${YELLOW}🔍 检查依赖...${NC}"
    
    # 检查Go
    if ! command -v go &> /dev/null; then
        echo -e "${RED}❌ Go未安装${NC}"
        exit 1
    fi
    echo -e "${GREEN}✅ Go已安装: $(go version)${NC}"
    
    # 检查Java
    # 首先尝试检测Homebrew安装的OpenJDK
    if command -v brew &> /dev/null; then
        OPENJDK_PATH=$(brew --prefix openjdk 2>/dev/null)
        if [ -n "$OPENJDK_PATH" ] && [ -d "$OPENJDK_PATH" ]; then
            export JAVA_HOME="$OPENJDK_PATH"
            export PATH="$JAVA_HOME/bin:$PATH"
            echo -e "${GREEN}✅ 检测到Homebrew OpenJDK: $JAVA_HOME${NC}"
        fi
    fi
    
    # 检查系统Java
    if ! command -v java &> /dev/null; then
        echo -e "${RED}❌ Java未安装${NC}"
        echo "请安装Java: brew install openjdk"
        exit 1
    fi
    echo -e "${GREEN}✅ Java已安装: $(java -version 2>&1 | head -n 1)${NC}"
    
    # 检查JNI库
    if [ ! -f "$PROJECT_ROOT/sdk2/libmpcjni.dylib" ]; then
        echo -e "${RED}❌ MPC JNI库不存在${NC}"
        echo "请先编译JNI库: cd $PROJECT_ROOT/sdk2 && make"
        exit 1
    fi
    echo -e "${GREEN}✅ MPC JNI库存在${NC}"
}

# 编译Go模块
compile_go_modules() {
    echo -e "${YELLOW}🔨 编译Go模块...${NC}"
    
    # 编译协调服务器
    echo "编译协调服务器..."
    cd "$DISTRIBUTED_TEST_DIR/coordinator"
    go mod init coordinator 2>/dev/null || true
    go mod tidy
    go build -o coordinator coordinator.go
    echo -e "${GREEN}✅ 协调服务器编译完成${NC}"
    
    # 编译Go客户端
    echo "编译Go客户端..."
    cd "$DISTRIBUTED_TEST_DIR/go-client"
    go mod init go-client 2>/dev/null || true
    
    # 添加threshold-lib依赖
    if [ ! -f "go.mod" ] || ! grep -q "github.com/okx/threshold-lib" go.mod; then
        echo "replace github.com/okx/threshold-lib => $PROJECT_ROOT" >> go.mod
        go mod edit -require=github.com/okx/threshold-lib@v0.0.0
    fi
    
    go mod tidy
    go build -o go-client go_client.go
    echo -e "${GREEN}✅ Go客户端编译完成${NC}"
}

# 编译Java客户端
compile_java_client() {
    echo -e "${YELLOW}🔨 编译Java客户端...${NC}"
    
    cd "$DISTRIBUTED_TEST_DIR/java-client"
    
    # 设置Java环境
    if [ -n "$JAVA_HOME" ]; then
        JAVA_BIN="$JAVA_HOME/bin/java"
        JAVAC_BIN="$JAVA_HOME/bin/javac"
    else
        JAVA_BIN="java"
        JAVAC_BIN="javac"
    fi
    
    # 下载依赖（如果需要）
    if [ ! -f "gson-2.8.9.jar" ]; then
        echo "下载Gson库..."
        curl -L -o gson-2.8.9.jar "https://repo1.maven.org/maven2/com/google/code/gson/gson/2.8.9/gson-2.8.9.jar"
    fi
    
    if [ ! -f "java-websocket-1.5.3.jar" ]; then
        echo "下载Java WebSocket库..."
        curl -L -o java-websocket-1.5.3.jar "https://repo1.maven.org/maven2/org/java-websocket/Java-WebSocket/1.5.3/Java-WebSocket-1.5.3.jar"
    fi
    
    # 编译Java代码
    echo "编译Java代码..."
    $JAVAC_BIN -cp ".:$PROJECT_ROOT/sdk2:gson-2.8.9.jar:java-websocket-1.5.3.jar:slf4j-api-1.7.36.jar:slf4j-simple-1.7.36.jar" com/example/distributed/DistributedMPCClient.java
    if [ $? -ne 0 ]; then
        echo "❌ Java客户端编译失败"
        return 1
    fi
    echo -e "${GREEN}✅ Java客户端编译完成${NC}"
}

# 启动协调服务器
start_coordinator() {
    echo -e "${YELLOW}🌐 启动协调服务器...${NC}"
    
    cd "$DISTRIBUTED_TEST_DIR/coordinator"
    ./coordinator &
    COORDINATOR_PID=$!
    
    # 等待服务器启动
    sleep 3
    
    # 检查服务器是否启动成功
    if curl -s http://localhost:8080/health > /dev/null; then
        echo -e "${GREEN}✅ 协调服务器启动成功 (PID: $COORDINATOR_PID)${NC}"
    else
        echo -e "${RED}❌ 协调服务器启动失败${NC}"
        exit 1
    fi
}

# 启动Go客户端
start_go_clients() {
    echo -e "${YELLOW}🔧 启动Go客户端...${NC}"
    
    cd "$DISTRIBUTED_TEST_DIR/go-client"
    
    # 启动第一个Go客户端
    echo "启动Go客户端1..."
    ./go-client go-client-1 ws://localhost:8080/ws 1 &
    GO_CLIENT_1_PID=$!
    sleep 1
    
    # 启动第二个Go客户端
    echo "启动Go客户端2..."
    ./go-client go-client-2 ws://localhost:8080/ws 2 &
    GO_CLIENT_2_PID=$!
    sleep 1
    
    echo -e "${GREEN}✅ Go客户端启动完成${NC}"
    echo "Go客户端1 PID: $GO_CLIENT_1_PID"
    echo "Go客户端2 PID: $GO_CLIENT_2_PID"
}

# 启动Java客户端
start_java_client() {
    echo -e "${YELLOW}☕ 启动Java客户端...${NC}"
    
    cd "$DISTRIBUTED_TEST_DIR/java-client"
    
    # 设置库路径
    export DYLD_LIBRARY_PATH="$PROJECT_ROOT/sdk2:$DYLD_LIBRARY_PATH"
    export LD_LIBRARY_PATH="$PROJECT_ROOT/sdk2:$LD_LIBRARY_PATH"
    
    # 使用正确的Java路径
    if [ -n "$JAVA_HOME" ]; then
        JAVA_BIN="$JAVA_HOME/bin/java"
    else
        JAVA_BIN="java"
    fi
    
    # 启动Java客户端
    $JAVA_BIN -cp ".:$PROJECT_ROOT/sdk2:gson-2.8.9.jar:java-websocket-1.5.3.jar:slf4j-api-1.7.36.jar:slf4j-simple-1.7.36.jar" \
         -Djava.library.path="$PROJECT_ROOT/sdk2" \
         com.example.distributed.DistributedMPCClient java-client ws://localhost:8080/ws 3 &
    JAVA_CLIENT_PID=$!
    
    echo -e "${GREEN}✅ Java客户端启动完成 (PID: $JAVA_CLIENT_PID)${NC}"
}

# 监控测试进度
monitor_test() {
    echo -e "${YELLOW}📊 监控测试进度...${NC}"
    echo "=================================================="
    
    # 等待测试完成
    local timeout=120  # 2分钟超时
    local elapsed=0
    
    while [ $elapsed -lt $timeout ]; do
        # 检查服务器状态
        if curl -s http://localhost:8080/api/v1/status > /dev/null; then
            status=$(curl -s http://localhost:8080/api/v1/status | grep -o '"clients":[0-9]*' | cut -d':' -f2)
            sessions=$(curl -s http://localhost:8080/api/v1/status | grep -o '"sessions":[0-9]*' | cut -d':' -f2)
            echo -e "${BLUE}📈 当前状态: 客户端数量=$status, 会话数量=$sessions${NC}"
        fi
        
        sleep 5
        elapsed=$((elapsed + 5))
    done
    
    echo -e "${YELLOW}⏰ 测试监控结束${NC}"
}

# 清理进程
cleanup() {
    echo -e "${YELLOW}🧹 清理进程...${NC}"
    
    # 终止所有启动的进程
    if [ ! -z "$JAVA_CLIENT_PID" ]; then
        kill $JAVA_CLIENT_PID 2>/dev/null || true
        echo "Java客户端已终止"
    fi
    
    if [ ! -z "$GO_CLIENT_1_PID" ]; then
        kill $GO_CLIENT_1_PID 2>/dev/null || true
        echo "Go客户端1已终止"
    fi
    
    if [ ! -z "$GO_CLIENT_2_PID" ]; then
        kill $GO_CLIENT_2_PID 2>/dev/null || true
        echo "Go客户端2已终止"
    fi
    
    if [ ! -z "$COORDINATOR_PID" ]; then
        kill $COORDINATOR_PID 2>/dev/null || true
        echo "协调服务器已终止"
    fi
    
    echo -e "${GREEN}✅ 清理完成${NC}"
}

# 设置信号处理
trap cleanup EXIT INT TERM

# 主执行流程
main() {
    echo -e "${BLUE}开始分布式MPC测试流程${NC}"
    
    # 1. 检查依赖
    check_dependencies
    
    # 2. 编译模块
    compile_go_modules
    compile_java_client
    
    # 3. 启动服务
    start_coordinator
    
    # 4. 启动客户端
    start_go_clients
    start_java_client
    
    # 5. 监控测试
    monitor_test
    
    echo -e "${GREEN}🎉 分布式MPC测试完成${NC}"
    echo "=================================================="
    echo "测试结果:"
    echo "- 协调服务器: http://localhost:8080"
    echo "- 状态API: http://localhost:8080/api/v1/status"
    echo "- 会话API: http://localhost:8080/api/v1/sessions"
    echo ""
    echo "按Ctrl+C退出并清理所有进程"
    
    # 保持脚本运行
    wait
}

# 如果直接运行脚本
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi