#!/bin/bash

# Android MPC密钥生成演示运行脚本

# 设置JAVA_HOME
export JAVA_HOME=/opt/homebrew/opt/openjdk/libexec/openjdk.jdk/Contents/Home

echo "🚀 Android MPC密钥生成演示"
echo "=========================="

# 编译Java文件
echo "📦 编译Java演示..."
javac -cp . com/example/mpctest/AndroidKeygenDemo.java

if [ $? -eq 0 ]; then
    echo "✅ 编译成功！"
    
    # 运行演示
    echo ""
    echo "🎯 运行Android密钥生成演示..."
    java -cp . com.example.mpctest.AndroidKeygenDemo
    
    echo ""
    echo "📋 演示说明："
    echo "1. 这个演示展示了Android MPC密钥生成的完整流程"
    echo "2. 模拟了三方密钥生成的三个轮次"
    echo "3. 包含了Android应用集成指南"
    echo "4. 实际使用时需要编译JNI库并处理网络通信"
    
else
    echo "❌ 编译失败！"
    exit 1
fi