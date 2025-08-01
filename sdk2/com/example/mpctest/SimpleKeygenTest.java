package com.example.mpctest;

/**
 * 简单的MPC密钥生成测试
 * 基于test_corrected_keygen.c的逻辑
 * 先跑通基本流程
 */
public class SimpleKeygenTest {
    
    // 配置常量
    private static final int CURVE_SECP256K1 = 0;
    private static final int CURVE_ED25519 = 1;
    private static final int THRESHOLD = 2;
    private static final int TOTAL_PARTIES = 3;
    
    static {
        try {
            System.loadLibrary("mpc");
            System.out.println("✅ MPC库加载成功");
        } catch (UnsatisfiedLinkError e) {
            System.err.println("❌ MPC库加载失败: " + e.getMessage());
            System.err.println("请确保libmpc.so在库路径中");
        }
    }
    
    public static void main(String[] args) {
        System.out.println("🔐 开始简单的MPC密钥生成测试");
        System.out.println("========================================");
        
        SimpleKeygenTest test = new SimpleKeygenTest();
        
        // 测试Ed25519
        System.out.println("\n=== 测试Ed25519密钥生成 ===");
        boolean success = test.testKeygen(CURVE_ED25519);
        if (success) {
            System.out.println("🎊 Ed25519测试成功！");
        } else {
            System.out.println("💥 Ed25519测试失败！");
        }
        
        // 测试secp256k1
        System.out.println("\n=== 测试secp256k1密钥生成 ===");
        success = test.testKeygen(CURVE_SECP256K1);
        if (success) {
            System.out.println("🎊 secp256k1测试成功！");
        } else {
            System.out.println("💥 secp256k1测试失败！");
        }
    }
    
    public boolean testKeygen(int curve) {
        long[] handles = new long[3];
        byte[][] round1Outputs = new byte[3][];
        byte[][] round2Outputs = new byte[3][];
        byte[][] finalKeys = new byte[3][];
        
        try {
            // 步骤1：初始化三个参与方
            System.out.println("📋 步骤1：初始化参与方");
            for (int i = 0; i < 3; i++) {
                int partyId = i + 1;
                handles[i] = MPCNative.keygenInit(curve, partyId, THRESHOLD, TOTAL_PARTIES);
                
                if (handles[i] == 0) {
                    System.err.println("❌ 参与方" + partyId + "初始化失败");
                    return false;
                }
                System.out.println("   ✅ 参与方" + partyId + "初始化成功");
            }
            
            // 步骤2：执行第一轮
            System.out.println("📋 步骤2：执行第一轮");
            for (int i = 0; i < 3; i++) {
                round1Outputs[i] = MPCNative.keygenRound1(handles[i]);
                if (round1Outputs[i] == null) {
                    System.err.println("❌ 参与方" + (i+1) + "第一轮失败");
                    return false;
                }
                System.out.println("   ✅ 参与方" + (i+1) + "第一轮完成，长度: " + round1Outputs[i].length);
            }
            
            // 步骤3：执行第二轮
            System.out.println("📋 步骤3：执行第二轮");
            for (int i = 0; i < 3; i++) {
                int partyId = i + 1;
                
                // 简单的消息格式转换
                byte[] messages = convertToMessages(round1Outputs, partyId);
                if (messages == null) {
                    System.err.println("❌ 参与方" + partyId + "消息转换失败");
                    return false;
                }
                
                round2Outputs[i] = MPCNative.keygenRound2(handles[i], messages);
                if (round2Outputs[i] == null) {
                    System.err.println("❌ 参与方" + partyId + "第二轮失败");
                    return false;
                }
                System.out.println("   ✅ 参与方" + partyId + "第二轮完成，长度: " + round2Outputs[i].length);
            }
            
            // 步骤4：执行第三轮
            System.out.println("📋 步骤4：执行第三轮");
            for (int i = 0; i < 3; i++) {
                int partyId = i + 1;
                
                // 转换第二轮消息
                byte[] messages = convertToMessages(round2Outputs, partyId);
                if (messages == null) {
                    System.err.println("❌ 参与方" + partyId + "第二轮消息转换失败");
                    return false;
                }
                
                // 添加第三轮输入调试信息
                System.out.println("🔍 参与方" + partyId + "第三轮输入长度: " + messages.length);
                String inputPreview = new String(messages, java.nio.charset.StandardCharsets.UTF_8);
                System.out.println("🔍 参与方" + partyId + "第三轮输入预览: " + inputPreview.substring(0, Math.min(200, inputPreview.length())) + "...");
                
                finalKeys[i] = MPCNative.keygenRound3(handles[i], messages);
                if (finalKeys[i] == null) {
                    System.err.println("❌ 参与方" + partyId + "第三轮失败");
                    return false;
                }
                System.out.println("   ✅ 参与方" + partyId + "第三轮完成，密钥长度: " + finalKeys[i].length);
            }
            
            // 显示结果
            System.out.println("📋 最终结果:");
            for (int i = 0; i < 3; i++) {
                System.out.println("   参与方" + (i+1) + "密钥长度: " + finalKeys[i].length);
                
                // 显示前32字节的十六进制
                StringBuilder hex = new StringBuilder();
                int maxBytes = Math.min(finalKeys[i].length, 32);
                for (int j = 0; j < maxBytes; j++) {
                    hex.append(String.format("%02x", finalKeys[i][j] & 0xFF));
                }
                if (finalKeys[i].length > 32) hex.append("...");
                System.out.println("   密钥预览: " + hex.toString());
            }
            
            return true;
            
        } catch (Exception e) {
            System.err.println("❌ 测试过程中发生异常: " + e.getMessage());
            e.printStackTrace();
            return false;
        } finally {
            // 清理资源
            for (int i = 0; i < 3; i++) {
                if (handles[i] != 0) {
                    MPCNative.keygenDestroy(handles[i]);
                }
            }
        }
    }
    
    /**
     * 简单的消息格式转换
     * 参考C版本的逻辑，但简化实现
     */
    private byte[] convertToMessages(byte[][] roundOutputs, int targetParty) {
        try {
            StringBuilder result = new StringBuilder("[");
            int messageCount = 0;
            
            for (int i = 0; i < roundOutputs.length; i++) {
                int fromParty = i + 1;
                if (fromParty == targetParty) continue; // 跳过自己
                
                String output = new String(roundOutputs[i], "UTF-8");
                
                // 查找目标参与方的消息
                String targetKey = "\"" + targetParty + "\":";
                int targetPos = output.indexOf(targetKey);
                if (targetPos == -1) continue;
                
                // 找到消息的开始和结束位置
                int msgStart = output.indexOf('{', targetPos);
                if (msgStart == -1) continue;
                
                int braceCount = 0;
                int msgEnd = msgStart;
                while (msgEnd < output.length()) {
                    char c = output.charAt(msgEnd);
                    if (c == '{') braceCount++;
                    else if (c == '}') braceCount--;
                    msgEnd++;
                    if (braceCount == 0) break;
                }
                
                if (braceCount != 0) continue;
                
                if (messageCount > 0) {
                    result.append(",");
                }
                
                result.append(output.substring(msgStart, msgEnd));
                messageCount++;
            }
            
            result.append("]");
            
            System.out.println("   🔄 为参与方" + targetParty + "转换消息，长度: " + result.length());
            
            return result.toString().getBytes("UTF-8");
            
        } catch (Exception e) {
            System.err.println("消息转换异常: " + e.getMessage());
            return null;
        }
    }
}