# 2方MPC签名架构改进方案

## 问题分析

当前的MPC签名实现存在以下问题：

1. **架构不匹配**：底层ECDSA签名算法是2方协议，但上层架构假设需要多方参与
2. **信息交换缺失**：每个签名步骤都需要参与者间的数据交换，但当前实现没有正确处理
3. **不必要的复杂性**：third-party服务器实际上不需要参与签名过程

## 改进方案

### 1. 架构简化

**原架构**：
```
Client -> Enterprise Server -> Third-party Server (不必要)
```

**新架构**：
```
Mobile App -> Enterprise Server (2方签名)
```

### 2. 签名流程优化

#### 当前流程问题
```go
// 当前的签名流程在单个函数中模拟了所有步骤
func (m *MPCManager) performRealECDSASign(sessionID string, threshold int, participants []string) error {
    // 问题1: 在同一个函数中创建P1和P2上下文
    p1 := sign.NewP1(publicKey, messageHex, paiPriKey, E_x1, pedParams)
    p2 := sign.NewP2(x2, E_x1, publicKey, paiPubKey, messageHex, pedParams)
    
    // 问题2: 没有真正的网络通信，只是本地调用
    cmtC, err := p1.Step1()
    p2Proof, R2, err := p2.Step1(cmtC)
    // ...
}
```

#### 改进后的流程
```go
// 每个参与者只维护自己的上下文
type TwoPartySignSession struct {
    IsInitiator bool
    CurrentStep SignStep
    P1Context   *sign.P1Context  // 只有P1角色的服务器有这个
    P2Context   *sign.P2Context  // 只有P2角色的服务器有这个
    // 网络通信的中间数据
    IntermediateData map[string]interface{}
}
```

### 3. 详细的信息交换流程

#### 步骤1: 预参数生成和交换
```
P1 (Enterprise):
1. 生成 PreParamsWithDlnProof
2. 生成 Paillier 密钥对
3. 发送预参数给 P2

P2 (Mobile):
1. 接收 P1 的预参数
2. 确认接收并准备进入密钥协商阶段
```

#### 步骤2: 密钥协商数据交换
```
P1:
1. 调用 keygen.P1() 生成 p1Dto 和 E_x1
2. 发送 p1Dto 给 P2

P2:
1. 接收 p1Dto
2. 调用 keygen.P2() 生成 p2SaveData
3. 发送确认给 P1
```

#### 步骤3: 签名轮次交换
```
Round 1:
P1: 生成承诺 -> 发送给 P2
P2: 接收承诺 -> 生成 Schnorr 证明和 R2 -> 发送给 P1

Round 2:
P1: 接收 P2 数据 -> 生成 Schnorr 证明和承诺开启 -> 发送给 P2
P2: 接收 P1 数据 -> 生成加密值和仿射证明 -> 发送给 P1

Round 3:
P1: 接收 P2 数据 -> 计算最终签名 (r, s)
```

### 4. 实现要点

#### 4.1 消息路由
```go
// 新增专门的2方签名消息处理
func (h *Handler) handleTwoPartySignMessage(msg *protocol.Message) {
    switch msg.Type {
    case protocol.MsgTypeTwoPartySignInit:
        h.handleTwoPartySignInit(msg)
    case protocol.MsgTypePreParams:
        h.handlePreParams(msg)
    case protocol.MsgTypeKeygenP1Data:
        h.handleKeygenP1Data(msg)
    // ... 其他消息类型
    }
}
```

#### 4.2 状态管理
```go
type SignStep int

const (
    StepPreParams SignStep = iota + 1
    StepKeygenP1
    StepKeygenP2
    StepSignRound1
    StepSignRound2
    StepSignRound3
    StepCompleted
)
```

#### 4.3 错误处理
```go
// 每个步骤都需要适当的错误处理和重试机制
func (h *Handler) handleSignStep(session *mpc.Session, step SignStep) error {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("Sign step %d failed: %v", step, r)
            session.Status = mpc.StatusFailed
        }
    }()
    
    // 具体步骤实现...
}
```

### 5. API 接口设计

#### 5.1 新的签名接口
```http
POST /api/v1/sign/two-party
{
    "session_id": "keygen_session_id",
    "message": "要签名的消息",
    "partner": "mobile-app"
}
```

#### 5.2 状态查询接口
```http
GET /api/v1/sessions/{session_id}
```

### 6. 部署建议

#### 6.1 服务器配置
```yaml
# Enterprise Server (端口 8082)
capabilities: ["keygen", "sign"]
role: "enterprise"
partners: ["mobile-app"]

# Mobile App Server (端口 8083) 
capabilities: ["keygen", "sign"]
role: "mobile-app"
partners: ["enterprise"]
```

#### 6.2 网络通信
- 使用 WebSocket 进行实时通信
- 实现消息确认和重试机制
- 添加超时处理

### 7. 测试策略

#### 7.1 单元测试
- 测试每个签名步骤的正确性
- 测试消息序列化/反序列化
- 测试错误处理

#### 7.2 集成测试
- 测试完整的2方签名流程
- 测试网络中断恢复
- 测试并发签名请求

#### 7.3 性能测试
- 测试签名延迟
- 测试吞吐量
- 测试资源使用

## 实施步骤

1. **第一阶段**：实现基本的2方签名消息协议
2. **第二阶段**：实现完整的信息交换流程
3. **第三阶段**：添加错误处理和重试机制
4. **第四阶段**：性能优化和测试

## 预期收益

1. **简化架构**：移除不必要的第三方服务器参与
2. **提高可靠性**：正确实现信息交换，避免"no message to sign"错误
3. **提升性能**：减少不必要的网络通信
4. **增强可维护性**：清晰的步骤划分和状态管理