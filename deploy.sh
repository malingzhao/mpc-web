#!/bin/bash

# MPC钱包部署脚本
# 用于快速部署和配置MPC钱包节点

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置变量
PROJECT_NAME="mpc-wallet"
GO_VERSION="1.19"
THRESHOLD_LIB_VERSION="latest"

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查系统要求
check_requirements() {
    log_info "检查系统要求..."
    
    # 检查Go版本
    if ! command -v go &> /dev/null; then
        log_error "Go未安装，请先安装Go ${GO_VERSION}或更高版本"
        exit 1
    fi
    
    GO_CURRENT_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    log_info "当前Go版本: ${GO_CURRENT_VERSION}"
    
    # 检查Git
    if ! command -v git &> /dev/null; then
        log_error "Git未安装，请先安装Git"
        exit 1
    fi
    
    log_success "系统要求检查通过"
}

# 创建项目结构
create_project_structure() {
    log_info "创建项目结构..."
    
    mkdir -p ${PROJECT_NAME}/{cmd,internal,pkg,config,scripts,data,backups,certs,logs}
    mkdir -p ${PROJECT_NAME}/internal/{wallet,network,storage,crypto}
    mkdir -p ${PROJECT_NAME}/pkg/{api,utils}
    
    log_success "项目结构创建完成"
}

# 初始化Go模块
init_go_module() {
    log_info "初始化Go模块..."
    
    cd ${PROJECT_NAME}
    
    if [ ! -f "go.mod" ]; then
        go mod init ${PROJECT_NAME}
        log_success "Go模块初始化完成"
    else
        log_warning "Go模块已存在，跳过初始化"
    fi
    
    # 添加threshold-lib依赖
    log_info "添加threshold-lib依赖..."
    go get github.com/okx/threshold-lib@${THRESHOLD_LIB_VERSION}
    
    # 添加其他必要依赖
    go get github.com/gin-gonic/gin
    go get github.com/spf13/cobra
    go get github.com/spf13/viper
    go get github.com/syndtr/goleveldb/leveldb
    go get github.com/sirupsen/logrus
    go get golang.org/x/crypto/pbkdf2
    
    log_success "依赖添加完成"
}

# 生成配置文件
generate_configs() {
    log_info "生成配置文件..."
    
    # 为每个节点生成配置
    for i in {1..3}; do
        cat > config/node${i}_config.json << EOF
{
  "participant_id": ${i},
  "total_parties": 3,
  "threshold": 2,
  "wallet_name": "MPC-Wallet-Node-${i}",
  "network": {
    "listen_port": $((8080 + i - 1)),
    "peers": {
$(for j in {1..3}; do
    if [ $j -ne $i ]; then
        echo "      \"${j}\": {"
        echo "        \"address\": \"127.0.0.1\","
        echo "        \"port\": $((8080 + j - 1)),"
        echo "        \"public_key\": \"\""
        if [ $j -eq 3 ] && [ $i -eq 1 ]; then
            echo "      }"
        elif [ $j -eq 2 ] && [ $i -eq 3 ]; then
            echo "      }"
        else
            echo "      },"
        fi
    fi
done)
    },
    "tls_enabled": false,
    "timeout_seconds": 30,
    "retry_attempts": 3
  },
  "supported_coins": [
    {
      "symbol": "BTC",
      "name": "Bitcoin",
      "network": "testnet",
      "enabled": true,
      "derive_path": "m/44'/1'/0'",
      "rpc_endpoint": "https://testnet.blockstream.info/api"
    }
  ],
  "security": {
    "encryption_enabled": true,
    "key_derivation": "pbkdf2",
    "iteration_count": 100000,
    "salt_length": 32,
    "hsm_enabled": false
  },
  "storage": {
    "data_dir": "./data/node${i}",
    "backup_enabled": true,
    "backup_interval_hours": 24,
    "backup_location": "./backups/node${i}",
    "database_type": "leveldb"
  }
}
EOF
    done
    
    log_success "配置文件生成完成"
}

