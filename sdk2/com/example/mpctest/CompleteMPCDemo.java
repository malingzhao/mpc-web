package com.example.mpctest;

import java.util.Arrays;

/**
 * 完整的MPC演示程序
 * 包含ECDSA签名、Ed25519签名和密钥刷新功能
 */
public class CompleteMPCDemo {
    
    // 常量定义
    private static final int SECP256K1 = 0;
    private static final int ED25519 = 1;
    private static final int THRESHOLD = 2;
    private static final int TOTAL_PARTIES = 3;
    
    public static void main(String[] args) {
        System.out.println("🚀 开始完整MPC演示...\n");
        
        try {
            // 1. 生成DKG密钥
            System.out.println("=== 第一步：DKG密钥生成 ===");
            byte[][] dkgKeys = generateDKGKeys();
            
            // 2. ECDSA签名演示
            System.out.println("\n=== 第二步：ECDSA签名演示 ===");
            demonstrateECDSASign(dkgKeys);
            
            // 3. Ed25519签名演示
            System.out.println("\n=== 第三步：Ed25519签名演示 ===");
            demonstrateEd25519Sign();
            
            // 4. 密钥刷新演示
            System.out.println("\n=== 第四步：密钥刷新演示 ===");
            demonstrateKeyRefresh(dkgKeys);
            
            System.out.println("\n✅ 所有演示完成！");
            
        } catch (Exception e) {
            System.err.println("❌ 演示过程中发生错误: " + e.getMessage());
            e.printStackTrace();
        }
    }
    
    /**
     * 生成DKG密钥（用于ECDSA）
     */
    private static byte[][] generateDKGKeys() {
        System.out.println("正在生成secp256k1 DKG密钥...");
        
        long[] handles = new long[TOTAL_PARTIES];
        byte[][] keys = new byte[TOTAL_PARTIES][];
        
        try {
            // 初始化所有参与方
            for (int i = 0; i < TOTAL_PARTIES; i++) {
                handles[i] = MPCNative.keygenInit(SECP256K1, i + 1, THRESHOLD, TOTAL_PARTIES);
                if (handles[i] == 0) {
                    throw new RuntimeException("参与方 " + (i + 1) + " 初始化失败");
                }
            }
            
            // 第一轮
            byte[][] round1Outputs = new byte[TOTAL_PARTIES][];
            for (int i = 0; i < TOTAL_PARTIES; i++) {
                round1Outputs[i] = MPCNative.keygenRound1(handles[i]);
                if (round1Outputs[i] == null) {
                    throw new RuntimeException("参与方 " + (i + 1) + " 第一轮失败");
                }
            }
            
            // 第二轮
            byte[][] round2Outputs = new byte[TOTAL_PARTIES][];
            for (int i = 0; i < TOTAL_PARTIES; i++) {
                byte[] round2Input = convertRound1ToMessages(round1Outputs, i + 1);
                round2Outputs[i] = MPCNative.keygenRound2(handles[i], round2Input);
                if (round2Outputs[i] == null) {
                    throw new RuntimeException("参与方 " + (i + 1) + " 第二轮失败");
                }
            }
            
            // 第三轮
            for (int i = 0; i < TOTAL_PARTIES; i++) {
                byte[] round3Input = convertRound2ToMessages(round2Outputs, i + 1);
                keys[i] = MPCNative.keygenRound3(handles[i], round3Input);
                if (keys[i] == null) {
                    throw new RuntimeException("参与方 " + (i + 1) + " 第三轮失败");
                }
            }
            
            System.out.println("✅ DKG密钥生成成功");
            for (int i = 0; i < TOTAL_PARTIES; i++) {
                System.out.println("参与方 " + (i + 1) + " 密钥长度: " + keys[i].length + " 字节");
            }
            
            return keys;
            
        } finally {
            // 清理资源
            for (int i = 0; i < TOTAL_PARTIES; i++) {
                if (handles[i] != 0) {
                    MPCNative.keygenDestroy(handles[i]);
                }
            }
        }
    }
    
