package com.example.mpctest;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import android.util.Log;

/**
 * MPC密钥生成Demo - Java版本
 * 参考test_corrected_keygen.c的逻辑实现
 * 适用于Android平台的三方密钥生成
 */
public class KeygenDemo {
    
    private static final String TAG = "KeygenDemo";
    
    // 配置常量
    private static final int CURVE_SECP256K1 = 0;
    private static final int CURVE_ED25519 = 1;
    private static final int THRESHOLD = 2;
    private static final int TOTAL_PARTIES = 3;
    
    // 参与方ID
    private static final int PARTY_SERVER = 1;
    private static final int PARTY_THIRD_PARTY = 2;
    private static final int PARTY_ANDROID = 3;
    
    // 会话句柄
    private long[] handles = new long[3];
    
    // 轮次数据存储
    private byte[][] round1Outputs = new byte[3][];
    private byte[][] round2Outputs = new byte[3][];
    private byte[][] finalKeys = new byte[3][];
    
    // 线程池用于异步操作
    private ExecutorService executor = Executors.newCachedThreadPool();
    
    /**
     * 执行完整的三方密钥生成流程
     * @param curve 曲线类型 (0=secp256k1, 1=ed25519)
     * @return 是否成功
     */
    public boolean performKeyGeneration(int curve) {
        Log.i(TAG, "🔐 开始MPC密钥生成");
        Log.i(TAG, "目标: 使用正确的消息格式完成三轮密钥生成");
        Log.i(TAG, "========================================");
        
        try {
            // 第一步：初始化参与方
            if (!initializeParties(curve)) {
                Log.e(TAG, "❌ 参与方初始化失败");
                return false;
            }
            
            // 第二步：执行第一轮
            if (!executeRound1()) {
                Log.e(TAG, "❌ 第一轮执行失败");
                return false;
            }
            
            // 第三步：执行第二轮
            if (!executeRound2()) {
                Log.e(TAG, "❌ 第二轮执行失败");
                return false;
            }
            
            // 第四步：执行第三轮
            if (!executeRound3()) {
                Log.e(TAG, "❌ 第三轮执行失败");
                return false;
            }
            
            // 第五步：显示结果
            displayResults();
            
            Log.i(TAG, "🎊 密钥生成成功完成！");
            return true;
            
        } catch (Exception e) {
            Log.e(TAG, "💥 密钥生成过程中发生异常", e);
            return false;
        } finally {
            cleanup();
        }
    }
    
    /**
     * 初始化所有参与方
     */
    private boolean initializeParties(int curve) {
        Log.i(TAG, "📋 第一步：初始化参与方");
        
        for (int i = 0; i < 3; i++) {
            int partyId = i + 1;
            handles[i] = MPCNative.keygenInit(curve, partyId, THRESHOLD, TOTAL_PARTIES);
            
            if (handles[i] == 0) {
                String error = MPCNative.getErrorString(-1);
                Log.e(TAG, "❌ 参与方" + partyId + "初始化失败: " + error);
                return false;
            }
            
            Log.i(TAG, "   ✅ 参与方" + partyId + "初始化成功");
        }
        
        return true;
    }
    
    /**
     * 执行第一轮密钥生成
     */
    private boolean executeRound1() {
        Log.i(TAG, "📋 第二步：执行第一轮密钥生成");
        
        for (int i = 0; i < 3; i++) {
            round1Outputs[i] = MPCNative.keygenRound1(handles[i]);
            
            if (round1Outputs[i] == null) {
                Log.e(TAG, "❌ 参与方" + (i+1) + "第一轮失败");
                return false;
            }
            
            Log.i(TAG, "   ✅ 参与方" + (i+1) + "第一轮完成，输出长度: " + round1Outputs[i].length);
        }
        
        return true;
    }
    
    /**
     * 执行第二轮密钥生成
     */
    private boolean executeRound2() {
        Log.i(TAG, "📋 第三步：转换消息格式并执行第二轮");
        
        for (int i = 0; i < 3; i++) {
            int partyId = i + 1;
            
            // 为当前参与方转换消息
            byte[] messagesForParty = convertRound1ToMessages(round1Outputs, partyId);
            if (messagesForParty == null) {
                Log.e(TAG, "❌ 参与方" + partyId + "消息转换失败");
                return false;
            }
            
            round2Outputs[i] = MPCNative.keygenRound2(handles[i], messagesForParty);
            
            if (round2Outputs[i] == null) {
                Log.e(TAG, "❌ 参与方" + partyId + "第二轮失败");
                return false;
            }
            
            Log.i(TAG, "   ✅ 参与方" + partyId + "第二轮完成，输出长度: " + round2Outputs[i].length);
        }
        
        return true;
    }
    
    /**
     * 执行第三轮密钥生成
     */
    private boolean executeRound3() {
        Log.i(TAG, "📋 第四步：执行第三轮密钥生成");
        
        for (int i = 0; i < 3; i++) {
            int partyId = i + 1;
            
            // 为当前参与方转换第二轮消息
            byte[] messagesForParty = convertRound1ToMessages(round2Outputs, partyId);
            if (messagesForParty == null) {
                Log.e(TAG, "❌ 参与方" + partyId + "第二轮消息转换失败");
                return false;
            }
            
            finalKeys[i] = MPCNative.keygenRound3(handles[i], messagesForParty);
            
            if (finalKeys[i] == null) {
                Log.e(TAG, "❌ 参与方" + partyId + "第三轮失败");
                return false;
            }
            
            Log.i(TAG, "   ✅ 参与方" + partyId + "第三轮完成，密钥长度: " + finalKeys[i].length);
        }
        
        return true;
    }
    
