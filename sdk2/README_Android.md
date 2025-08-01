# Android MPC SDK 使用指南

这个SDK允许Android应用直接调用`libmpc.h`中的MPC函数，实现三方密钥生成、密钥刷新和两方签名。

## 架构说明

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   MPC Server    │    │  Third Party    │    │  Android App    │
│   (Party 1)     │    │   (Party 2)     │    │   (Party 3)     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    三方密钥生成 & 密钥刷新
                                 │
         ┌───────────────────────┼───────────────────────┐
         │                       │                       │
┌─────────────────┐                            ┌─────────────────┐
│   MPC Server    │ ←─────── 两方签名 ────────→ │  Android App    │
│   (Party 1)     │                            │   (Party 3)     │
└─────────────────┘                            └─────────────────┘
```

## 核心组件

### 1. MPCNative.java
直接映射`libmpc.h`中所有MPC函数的JNI接口：
- 密钥生成：`keygenInit`, `keygenRound1/2/3`, `keygenDestroy`
- 密钥刷新：`refreshInit`, `refreshRound1/2/3`, `refreshDestroy`
- Ed25519签名：`ed25519SignInit`, `ed25519SignRound1/2/3`, `ed25519SignDestroy`
- ECDSA签名：`ecdsaSignInitComplex`, `ecdsaSignStep1`, `ecdsaSignDestroy`

### 2. mpc_jni.c
JNI C实现，直接调用`libmpc.h`中的函数，处理Java和C之间的数据转换。

### 3. AndroidMPCExample.java
完整的使用示例，演示如何进行：
- 三方密钥生成
- 密钥刷新
- Ed25519两方签名
- ECDSA两方签名

## 构建步骤

### 1. 检查依赖
```bash
make -f Makefile.android check-deps
```

### 2. 构建JNI库
```bash
make -f Makefile.android android-lib
```

### 3. 编译Java类
```bash
make -f Makefile.android java-classes
```

### 4. 构建完整AAR包
```bash
make -f Makefile.android android-aar
```

### 5. 安装到Android项目
```bash
ANDROID_PROJECT_DIR=/path/to/your/android/project make -f Makefile.android install-android
```

## 使用方法

### 1. 在Android项目中添加依赖
在`app/build.gradle`中添加：
```gradle
dependencies {
    implementation files('libs/mpc-android-sdk.aar')
}
```

### 2. 加载本地库
```java
static {
    System.loadLibrary("mpc");
    System.loadLibrary("mpcjni");
}
```

### 3. 执行MPC操作

#### 三方密钥生成
```java
AndroidMPCExample client = new AndroidMPCExample(PARTY_ANDROID);
boolean success = client.performKeyGeneration(CURVE_ED25519);
```

#### 密钥刷新
```java
boolean success = client.performKeyRefresh(CURVE_ED25519);
```

#### Ed25519签名
```java
String message = "Hello, MPC World!";
String[] signature = client.performEd25519Signing(message.getBytes());
// signature[0] = R, signature[1] = S
```

## 网络通信

当前示例使用模拟数据交换。实际应用中需要实现：

### 1. WebSocket客户端
```java
public class MPCWebSocketClient {
    private WebSocket webSocket;
    
    public void connect(String serverUrl) {
        // 连接到MPC服务器
    }
    
    public void sendMessage(int toParty, byte[] data) {
        // 发送消息到指定参与方
    }
    
    public byte[] receiveMessage(int fromParty) {
        // 接收来自指定参与方的消息
    }
}
```

### 2. 消息路由
```java
public class MPCMessageRouter {
    private MPCWebSocketClient wsClient;
    
    public void routeMessage(int round, int toParty, byte[] data) {
        // 根据轮次和目标方路由消息
    }
    
    public byte[] waitForMessage(int round, int fromParty) {
        // 等待特定轮次和来源方的消息
    }
}
```

## 错误处理

### 1. 检查返回值
```java
long handle = MPCNative.keygenInit(curve, partyID, threshold, totalParties);
if (handle == 0) {
    String error = MPCNative.getErrorString(-1);
    Log.e("MPC", "密钥生成初始化失败: " + error);
    return false;
}
```

### 2. 异常处理
```java
try {
    byte[] result = MPCNative.keygenRound1(handle);
    if (result == null) {
        throw new MPCException("第一轮失败");
    }
} catch (Exception e) {
    Log.e("MPC", "MPC操作异常", e);
} finally {
    MPCNative.keygenDestroy(handle);
}
```

## 性能优化

### 1. 内存管理
- 及时调用`destroy`函数释放资源
- 使用`mpc_string_free`释放C分配的内存
- 避免频繁的JNI调用

### 2. 并发处理
```java
public class AsyncMPCClient {
    private ExecutorService executor = Executors.newCachedThreadPool();
    
    public CompletableFuture<byte[]> performRoundAsync(int round, byte[] data) {
        return CompletableFuture.supplyAsync(() -> {
            // 异步执行MPC轮次
            return MPCNative.keygenRound1(handle);
        }, executor);
    }
}
```

## 安全考虑

### 1. 密钥存储
```java
public class SecureKeyStorage {
    private static final String KEY_ALIAS = "mpc_key";
    
    public void storeKey(byte[] keyData) {
        // 使用Android Keystore安全存储密钥
    }
    
    public byte[] retrieveKey() {
        // 从Android Keystore安全检索密钥
    }
}
```

### 2. 通信安全
- 使用WSS (WebSocket Secure) 进行通信
- 验证服务器证书
- 实现消息完整性检查

## 调试

### 1. 启用日志
```java
public class MPCLogger {
    private static final String TAG = "MPC";
    
    public static void logRound(int round, String operation, byte[] data) {
        Log.d(TAG, String.format("Round %d %s: %d bytes", 
               round, operation, data != null ? data.length : 0));
    }
}
```

### 2. 数据验证
```java
public boolean validateKeyData(byte[] keyData) {
    if (keyData == null || keyData.length == 0) {
        Log.e("MPC", "无效的密钥数据");
        return false;
    }
    // 添加更多验证逻辑
    return true;
}
```

## 示例项目结构

```
android-project/
├── app/
│   ├── libs/
│   │   └── mpc-android-sdk.aar
│   └── src/main/java/
│       └── com/yourpackage/
│           ├── MainActivity.java
│           ├── MPCService.java
│           └── mpc/
│               ├── MPCClient.java
│               ├── MPCWebSocketClient.java
│               └── SecureKeyStorage.java
└── build.gradle
```

这个实现提供了完整的Android MPC SDK，直接调用`libmpc.h`中的函数，无需额外的包装层，性能更好，使用更简单。