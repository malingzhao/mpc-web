# MPC Server 项目

一个基于Go语言实现的多方安全计算(MPC)服务器项目，支持密钥生成、密钥重分享和数字签名功能。

## 项目概述

本项目实现了一个完整的MPC服务器系统，包含三个不同角色的服务器实例：

1. **第三方服务器** (third-party) - 端口8081
   - 支持密钥生成(keygen)
   - 不支持密钥重分享和签名

2. **企业服务器** (enterprise) - 端口8082
   - 支持密钥生成(keygen)
   - 支持密钥重分享(reshare)
   - 支持数字签名(sign)

3. **移动应用服务器** (mobile-app) - 端口8083
   - 支持密钥生成(keygen)
   - 支持密钥重分享(reshare)
   - 支持数字签名(sign)

## 架构设计

### 通信方式
- **RESTful API**: 用于发起MPC操作
- **WebSocket**: 用于服务器间的实时通信
- **JSON**: 统一的数据交换格式

### 核心组件
- **MPC Manager**: 管理MPC会话和协议执行
- **WebSocket Hub**: 处理服务器间的实时通信
- **HTTP Handlers**: 处理RESTful API请求
- **Protocol**: 定义通信协议和消息格式

## 目录结构

```
mpc-server/
├── cmd/
│   └── server/
│       └── main.go              # 服务器主程序
├── internal/
│   ├── config/
│   │   └── config.go            # 服务器配置
│   ├── handlers/
│   │   └── handlers.go          # HTTP和WebSocket处理器
│   ├── mpc/
│   │   └── manager.go           # MPC核心逻辑
│   ├── protocol/
│   │   └── message.go           # 通信协议定义
│   └── websocket/
│       └── hub.go               # WebSocket连接管理
├── scripts/
│   ├── start.sh                 # 服务器启动脚本
│   └── test_client.py           # Python测试客户端
├── logs/                        # 日志目录
├── go.mod                       # Go模块定义
└── README.md                    # 项目文档
```

## 快速开始

### 1. 启动所有服务器

```bash
cd mpc-server
./scripts/start.sh start
```

### 2. 检查服务器状态

```bash
./scripts/start.sh status
```

### 3. 运行测试

```bash
python3 scripts/test_client.py
```

### 4. 停止所有服务器

```bash
./scripts/start.sh stop
```

## API接口

### 服务器信息
- `GET /api/v1/info` - 获取服务器信息

### 会话管理
- `GET /api/v1/sessions` - 列出所有会话
- `GET /api/v1/sessions/{sessionId}` - 获取会话状态

### MPC操作

#### 密钥生成
```bash
POST /api/v1/keygen
{
    "threshold": 2,
    "participants": ["third-party", "enterprise", "mobile-app"]
}
```

#### 密钥重分享
```bash
POST /api/v1/reshare
{
    "session_id": "原会话ID",
    "new_threshold": 2,
    "new_participants": ["enterprise", "mobile-app"]
}
```

#### 数字签名
```bash
POST /api/v1/sign
{
    "session_id": "密钥会话ID",
    "message": "要签名的消息",
    "signers": ["enterprise", "mobile-app"]
}
```

### WebSocket连接
- `GET /ws?client_id={clientId}` - 建立WebSocket连接

## 使用示例

### 1. 密钥生成示例

```bash
# 使用企业服务器发起密钥生成
curl -X POST http://localhost:8082/api/v1/keygen \
  -H "Content-Type: application/json" \
  -d '{
    "threshold": 2,
    "participants": ["third-party", "enterprise", "mobile-app"]
  }'
```

### 2. 密钥重分享示例

```bash
# 使用移动应用服务器发起密钥重分享
curl -X POST http://localhost:8083/api/v1/reshare \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "原密钥生成会话ID",
    "new_threshold": 2,
    "new_participants": ["enterprise", "mobile-app"]
  }'
```

### 3. 数字签名示例

```bash
# 使用企业服务器发起签名
curl -X POST http://localhost:8082/api/v1/sign \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "密钥会话ID",
    "message": "Hello, MPC World!",
    "signers": ["enterprise", "mobile-app"]
  }'
```

## 功能特性

- ✅ **多服务器架构**: 支持三个不同角色的服务器实例
- ✅ **灵活的权限控制**: 每个服务器有不同的功能权限
- ✅ **实时通信**: 基于WebSocket的服务器间通信
- ✅ **RESTful API**: 简洁的HTTP接口
- ✅ **会话管理**: 完整的MPC会话生命周期管理
- ✅ **错误处理**: 完善的错误处理和状态管理
- ✅ **日志记录**: 详细的操作日志
- ✅ **健康检查**: 服务器健康状态监控
- ✅ **优雅关闭**: 支持优雅的服务器关闭

## 技术栈

- **Go 1.21+**: 主要编程语言
- **Gin**: HTTP Web框架
- **Gorilla WebSocket**: WebSocket通信
- **UUID**: 唯一标识符生成
- **Threshold-lib**: 阈值签名库

## 开发和调试

### 查看日志
```bash
./scripts/start.sh logs
```

### 单独启动服务器
```bash
# 启动第三方服务器
go run cmd/server/main.go -server third-party

# 启动企业服务器
go run cmd/server/main.go -server enterprise

# 启动移动应用服务器
go run cmd/server/main.go -server mobile-app
```

### 健康检查
```bash
curl http://localhost:8081/health  # 第三方服务器
curl http://localhost:8082/health  # 企业服务器
curl http://localhost:8083/health  # 移动应用服务器
```

## 注意事项

1. **端口占用**: 确保端口8081、8082、8083未被其他程序占用
2. **权限限制**: 第三方服务器不支持reshare和sign操作
3. **会话依赖**: reshare和sign操作需要先完成keygen
4. **网络连接**: 确保服务器间可以正常进行WebSocket通信

## 故障排除

### 常见问题

1. **端口被占用**
   ```bash
   lsof -i :8081  # 检查端口占用
   ```

2. **服务器无法启动**
   - 检查Go版本是否为1.21+
   - 确认依赖包已正确安装

3. **WebSocket连接失败**
   - 检查防火墙设置
   - 确认服务器正常运行

4. **API调用失败**
   - 检查请求格式是否正确
   - 确认服务器支持相应功能

## 许可证

本项目基于MIT许可证开源。