# 生成启动脚本
generate_start_scripts() {
    log_info "生成启动脚本..."
    
    # 主启动脚本
    cat > scripts/start_all.sh << 'EOF'
#!/bin/bash

# 启动所有MPC钱包节点

echo "启动MPC钱包集群..."

# 启动节点1
echo "启动节点1..."
./mpc-wallet --config=config/node1_config.json > logs/node1.log 2>&1 &
NODE1_PID=$!
echo "节点1 PID: $NODE1_PID"

sleep 2

# 启动节点2  
echo "启动节点2..."
./mpc-wallet --config=config/node2_config.json > logs/node2.log 2>&1 &
NODE2_PID=$!
echo "节点2 PID: $NODE2_PID"

sleep 2

# 启动节点3
echo "启动节点3..."
./mpc-wallet --config=config/node3_config.json > logs/node3.log 2>&1 &
NODE3_PID=$!
echo "节点3 PID: $NODE3_PID"

# 保存PID
echo "$NODE1_PID" > logs/node1.pid
echo "$NODE2_PID" > logs/node2.pid  
echo "$NODE3_PID" > logs/node3.pid

echo "所有节点启动完成"
echo "使用 ./scripts/stop_all.sh 停止所有节点"
EOF

    # 停止脚本
    cat > scripts/stop_all.sh << 'EOF'
#!/bin/bash

# 停止所有MPC钱包节点

echo "停止MPC钱包集群..."

for i in {1..3}; do
    if [ -f "logs/node${i}.pid" ]; then
        PID=$(cat logs/node${i}.pid)
        if kill -0 $PID 2>/dev/null; then
            echo "停止节点${i} (PID: $PID)..."
            kill $PID
            rm logs/node${i}.pid
        else
            echo "节点${i}已停止"
        fi
    else
        echo "未找到节点${i}的PID文件"
    fi
done

echo "所有节点已停止"
EOF

    # 单节点启动脚本
    for i in {1..3}; do
        cat > scripts/start_node${i}.sh << EOF
#!/bin/bash
echo "启动MPC钱包节点${i}..."
./mpc-wallet --config=config/node${i}_config.json
EOF
        chmod +x scripts/start_node${i}.sh
    done
    
    chmod +x scripts/start_all.sh
    chmod +x scripts/stop_all.sh
    
    log_success "启动脚本生成完成"
}

# 生成主程序
generate_main_program() {
    log_info "生成主程序..."
    
    cat > cmd/main.go << 'EOF'
package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    
    "github.com/sirupsen/logrus"
)

var (
    configFile = flag.String("config", "config/wallet_config.json", "配置文件路径")
    version    = flag.Bool("version", false, "显示版本信息")
)

func main() {
    flag.Parse()
    
    if *version {
        fmt.Println("MPC Wallet v1.0.0")
        fmt.Println("基于threshold-lib的多方计算钱包")
        return
    }
    
    // 设置日志
    logrus.SetLevel(logrus.InfoLevel)
    logrus.SetFormatter(&logrus.TextFormatter{
        FullTimestamp: true,
    })
    
    logrus.Infof("启动MPC钱包，配置文件: %s", *configFile)
    
    // TODO: 加载配置
    // TODO: 初始化钱包
    // TODO: 启动网络服务
    
    // 等待退出信号
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    
    logrus.Info("MPC钱包启动完成，按Ctrl+C退出")
    <-c
    
    logrus.Info("正在关闭MPC钱包...")
    // TODO: 清理资源
    logrus.Info("MPC钱包已关闭")
}
EOF
    
    log_success "主程序生成完成"
}