    /**
     * ECDSA签名演示
     */
    private static void demonstrateECDSASign(byte[][] dkgKeys) {
        System.out.println("正在演示ECDSA签名...");
        
        // 使用P1和P2进行签名
        byte[] p1DkgKey = dkgKeys[0];
        byte[] p2DkgKey = dkgKeys[1];
        
        System.out.println("P1 DKG密钥长度: " + p1DkgKey.length + " 字节");
        System.out.println("P2 DKG密钥长度: " + p2DkgKey.length + " 字节");
        
        // 第一步：ECDSA Keygen
        System.out.println("\n=== ECDSA Keygen 阶段 ===");
        byte[] p1SignData = null;
        byte[] p2SignData = null;
        
        try {
            // 生成P2参数
            System.out.println("1. 生成P2参数...");
            byte[] p2Params = MPCNative.ecdsaKeygenGenerateP2Params();
            System.out.println("   ✅ P2参数生成成功，长度: " + p2Params.length);
            
            // P1执行ECDSA keygen
            System.out.println("2. P1执行ECDSA keygen...");
            byte[][] p1KeygenResult = MPCNative.ecdsaKeygenP1(p1DkgKey, 2, p2Params);
            p1SignData = p1KeygenResult[0];  // 签名数据
            byte[] p1MessageData = p1KeygenResult[1];  // 消息数据
            System.out.println("   ✅ P1 ECDSA keygen成功，签名数据长度: " + p1SignData.length + ", 消息数据长度: " + p1MessageData.length);
            
            // P2执行ECDSA keygen
            System.out.println("3. P2执行ECDSA keygen...");
            p2SignData = MPCNative.ecdsaKeygenP2(p2DkgKey, 1, p1MessageData, p2Params);
            System.out.println("   ✅ P2 ECDSA keygen成功，签名数据长度: " + p2SignData.length);
            
            System.out.println("✅ ECDSA Keygen完成");
            
        } catch (Exception e) {
            throw new RuntimeException("ECDSA Keygen失败: " + e.getMessage(), e);
        }
        
        // 第二步：ECDSA签名
        System.out.println("\n=== ECDSA 签名阶段 ===");
        String message = "Hello, ECDSA MPC!";
        System.out.println("待签名消息: \"" + message + "\"");
        
        // 将消息转换为十六进制字符串（与C代码保持一致）
        String hexMessage = stringToHex(message);
        byte[] messageBytes = hexMessage.getBytes();
        System.out.println("十六进制消息: " + hexMessage);
        
        try {
            // 初始化P1签名
            System.out.println("1. 初始化P1签名...");
            long p1Handle = MPCNative.ecdsaSignInitP1Complex(1, 2, p1SignData, messageBytes);
            System.out.println("   ✅ P1签名初始化成功，句柄: " + p1Handle);
            
            // 初始化P2签名
            System.out.println("2. 初始化P2签名...");
            long p2Handle = MPCNative.ecdsaSignInitP2Complex(2, 1, p2SignData, messageBytes);
            System.out.println("   ✅ P2签名初始化成功，句柄: " + p2Handle);
            
            // P1 Step1: 生成承诺
            System.out.println("3. P1 Step1: 生成承诺...");
            byte[] p1CommitData = MPCNative.ecdsaSignStep1(p1Handle);
            System.out.println("   ✅ P1 Step1成功，承诺数据长度: " + p1CommitData.length);
            
            // P2 Step1: 处理承诺并生成证明
            System.out.println("4. P2 Step1: 处理承诺并生成证明...");
            byte[][] p2Step1Result = MPCNative.ecdsaSignP2Step1(p2Handle, p1CommitData);
            byte[] p2ProofData = p2Step1Result[0];
            byte[] p2R2Data = p2Step1Result[1];
            System.out.println("   ✅ P2 Step1成功，证明数据长度: " + p2ProofData.length + ", R2数据长度: " + p2R2Data.length);
            
            // P1 Step2: 处理P2的证明
            System.out.println("5. P1 Step2: 处理P2的证明...");
            byte[][] p1Step2Result = MPCNative.ecdsaSignP1Step2(p1Handle, p2ProofData, p2R2Data);
            byte[] p1ProofData = p1Step2Result[0];
            byte[] p1CmtdData = p1Step2Result[1];
            System.out.println("   ✅ P1 Step2成功，P1证明数据长度: " + p1ProofData.length + ", 承诺D数据长度: " + p1CmtdData.length);
            
            // P2 Step2: 处理P1的证明
            System.out.println("6. P2 Step2: 处理P1的证明...");
            byte[][] p2Step2Result = MPCNative.ecdsaSignP2Step2(p2Handle, p1CmtdData, p1ProofData);
            byte[] p2EkData = p2Step2Result[0];
            byte[] p2AffineProofData = p2Step2Result[1];
            System.out.println("   ✅ P2 Step2成功，EK数据长度: " + p2EkData.length + ", 仿射证明数据长度: " + p2AffineProofData.length);
            
            // P1 Step3: 生成最终签名
            System.out.println("7. P1 Step3: 生成最终签名...");
            String[] signature = MPCNative.ecdsaSignP1Step3(p1Handle, p2EkData, p2AffineProofData);
            System.out.println("   ✅ P1 Step3成功，生成签名!");
            System.out.println("   📝 签名R: " + signature[0]);
            System.out.println("   📝 签名S: " + signature[1]);
            
            System.out.println("✅ ECDSA签名完成");
            
            // 清理签名资源
            MPCNative.ecdsaSignDestroy(p1Handle);
            MPCNative.ecdsaSignDestroy(p2Handle);
            
        } catch (Exception e) {
            System.err.println("❌ ECDSA签名过程中发生错误: " + e.getMessage());
            e.printStackTrace();
        }
    }
    
