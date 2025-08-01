# 分布式MPC测试项目

这个项目演示了Java和Go客户端协作完成分布式MPC密钥生成的完整流程。

## 项目结构

```
distributed-test/
├── coordinator/           # 协调服务器 (Go)
│   ├── coordinator.go    # 主服务器代码
│   └── go.mod           # Go模块文件
├── java-client/          # Java客户端
│   └── DistributedMPCClient.java
├── go-client/            # Go客户端
│   ├── go_client.go     # 客户端代码
│   └── go.mod           # Go模块文件
├── run_distributed_test.sh  # 自动化测试脚本
└── README.md            # 本文件
```

## 系统架构

### 参与方
- **Java客户端**: 参与方1，使用JNI调用底层MPC库
- **Go客户端1**: 参与方2，直接使用Go MPC库
- **Go客户端2**: 参与方3，直接使用Go MPC库

### 通信流程
1. **协调服务器**: 管理客户端连接和消息路由
2. **WebSocket通信**: 实时双向通信
3. **三轮密钥生成**: 
   - Round 1: 各方生成初始承诺
   - Round 2: 交换中间数据
   - Round 3: 生成最终密钥分片

## 快速开始

### 前置条件
- Go 1.21+
- Java 8+
- 已编译的MPC JNI库 (`libmpcjni.dylib`)

### 自动化运行
```bash
# 给脚本执行权限
chmod +x run_distributed_test.sh

# 运行完整测试
./run_distributed_test.sh
```

### 手动运行

#### 1. 编译协调服务器
```bash
cd coordinator
go mod tidy
go build -o coordinator coordinator.go
```

#### 2. 编译Go客户端
```bash
cd go-client
go mod tidy
go build -o go-client go_client.go
```

#### 3. 编译Java客户端
```bash
cd java-client
# 下载依赖
curl -L -o gson-2.8.9.jar "https://repo1.maven.org/maven2/com/google/code/gson/gson/2.8.9/gson-2.8.9.jar"
curl -L -o java-websocket-1.5.3.jar "https://repo1.maven.org/maven2/org/java-websocket/Java-WebSocket/1.5.3/Java-WebSocket-1.5.3.jar"

# 编译
javac -cp ".:gson-2.8.9.jar:java-websocket-1.5.3.jar" DistributedMPCClient.java
```

#### 4. 启动服务

**启动协调服务器:**
```bash
cd coordinator
./coordinator
```

**启动Go客户端:**
```bash
cd go-client
./go-client go-client-1 ws://localhost:8080/ws 2 &
./go-client go-client-2 ws://localhost:8080/ws 3 &
```

**启动Java客户端:**
```bash
cd java-client
export DYLD_LIBRARY_PATH="/Users/malltony/mpc/threshold-lib/sdk2:$DYLD_LIBRARY_PATH"
java -cp ".:gson-2.8.9.jar:java-websocket-1.5.3.jar" \
     -Djava.library.path="/Users/malltony/mpc/threshold-lib/sdk2" \
     DistributedMPCClient java-client ws://localhost:8080/ws
```

## API接口

### 协调服务器端点
- **WebSocket**: `ws://localhost:8080/ws`
- **健康检查**: `GET http://localhost:8080/health`
- **状态查询**: `GET http://localhost:8080/api/v1/status`
- **会话列表**: `GET http://localhost:8080/api/v1/sessions`

### WebSocket消息格式

#### 创建会话
```json
{
  "type": "create_session",
  "client_id": "java-client",
  "session_type": "keygen",
  "threshold": 2,
  "total_parties": 3
}
```

#### 密钥生成轮次
```json
{
  "type": "keygen_round1",
  "session_id": "session_1234567890",
  "from_party": 1,
  "data": "base64_encoded_data"
}
```

#### 完成通知
```json
{
  "type": "keygen_complete",
  "session_id": "session_1234567890",
  "from_party": 1,
  "success": true
}
```

## 测试场景

### 基本密钥生成测试
1. 三个客户端连接到协调服务器
2. Java客户端发起密钥生成会话
3. 所有客户端执行三轮MPC协议
4. 验证密钥生成成功

### 跨平台兼容性测试
- **数据序列化**: 测试不同语言间的数据交换
- **大数运算**: 验证密码学计算的一致性
- **网络通信**: 测试WebSocket连接稳定性

## 故障排除

### 常见问题

1. **JNI库加载失败**
   ```
   解决方案: 确保libmpcjni.dylib在正确路径，设置DYLD_LIBRARY_PATH
   ```

2. **WebSocket连接失败**
   ```
   解决方案: 检查协调服务器是否启动，端口8080是否被占用
   ```

3. **密钥生成超时**
   ```
   解决方案: 检查所有客户端是否正常连接，查看日志输出
   ```

### 调试模式
启用详细日志输出:
```bash
export MPC_DEBUG=1
./run_distributed_test.sh
```

## 扩展功能

### 支持更多参与方
修改配置参数:
```go
// 在客户端代码中修改
totalParties := 5  // 支持5方
threshold := 3     // 3-of-5阈值
```

### 添加签名功能
在密钥生成完成后，可以扩展支持分布式签名:
1. 实现签名协议
2. 添加签名消息类型
3. 扩展协调服务器路由

## 性能指标

### 预期性能
- **密钥生成时间**: < 30秒 (3方)
- **网络延迟**: < 100ms (本地测试)
- **内存使用**: < 100MB (每个客户端)

### 监控指标
- 连接客户端数量
- 活跃会话数量
- 消息传输延迟
- 错误率统计

## 安全考虑

### 网络安全
- 生产环境应使用WSS (WebSocket Secure)
- 实现客户端身份验证
- 添加消息完整性校验

### 密码学安全
- 确保随机数生成器质量
- 验证密钥分片的正确性
- 实现安全的密钥存储

## 贡献指南

1. Fork项目
2. 创建功能分支
3. 提交更改
4. 创建Pull Request

## 许可证

本项目遵循与threshold-lib相同的许可证。