# 生成Makefile
generate_makefile() {
    log_info "生成Makefile..."
    
    cat > Makefile << 'EOF'
.PHONY: build clean test run install deps

# 变量定义
BINARY_NAME=mpc-wallet
BUILD_DIR=build
GO_FILES=$(shell find . -name "*.go" -type f)

# 默认目标
all: build

# 构建
build: deps
	@echo "构建MPC钱包..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) cmd/main.go
	@echo "构建完成: $(BUILD_DIR)/$(BINARY_NAME)"

# 安装依赖
deps:
	@echo "安装依赖..."
	go mod download
	go mod tidy

# 运行测试
test:
	@echo "运行测试..."
	go test -v ./...

# 清理
clean:
	@echo "清理构建文件..."
	rm -rf $(BUILD_DIR)
	rm -rf data/*/
	rm -rf logs/*.log
	rm -rf logs/*.pid

# 运行节点1
run1: build
	@echo "启动节点1..."
	./$(BUILD_DIR)/$(BINARY_NAME) --config=config/node1_config.json

# 运行节点2  
run2: build
	@echo "启动节点2..."
	./$(BUILD_DIR)/$(BINARY_NAME) --config=config/node2_config.json

# 运行节点3
run3: build
	@echo "启动节点3..."
	./$(BUILD_DIR)/$(BINARY_NAME) --config=config/node3_config.json

# 启动所有节点
start-all: build
	@echo "启动所有节点..."
	cp $(BUILD_DIR)/$(BINARY_NAME) ./
	./scripts/start_all.sh

# 停止所有节点
stop-all:
	@echo "停止所有节点..."
	./scripts/stop_all.sh

# 安装
install: build
	@echo "安装MPC钱包..."
	cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/

# 格式化代码
fmt:
	@echo "格式化代码..."
	go fmt ./...

# 代码检查
lint:
	@echo "代码检查..."
	golangci-lint run

# 生成文档
docs:
	@echo "生成文档..."
	godoc -http=:6060

# 帮助
help:
	@echo "可用命令:"
	@echo "  build      - 构建项目"
	@echo "  test       - 运行测试"
	@echo "  clean      - 清理构建文件"
	@echo "  run1/2/3   - 运行指定节点"
	@echo "  start-all  - 启动所有节点"
	@echo "  stop-all   - 停止所有节点"
	@echo "  install    - 安装到系统"
	@echo "  fmt        - 格式化代码"
	@echo "  lint       - 代码检查"
	@echo "  docs       - 生成文档"
EOF
    
    log_success "Makefile生成完成"
}

# 生成README
generate_readme() {
    log_info "生成README..."
    
    cat > README.md << 'EOF'
# MPC钱包

基于threshold-lib的多方计算钱包实现

## 功能特性

- 🔐 2/n阈值签名 - 任意2方即可完成签名
- 🪙 多币种支持 - 支持BTC、ETH等主流币种  
- 🔑 BIP32密钥派生 - 分层确定性钱包
- 🔄 密钥分片刷新 - 支持密钥轮换和新成员加入
- 🛡️ 零知识证明 - 保证计算过程的安全性
- 🌐 分布式架构 - 无单点故障

## 快速开始

### 1. 环境要求

- Go 1.19+
- Git

### 2. 部署

```bash
# 运行部署脚本
./deploy.sh

# 或手动构建
cd mpc-wallet
make build
```

### 3. 启动

```bash
# 启动所有节点
make start-all

# 或单独启动节点
make run1  # 启动节点1
make run2  # 启动节点2  
make run3  # 启动节点3
```

### 4. 停止

```bash
make stop-all
```

## 配置说明

每个节点的配置文件位于 `config/` 目录：

- `node1_config.json` - 节点1配置
- `node2_config.json` - 节点2配置
- `node3_config.json` - 节点3配置

主要配置项：

```json
{
  "participant_id": 1,        // 参与方ID
  "total_parties": 3,         // 总参与方数量
  "threshold": 2,             // 签名阈值
  "network": {
    "listen_port": 8080,      // 监听端口
    "peers": {...}            // 其他节点信息
  },
  "supported_coins": [...],   // 支持的币种
  "security": {...},          // 安全配置
  "storage": {...}            // 存储配置
}
```

## API接口

### 生成密钥分片

```bash
POST /api/v1/keygen
{
  "coin_type": "BTC"
}
```

### 签名交易

```bash
POST /api/v1/sign
{
  "coin_type": "BTC",
  "message": "hex_encoded_message",
  "signer_id": 2
}
```

### 派生子密钥

```bash
POST /api/v1/derive
{
  "coin_type": "BTC", 
  "path": [44, 0, 0, 0, 1]
}
```

## 目录结构

```
mpc-wallet/
├── cmd/                    # 主程序入口
├── internal/               # 内部包
│   ├── wallet/            # 钱包核心逻辑
│   ├── network/           # 网络通信
│   ├── storage/           # 数据存储
│   └── crypto/            # 密码学工具
├── pkg/                   # 公共包
├── config/                # 配置文件
├── scripts/               # 脚本文件
├── data/                  # 数据目录
├── backups/               # 备份目录
├── logs/                  # 日志目录
└── certs/                 # 证书目录
```

## 安全注意事项

1. **密钥保护** - 确保Paillier私钥安全存储
2. **网络安全** - 生产环境启用TLS加密
3. **访问控制** - 限制API访问权限
4. **备份策略** - 定期备份密钥分片
5. **监控告警** - 监控节点状态和异常

## 开发指南

### 构建

```bash
make build
```

### 测试

```bash
make test
```

### 代码格式化

```bash
make fmt
```

### 清理

```bash
make clean
```

## 许可证

Apache-2.0 License

## 贡献

欢迎提交Issue和Pull Request！
EOF
    
    log_success "README生成完成"
}

# 主函数
main() {
    log_info "开始部署MPC钱包..."
    
    check_requirements
    create_project_structure
    init_go_module
    generate_configs
    generate_start_scripts
    generate_main_program
    generate_makefile
    generate_readme
    
    cd ..
    
    log_success "MPC钱包部署完成！"
    log_info "项目位置: $(pwd)/${PROJECT_NAME}"
    log_info "下一步操作:"
    log_info "  1. cd ${PROJECT_NAME}"
    log_info "  2. make build"
    log_info "  3. make start-all"
    log_info ""
    log_info "查看README.md了解更多使用说明"
}

# 执行主函数
main "$@"