    /**
     * Ed25519签名演示
     */
    private static void demonstrateEd25519Sign() {
        System.out.println("正在演示Ed25519签名...");
        
        // 首先生成Ed25519 DKG密钥
        long[] handles = new long[TOTAL_PARTIES];
        byte[][] keys = new byte[TOTAL_PARTIES][];
        
        try {
            // 初始化所有参与方
            for (int i = 0; i < TOTAL_PARTIES; i++) {
                handles[i] = MPCNative.keygenInit(ED25519, i + 1, THRESHOLD, TOTAL_PARTIES);
                if (handles[i] == 0) {
                    throw new RuntimeException("Ed25519参与方 " + (i + 1) + " 初始化失败");
                }
            }
            
            // 第一轮
            byte[][] round1Outputs = new byte[TOTAL_PARTIES][];
            for (int i = 0; i < TOTAL_PARTIES; i++) {
                round1Outputs[i] = MPCNative.keygenRound1(handles[i]);
                if (round1Outputs[i] == null) {
                    throw new RuntimeException("Ed25519参与方 " + (i + 1) + " 第一轮失败");
                }
            }
            
            // 第二轮
            byte[][] round2Outputs = new byte[TOTAL_PARTIES][];
            for (int i = 0; i < TOTAL_PARTIES; i++) {
                byte[] round2Input = convertRound1ToMessages(round1Outputs, i + 1);
                round2Outputs[i] = MPCNative.keygenRound2(handles[i], round2Input);
                if (round2Outputs[i] == null) {
                    throw new RuntimeException("Ed25519参与方 " + (i + 1) + " 第二轮失败");
                }
            }
            
            // 第三轮
            for (int i = 0; i < TOTAL_PARTIES; i++) {
                byte[] round3Input = convertRound2ToMessages(round2Outputs, i + 1);
                keys[i] = MPCNative.keygenRound3(handles[i], round3Input);
                if (keys[i] == null) {
                    throw new RuntimeException("Ed25519参与方 " + (i + 1) + " 第三轮失败");
                }
            }
            
            System.out.println("✅ Ed25519 DKG密钥生成成功");
            
            // 现在进行Ed25519签名（使用P1和P2）
            demonstrateEd25519SignWithKeys(keys[0], keys[1]);
            
        } finally {
            // 清理资源
            for (int i = 0; i < TOTAL_PARTIES; i++) {
                if (handles[i] != 0) {
                    MPCNative.keygenDestroy(handles[i]);
                }
            }
        }
    }
    
