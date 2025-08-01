package com.example.mpctest;

/**
 * 真正的MPC密钥生成测试 - 直接调用C库
 * 基于test_corrected_keygen.c的逻辑
 */
public class RealKeygenTest {
    
    // 曲线类型常量
    public static final int CURVE_SECP256K1 = 0;
    public static final int CURVE_ED25519 = 1;
    
    /**
     * 执行真正的三方密钥生成测试
     */
    public static void testRealKeygen() {
        System.out.println("🔐 开始真正的MPC密钥生成测试");
        System.out.println("========================================");
        
        // 测试Ed25519密钥生成
        System.out.println("\n=== 测试Ed25519密钥生成 ===");
        testKeygenForCurve(CURVE_ED25519, "Ed25519");
        
        // 测试secp256k1密钥生成
        System.out.println("\n=== 测试secp256k1密钥生成 ===");
        testKeygenForCurve(CURVE_SECP256K1, "secp256k1");
    }
    
    /**
     * 测试指定曲线的密钥生成
     */
    private static void testKeygenForCurve(int curve, String curveName) {
        try {
            System.out.println("📋 步骤1：初始化参与方");
            
            // 初始化三个参与方
            long[] handles = new long[3];
            for (int i = 0; i < 3; i++) {
                handles[i] = MPCNative.keygenInit(curve, i + 1, 2, 3);
                if (handles[i] == 0) {
                    throw new RuntimeException("参与方 " + (i + 1) + " 初始化失败");
                }
                System.out.println("  ✅ 参与方 " + (i + 1) + " 初始化完成 (handle: " + handles[i] + ")");
            }
            
            // 第一轮密钥生成
            System.out.println("\n🔄 步骤2：执行第一轮密钥生成");
            byte[][] round1Messages = new byte[3][];
            for (int i = 0; i < 3; i++) {
                round1Messages[i] = MPCNative.keygenRound1(handles[i]);
                if (round1Messages[i] == null) {
                    throw new RuntimeException("参与方 " + (i + 1) + " 第一轮失败");
                }
                System.out.println("  📤 参与方 " + (i + 1) + " 生成第一轮消息 (长度: " + round1Messages[i].length + ")");
            }
            
            // 转换第一轮消息格式
            System.out.println("\n🔄 步骤3：转换消息格式并执行第二轮");
            byte[][] round2Messages = new byte[3][];
            for (int i = 0; i < 3; i++) {
                int partyId = i + 1; // 参与方ID是1-based
                byte[] combinedMessages = convertRound1ToMessages(round1Messages, partyId);
                round2Messages[i] = MPCNative.keygenRound2(handles[i], combinedMessages);
                if (round2Messages[i] == null) {
                    throw new RuntimeException("参与方 " + partyId + " 第二轮失败");
                }
                System.out.println("  📤 参与方 " + partyId + " 生成第二轮消息 (长度: " + round2Messages[i].length + ")");
            }
            
            // 第三轮密钥生成
            System.out.println("\n🔄 步骤4：执行第三轮密钥生成");
            byte[][] finalKeys = new byte[3][];
            for (int i = 0; i < 3; i++) {
                int partyId = i + 1; // 参与方ID是1-based
                byte[] combinedMessages = convertRound2ToMessages(round2Messages, partyId);
                finalKeys[i] = MPCNative.keygenRound3(handles[i], combinedMessages);
                if (finalKeys[i] == null) {
                    throw new RuntimeException("参与方 " + partyId + " 第三轮失败");
                }
                System.out.println("  🔑 参与方 " + partyId + " 生成最终密钥份额 (长度: " + finalKeys[i].length + ")");
            }
            
            // 显示结果
            System.out.println("\n✅ " + curveName + " 密钥生成完成！");
            System.out.println("📊 结果摘要:");
            for (int i = 0; i < 3; i++) {
                String keyHex = bytesToHex(finalKeys[i], 32); // 显示前32字节
                System.out.println("  参与方 " + (i + 1) + " 密钥份额: " + keyHex + "...");
            }
            
            // 清理资源
            System.out.println("\n🧹 清理资源...");
            for (int i = 0; i < 3; i++) {
                MPCNative.keygenDestroy(handles[i]);
                System.out.println("  ✅ 参与方 " + (i + 1) + " 资源已清理");
            }
            
        } catch (Exception e) {
            System.err.println("❌ " + curveName + " 密钥生成失败: " + e.getMessage());
            e.printStackTrace();
        }
    }
    
