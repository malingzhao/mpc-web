#!/bin/bash

# 完整MPC演示运行脚本
# 包含ECDSA签名、Ed25519签名和密钥刷新功能

echo "🚀 开始编译和运行完整MPC演示..."

# 设置Java环境
export JAVA_HOME="/opt/homebrew/opt/openjdk"
export PATH="$JAVA_HOME/bin:$PATH"

# 设置环境变量
export DYLD_LIBRARY_PATH=/Users/malltony/mpc/threshold-lib/sdk2:$DYLD_LIBRARY_PATH

# 检查JNI库是否存在
if [ ! -f "/Users/malltony/mpc/threshold-lib/sdk2/libmpcjni.dylib" ]; then
    echo "❌ 错误：libmpcjni.dylib 不存在"
    echo "请先编译JNI库"
    exit 1
fi

echo "✅ 找到JNI库: libmpcjni.dylib"

# 检查Java环境
echo "📋 检查Java环境..."
$JAVA_HOME/bin/javac -version
$JAVA_HOME/bin/java -version

# 编译Java文件
echo "📦 编译Java文件..."
cd /Users/malltony/mpc/threshold-lib/sdk2

$JAVA_HOME/bin/javac -cp . com/example/mpctest/MPCNative.java
if [ $? -ne 0 ]; then
    echo "❌ MPCNative.java 编译失败"
    exit 1
fi

$JAVA_HOME/bin/javac -cp . com/example/mpctest/CompleteMPCDemo.java
if [ $? -ne 0 ]; then
    echo "❌ CompleteMPCDemo.java 编译失败"
    exit 1
fi

echo "✅ Java文件编译成功"

# 运行演示
echo "🎯 运行完整MPC演示..."
echo "================================"

$JAVA_HOME/bin/java -cp . -Djava.library.path=/Users/malltony/mpc/threshold-lib/sdk2 com.example.mpctest.CompleteMPCDemo

echo "================================"
echo "✅ 演示完成"

echo ""
echo "📋 演示说明："
echo "1. DKG密钥生成 - 生成三方secp256k1密钥"
echo "2. ECDSA签名演示 - 使用生成的密钥进行ECDSA签名"
echo "3. Ed25519签名演示 - 生成Ed25519密钥并进行签名"
echo "4. 密钥刷新演示 - 刷新现有密钥"
echo ""
echo "🔧 技术特点："
echo "- 直接调用C库函数"
echo "- JSON格式消息转换"
echo "- 完整的三轮协议实现"
echo "- 资源自动清理"