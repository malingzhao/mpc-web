#!/bin/bash

# Set JAVA_HOME
export JAVA_HOME=/opt/homebrew/opt/openjdk/libexec/openjdk.jdk/Contents/Home

# 简单的Java keygen测试编译和运行脚本

echo "🔧 编译Java keygen测试..."

# 设置路径
SDK_DIR="/Users/malltony/mpc/threshold-lib/sdk2"
JAVA_SRC_DIR="$SDK_DIR/com/example/mpctest"

cd "$SDK_DIR"

# 编译Java文件
echo "编译MPCNative.java..."
javac com/example/mpctest/MPCNative.java

echo "编译SimpleKeygenTest.java..."
javac com/example/mpctest/SimpleKeygenTest.java

if [ $? -eq 0 ]; then
    echo "✅ 编译成功！"
    
    echo ""
    echo "🚀 运行测试..."
    echo "确保libmpc.so在库路径中..."
    
    # 设置库路径
    export LD_LIBRARY_PATH="$SDK_DIR:$LD_LIBRARY_PATH"
    export DYLD_LIBRARY_PATH="$SDK_DIR:$DYLD_LIBRARY_PATH"
    
    # 运行测试
    java -Djava.library.path="$SDK_DIR" com.example.mpctest.SimpleKeygenTest
    
else
    echo "❌ 编译失败！"
    exit 1
fi