    /**
     * 转换第一轮消息格式 (基于test_corrected_keygen.c的逻辑)
     * 将字节数组转换为JSON消息数组格式
     */
    private static byte[] convertRound1ToMessages(byte[][] round1Messages, int targetParty) {
        try {
            // 首先将字节数组转换为字符串（假设是JSON格式）
            String[] jsonOutputs = new String[round1Messages.length];
            for (int i = 0; i < round1Messages.length; i++) {
                jsonOutputs[i] = new String(round1Messages[i], "UTF-8");
            }
            
            // 构建消息数组
            StringBuilder result = new StringBuilder("[");
            int messageCount = 0;
            
            // 遍历每个参与方的输出
            for (int i = 0; i < jsonOutputs.length; i++) {
                int fromParty = i + 1;
                if (fromParty == targetParty) continue; // 跳过自己
                
                String output = jsonOutputs[i];
                
                // 查找目标参与方的消息
                String targetKey = "\"" + targetParty + "\":";
                int targetPos = output.indexOf(targetKey);
                if (targetPos == -1) continue;
                
                // 找到消息的开始位置
                int msgStart = output.indexOf('{', targetPos);
                if (msgStart == -1) continue;
                
                // 找到消息的结束位置（匹配大括号）
                int braceCount = 0;
                int msgEnd = msgStart;
                while (msgEnd < output.length()) {
                    char c = output.charAt(msgEnd);
                    if (c == '{') braceCount++;
                    else if (c == '}') braceCount--;
                    msgEnd++;
                    if (braceCount == 0) break;
                }
                
                if (braceCount != 0) continue; // 格式错误
                
                // 添加逗号分隔符
                if (messageCount > 0) {
                    result.append(",");
                }
                
                // 添加消息
                result.append(output.substring(msgStart, msgEnd));
                messageCount++;
            }
            
            result.append("]");
            
            String resultStr = result.toString();
            System.out.println("   🔄 为参与方" + targetParty + "转换的消息数组: " + 
                (resultStr.length() > 200 ? resultStr.substring(0, 200) + "..." : resultStr));
            
            return resultStr.getBytes("UTF-8");
            
        } catch (Exception e) {
            System.err.println("❌ 消息转换失败: " + e.getMessage());
            return "[]".getBytes();
        }
    }
    
    /**
     * 转换第二轮消息格式 (基于test_corrected_keygen.c的逻辑)
     * 将字节数组转换为JSON消息数组格式
     */
    private static byte[] convertRound2ToMessages(byte[][] round2Messages, int targetParty) {
        try {
            // 首先将字节数组转换为字符串（假设是JSON格式）
            String[] jsonOutputs = new String[round2Messages.length];
            for (int i = 0; i < round2Messages.length; i++) {
                jsonOutputs[i] = new String(round2Messages[i], "UTF-8");
            }
            
            // 构建消息数组
            StringBuilder result = new StringBuilder("[");
            int messageCount = 0;
            
            // 遍历每个参与方的输出
            for (int i = 0; i < jsonOutputs.length; i++) {
                int fromParty = i + 1;
                if (fromParty == targetParty) continue; // 跳过自己
                
                String output = jsonOutputs[i];
                
                // 查找目标参与方的消息
                String targetKey = "\"" + targetParty + "\":";
                int targetPos = output.indexOf(targetKey);
                if (targetPos == -1) continue;
                
                // 找到消息的开始位置
                int msgStart = output.indexOf('{', targetPos);
                if (msgStart == -1) continue;
                
                // 找到消息的结束位置（匹配大括号）
                int braceCount = 0;
                int msgEnd = msgStart;
                while (msgEnd < output.length()) {
                    char c = output.charAt(msgEnd);
                    if (c == '{') braceCount++;
                    else if (c == '}') braceCount--;
                    msgEnd++;
                    if (braceCount == 0) break;
                }
                
                if (braceCount != 0) continue; // 格式错误
                
                // 添加逗号分隔符
                if (messageCount > 0) {
                    result.append(",");
                }
                
                // 添加消息
                result.append(output.substring(msgStart, msgEnd));
                messageCount++;
            }
            
            result.append("]");
            
            String resultStr = result.toString();
            System.out.println("   🔄 为参与方" + targetParty + "转换的第二轮消息数组: " + 
                (resultStr.length() > 200 ? resultStr.substring(0, 200) + "..." : resultStr));
            
            return resultStr.getBytes("UTF-8");
            
        } catch (Exception e) {
            System.err.println("❌ 第二轮消息转换失败: " + e.getMessage());
            return "[]".getBytes();
        }
    }
    
    /**
     * 将字节数组转换为十六进制字符串
     */
    private static String bytesToHex(byte[] bytes, int maxLength) {
        if (bytes == null) return "null";
        
        int length = Math.min(bytes.length, maxLength);
        StringBuilder hex = new StringBuilder();
        for (int i = 0; i < length; i++) {
            hex.append(String.format("%02x", bytes[i] & 0xFF));
        }
        return hex.toString();
    }
    
    /**
     * 主函数
     */
    public static void main(String[] args) {
        System.out.println("🚀 启动真正的MPC密钥生成测试");
        System.out.println("=====================================");
        
        try {
            testRealKeygen();
            System.out.println("\n🎉 所有测试完成！");
        } catch (Exception e) {
            System.err.println("\n💥 测试过程中发生错误: " + e.getMessage());
            e.printStackTrace();
        }
    }
}