    /**
     * 转换轮次输出为消息数组格式
     * 参考C版本的convert_round1_to_messages函数
     */
    private byte[] convertRound1ToMessages(byte[][] roundOutputs, int targetParty) {
        try {
            StringBuilder result = new StringBuilder("[");
            int messageCount = 0;
            
            // 遍历每个参与方的输出
            for (int i = 0; i < roundOutputs.length; i++) {
                int fromParty = i + 1;
                if (fromParty == targetParty) continue; // 跳过自己
                
                String output = new String(roundOutputs[i], "UTF-8");
                
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
            
            Log.d(TAG, "   🔄 为参与方" + targetParty + "转换的消息数组: " + result.toString());
            
            return result.toString().getBytes("UTF-8");
            
        } catch (Exception e) {
            Log.e(TAG, "消息转换异常", e);
            return null;
        }
    }
    
    /**
     * 显示最终结果
     */
    private void displayResults() {
        Log.i(TAG, "🎊 密钥生成成功完成！");
        Log.i(TAG, "📋 最终私钥分片:");
        
        for (int i = 0; i < 3; i++) {
            Log.i(TAG, "");
            Log.i(TAG, "参与方" + (i+1) + "的私钥分片:");
            Log.i(TAG, "   长度: " + finalKeys[i].length);
            
            // 显示内容预览
            try {
                String preview = new String(finalKeys[i], "UTF-8");
                if (preview.length() > 200) {
                    preview = preview.substring(0, 200) + "...";
                }
                Log.i(TAG, "   内容预览: " + preview);
            } catch (Exception e) {
                Log.w(TAG, "无法显示内容预览", e);
            }
            
            // 显示十六进制格式
            StringBuilder hex = new StringBuilder();
            int maxBytes = Math.min(finalKeys[i].length, 64);
            for (int j = 0; j < maxBytes; j++) {
                hex.append(String.format("%02x", finalKeys[i][j] & 0xFF));
            }
            if (finalKeys[i].length > 64) {
                hex.append("...");
            }
            Log.i(TAG, "   十六进制 (前64字节): " + hex.toString());
        }
    }
    
    /**
     * 清理资源
     */
    private void cleanup() {
        Log.d(TAG, "清理资源...");
        
        for (int i = 0; i < 3; i++) {
            if (handles[i] != 0) {
                MPCNative.keygenDestroy(handles[i]);
                handles[i] = 0;
            }
        }
        
        if (executor != null && !executor.isShutdown()) {
            executor.shutdown();
            try {
                if (!executor.awaitTermination(5, TimeUnit.SECONDS)) {
                    executor.shutdownNow();
                }
            } catch (InterruptedException e) {
                executor.shutdownNow();
                Thread.currentThread().interrupt();
            }
        }
    }
    
    /**
     * 获取指定参与方的密钥数据
     * @param partyId 参与方ID (1-3)
     * @return 密钥数据，如果无效则返回null
     */
    public byte[] getKeyData(int partyId) {
        if (partyId < 1 || partyId > 3) {
            Log.e(TAG, "无效的参与方ID: " + partyId);
            return null;
        }
        
        int index = partyId - 1;
        if (finalKeys[index] == null) {
            Log.w(TAG, "参与方" + partyId + "的密钥数据不存在");
            return null;
        }
        
        // 返回副本以避免外部修改
        byte[] copy = new byte[finalKeys[index].length];
        System.arraycopy(finalKeys[index], 0, copy, 0, finalKeys[index].length);
        return copy;
    }
    
    /**
     * 异步执行密钥生成
     * @param curve 曲线类型
     * @return CompletableFuture，包含执行结果
     */
    public CompletableFuture<Boolean> performKeyGenerationAsync(int curve) {
        return CompletableFuture.supplyAsync(() -> {
            return performKeyGeneration(curve);
        }, executor);
    }
    
    /**
     * 检查是否已完成密钥生成
     * @return 是否已生成密钥
     */
    public boolean isKeyGenerated() {
        for (byte[] key : finalKeys) {
            if (key == null) {
                return false;
            }
        }
        return true;
    }
    
    /**
     * 主函数 - 用于测试
     */
    public static void main(String[] args) {
        // 加载本地库
        try {
            System.loadLibrary("mpc");
            System.loadLibrary("mpcjni");
        } catch (UnsatisfiedLinkError e) {
            System.err.println("无法加载本地库: " + e.getMessage());
            System.err.println("请确保libmpc.so和libmpcjni.so在库路径中");
            return;
        }
        
        KeygenDemo demo = new KeygenDemo();
        
        // 测试Ed25519曲线
        System.out.println("=== 测试Ed25519密钥生成 ===");
        boolean success = demo.performKeyGeneration(CURVE_ED25519);
        
        if (success) {
            System.out.println("🎊 Ed25519密钥生成测试成功！");
            
            // 显示每个参与方的密钥
            for (int i = 1; i <= 3; i++) {
                byte[] keyData = demo.getKeyData(i);
                if (keyData != null) {
                    System.out.println("参与方" + i + "密钥长度: " + keyData.length);
                }
            }
        } else {
            System.err.println("💥 Ed25519密钥生成测试失败！");
        }
        
        // 测试secp256k1曲线
        System.out.println("\n=== 测试secp256k1密钥生成 ===");
        KeygenDemo demo2 = new KeygenDemo();
        success = demo2.performKeyGeneration(CURVE_SECP256K1);
        
        if (success) {
            System.out.println("🎊 secp256k1密钥生成测试成功！");
        } else {
            System.err.println("💥 secp256k1密钥生成测试失败！");
        }
    }
}