    /**
     * 使用生成的密钥进行Ed25519签名
     */
    private static void demonstrateEd25519SignWithKeys(byte[] p1Key, byte[] p2Key) {
        System.out.println("正在进行Ed25519签名...");
        
        String message = "Hello, Ed25519 MPC!";
        System.out.println("待签名消息: \"" + message + "\"");
        
        // 将消息转换为十六进制字符串（与C代码保持一致）
        String hexMessage = stringToHex(message);
        byte[] messageBytes = hexMessage.getBytes();
        System.out.println("十六进制消息: " + hexMessage);
        
        int[] partList = {1, 2}; // P1和P2参与签名
        
        long p1Handle = 0, p2Handle = 0;
        
        try {
            // 初始化签名
            p1Handle = MPCNative.ed25519SignInit(1, THRESHOLD, partList, p1Key, messageBytes);
            p2Handle = MPCNative.ed25519SignInit(2, THRESHOLD, partList, p2Key, messageBytes);
            
            if (p1Handle == 0 || p2Handle == 0) {
                throw new RuntimeException("Ed25519签名初始化失败");
            }
            System.out.println("✅ Ed25519签名初始化成功");
            
            // 第一轮
            byte[] p1Round1 = MPCNative.ed25519SignRound1(p1Handle);
            byte[] p2Round1 = MPCNative.ed25519SignRound1(p2Handle);
            
            if (p1Round1 == null || p2Round1 == null) {
                throw new RuntimeException("Ed25519签名第一轮失败");
            }
            System.out.println("✅ Ed25519签名第一轮完成");
            
            // 第二轮
            byte[] p1Round2Input = convertSignRound1ToMessages(new byte[][]{p1Round1, p2Round1}, 1);
            byte[] p2Round2Input = convertSignRound1ToMessages(new byte[][]{p1Round1, p2Round1}, 2);
            
            byte[] p1Round2 = MPCNative.ed25519SignRound2(p1Handle, p1Round2Input);
            byte[] p2Round2 = MPCNative.ed25519SignRound2(p2Handle, p2Round2Input);
            
            if (p1Round2 == null || p2Round2 == null) {
                throw new RuntimeException("Ed25519签名第二轮失败");
            }
            System.out.println("✅ Ed25519签名第二轮完成");
            
            // 第三轮
            byte[] p1Round3Input = convertSignRound2ToMessages(new byte[][]{p1Round2, p2Round2}, 1);
            byte[] p2Round3Input = convertSignRound2ToMessages(new byte[][]{p1Round2, p2Round2}, 2);
            
            String[] p1Signature = MPCNative.ed25519SignRound3(p1Handle, p1Round3Input);
            String[] p2Signature = MPCNative.ed25519SignRound3(p2Handle, p2Round3Input);
            
            if (p1Signature != null && p1Signature.length == 2) {
                System.out.println("✅ Ed25519签名成功！");
                System.out.println("📝 签名 R: " + p1Signature[0]);
                System.out.println("📝 签名 S: " + p1Signature[1]);
            } else if (p2Signature != null && p2Signature.length == 2) {
                System.out.println("✅ Ed25519签名成功！");
                System.out.println("📝 签名 R: " + p2Signature[0]);
                System.out.println("📝 签名 S: " + p2Signature[1]);
            } else {
                System.out.println("⚠️  Ed25519签名结果为空");
                System.out.println("P1签名结果: " + (p1Signature != null ? Arrays.toString(p1Signature) : "null"));
                System.out.println("P2签名结果: " + (p2Signature != null ? Arrays.toString(p2Signature) : "null"));
            }
            
        } catch (Exception e) {
            System.err.println("❌ Ed25519签名过程中发生错误: " + e.getMessage());
            e.printStackTrace();
        } finally {
            if (p1Handle != 0) MPCNative.ed25519SignDestroy(p1Handle);
            if (p2Handle != 0) MPCNative.ed25519SignDestroy(p2Handle);
        }
    }
    
    /**
     * 密钥刷新演示
     */
    private static void demonstrateKeyRefresh(byte[][] originalKeys) {
        System.out.println("正在演示密钥刷新...");
        
        int[] devoteList = {1, 2, 3}; // 所有参与方都参与刷新
        long[] handles = new long[TOTAL_PARTIES];
        byte[][] newKeys = new byte[TOTAL_PARTIES][];
        
        try {
            // 初始化刷新
            for (int i = 0; i < TOTAL_PARTIES; i++) {
                handles[i] = MPCNative.refreshInit(SECP256K1, i + 1, THRESHOLD, devoteList, originalKeys[i]);
                if (handles[i] == 0) {
                    throw new RuntimeException("参与方 " + (i + 1) + " 刷新初始化失败");
                }
            }
            
            // 第一轮
            byte[][] round1Outputs = new byte[TOTAL_PARTIES][];
            for (int i = 0; i < TOTAL_PARTIES; i++) {
                round1Outputs[i] = MPCNative.refreshRound1(handles[i]);
                if (round1Outputs[i] == null) {
                    throw new RuntimeException("参与方 " + (i + 1) + " 刷新第一轮失败");
                }
            }
            
            // 第二轮
            byte[][] round2Outputs = new byte[TOTAL_PARTIES][];
            for (int i = 0; i < TOTAL_PARTIES; i++) {
                byte[] round2Input = convertRound1ToMessages(round1Outputs, i + 1);
                round2Outputs[i] = MPCNative.refreshRound2(handles[i], round2Input);
                if (round2Outputs[i] == null) {
                    throw new RuntimeException("参与方 " + (i + 1) + " 刷新第二轮失败");
                }
            }
            
            // 第三轮
            for (int i = 0; i < TOTAL_PARTIES; i++) {
                byte[] round3Input = convertRound2ToMessages(round2Outputs, i + 1);
                newKeys[i] = MPCNative.refreshRound3(handles[i], round3Input);
                if (newKeys[i] == null) {
                    throw new RuntimeException("参与方 " + (i + 1) + " 刷新第三轮失败");
                }
            }
            
            System.out.println("✅ 密钥刷新成功！");
            for (int i = 0; i < TOTAL_PARTIES; i++) {
                System.out.println("参与方 " + (i + 1) + " 新密钥长度: " + newKeys[i].length + " 字节");
                System.out.println("原密钥与新密钥是否相同: " + Arrays.equals(originalKeys[i], newKeys[i]));
            }
            
        } finally {
            // 清理资源
            for (int i = 0; i < TOTAL_PARTIES; i++) {
                if (handles[i] != 0) {
                    MPCNative.refreshDestroy(handles[i]);
                }
            }
        }
    }
    
    // ==================== 辅助方法 ====================
    
    /**
     * 将第一轮输出转换为消息格式
     */
    private static byte[] convertRound1ToMessages(byte[][] round1Outputs, int partyId) {
        try {
            // 首先将字节数组转换为字符串（假设是JSON格式）
            String[] jsonOutputs = new String[round1Outputs.length];
            for (int i = 0; i < round1Outputs.length; i++) {
                jsonOutputs[i] = new String(round1Outputs[i], "UTF-8");
            }
            
            // 构建消息数组
            StringBuilder result = new StringBuilder("[");
            int messageCount = 0;
            
            // 遍历每个参与方的输出
            for (int i = 0; i < jsonOutputs.length; i++) {
                int fromParty = i + 1;
                if (fromParty == partyId) continue; // 跳过自己
                
                String output = jsonOutputs[i];
                
                // 查找目标参与方的消息
                String targetKey = "\"" + partyId + "\":";
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
            return result.toString().getBytes("UTF-8");
            
        } catch (Exception e) {
            System.err.println("❌ 消息转换失败: " + e.getMessage());
            return "[]".getBytes();
        }
    }
    
    /**
     * 将第二轮输出转换为消息格式
     */
    private static byte[] convertRound2ToMessages(byte[][] round2Outputs, int partyId) {
        try {
            // 首先将字节数组转换为字符串（假设是JSON格式）
            String[] jsonOutputs = new String[round2Outputs.length];
            for (int i = 0; i < round2Outputs.length; i++) {
                jsonOutputs[i] = new String(round2Outputs[i], "UTF-8");
            }
            
            // 构建消息数组
            StringBuilder result = new StringBuilder("[");
            int messageCount = 0;
            
            // 遍历每个参与方的输出
            for (int i = 0; i < jsonOutputs.length; i++) {
                int fromParty = i + 1;
                if (fromParty == partyId) continue; // 跳过自己
                
                String output = jsonOutputs[i];
                
                // 查找目标参与方的消息
                String targetKey = "\"" + partyId + "\":";
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
            return result.toString().getBytes("UTF-8");
            
        } catch (Exception e) {
            System.err.println("❌ 第二轮消息转换失败: " + e.getMessage());
            return "[]".getBytes();
        }
    }
    
    /**
     * 将签名第一轮输出转换为消息格式
     */
    private static byte[] convertSignRound1ToMessages(byte[][] round1Outputs, int partyId) {
        try {
            // 首先将字节数组转换为字符串（假设是JSON格式）
            String[] jsonOutputs = new String[round1Outputs.length];
            for (int i = 0; i < round1Outputs.length; i++) {
                jsonOutputs[i] = new String(round1Outputs[i], "UTF-8");
            }
            
            // 构建消息数组
            StringBuilder result = new StringBuilder("[");
            int messageCount = 0;
            
            // 遍历每个参与方的输出
            for (int i = 0; i < jsonOutputs.length; i++) {
                int fromParty = i + 1;
                if (fromParty == partyId) continue; // 跳过自己
                
                String output = jsonOutputs[i];
                
                // 查找目标参与方的消息
                String targetKey = "\"" + partyId + "\":";
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
            return result.toString().getBytes("UTF-8");
            
        } catch (Exception e) {
            System.err.println("❌ 签名第一轮消息转换失败: " + e.getMessage());
            return "[]".getBytes();
        }
    }
    
    /**
     * 将签名第二轮输出转换为消息格式
     */
    private static byte[] convertSignRound2ToMessages(byte[][] round2Outputs, int partyId) {
        try {
            // 首先将字节数组转换为字符串（假设是JSON格式）
            String[] jsonOutputs = new String[round2Outputs.length];
            for (int i = 0; i < round2Outputs.length; i++) {
                jsonOutputs[i] = new String(round2Outputs[i], "UTF-8");
            }
            
            // 构建消息数组
            StringBuilder result = new StringBuilder("[");
            int messageCount = 0;
            
            // 遍历每个参与方的输出
            for (int i = 0; i < jsonOutputs.length; i++) {
                int fromParty = i + 1;
                if (fromParty == partyId) continue; // 跳过自己
                
                String output = jsonOutputs[i];
                
                // 查找目标参与方的消息
                String targetKey = "\"" + partyId + "\":";
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
            return result.toString().getBytes("UTF-8");
            
        } catch (Exception e) {
            System.err.println("❌ 签名第二轮消息转换失败: " + e.getMessage());
            return "[]".getBytes();
        }
    }
    
    /**
     * 字节数组转十六进制字符串
     */
    private static String bytesToHex(byte[] bytes) {
        if (bytes == null) return "";
        StringBuilder result = new StringBuilder();
        for (byte b : bytes) {
            result.append(String.format("%02x", b));
        }
        return result.toString();
    }
    
    /**
     * 十六进制字符串转字节数组
     */
    private static byte[] hexStringToByteArray(String s) {
        int len = s.length();
        byte[] data = new byte[len / 2];
        for (int i = 0; i < len; i += 2) {
            data[i / 2] = (byte) ((Character.digit(s.charAt(i), 16) << 4)
                                 + Character.digit(s.charAt(i+1), 16));
        }
        return data;
    }
    
    /**
     * 字符串转十六进制字符串（与C代码中的string_to_hex保持一致）
     */
    private static String stringToHex(String input) {
        if (input == null) return "";
        StringBuilder result = new StringBuilder();
        for (char c : input.toCharArray()) {
            result.append(String.format("%02x", (int) c));
        }
        return result.toString();
    }
}