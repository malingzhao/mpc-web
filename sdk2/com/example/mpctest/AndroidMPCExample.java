package com.example.mpctest;

import java.util.Arrays;

/**
 * Android MPC客户端示例
 * 演示如何使用MPCNative进行三方密钥生成和两方签名
 */
public class AndroidMPCExample {
    
    // 配置常量
    private static final int CURVE_SECP256K1 = 0;
    private static final int CURVE_ED25519 = 1;
    private static final int THRESHOLD = 2;
    private static final int TOTAL_PARTIES = 3;
    
    // 参与方ID
    private static final int PARTY_SERVER = 1;
    private static final int PARTY_THIRD_PARTY = 2;
    private static final int PARTY_ANDROID = 3;
    
    private int myPartyID;
    private byte[] keyData;
    
    public AndroidMPCExample(int partyID) {
        this.myPartyID = partyID;
    }
    
    /**
     * 执行三方密钥生成
     * @param curve 曲线类型
     * @return 是否成功
     */
    public boolean performKeyGeneration(int curve) {
        System.out.println("开始三方密钥生成，当前方ID: " + myPartyID);
        
        // 1. 初始化密钥生成
        long handle = MPCNative.keygenInit(curve, myPartyID, THRESHOLD, TOTAL_PARTIES);
        if (handle == 0) {
            System.err.println("密钥生成初始化失败");
            return false;
        }
        
        try {
            // 2. 第一轮
            byte[] round1Data = MPCNative.keygenRound1(handle);
            if (round1Data == null) {
                System.err.println("密钥生成第一轮失败");
                return false;
            }
            System.out.println("第一轮完成，数据长度: " + round1Data.length);
            
            // 模拟接收其他方的数据（实际应用中需要通过网络通信）
            byte[] receivedRound1Data = simulateReceiveData(round1Data);
            
            // 3. 第二轮
            byte[] round2Data = MPCNative.keygenRound2(handle, receivedRound1Data);
            if (round2Data == null) {
                System.err.println("密钥生成第二轮失败");
                return false;
            }
            System.out.println("第二轮完成，数据长度: " + round2Data.length);
            
            // 模拟接收其他方的数据
            byte[] receivedRound2Data = simulateReceiveData(round2Data);
            
            // 4. 第三轮
            keyData = MPCNative.keygenRound3(handle, receivedRound2Data);
            if (keyData == null) {
                System.err.println("密钥生成第三轮失败");
                return false;
            }
            System.out.println("密钥生成完成，密钥长度: " + keyData.length);
            
            return true;
            
        } finally {
            // 5. 清理资源
            MPCNative.keygenDestroy(handle);
        }
    }
    
    /**
     * 执行密钥刷新
     * @param curve 曲线类型
     * @return 是否成功
     */
    public boolean performKeyRefresh(int curve) {
        if (keyData == null) {
            System.err.println("没有可用的密钥数据");
            return false;
        }
        
        System.out.println("开始密钥刷新，当前方ID: " + myPartyID);
        
        // 参与方列表（所有三方都参与刷新）
        int[] devoteList = {PARTY_SERVER, PARTY_THIRD_PARTY, PARTY_ANDROID};
        
        // 1. 初始化密钥刷新
        long handle = MPCNative.refreshInit(curve, myPartyID, THRESHOLD, devoteList, keyData);
        if (handle == 0) {
            System.err.println("密钥刷新初始化失败");
            return false;
        }
        
        try {
            // 2. 第一轮
            byte[] round1Data = MPCNative.refreshRound1(handle);
            if (round1Data == null) {
                System.err.println("密钥刷新第一轮失败");
                return false;
            }
            System.out.println("刷新第一轮完成，数据长度: " + round1Data.length);
            
            // 模拟接收其他方的数据
            byte[] receivedRound1Data = simulateReceiveData(round1Data);
            
            // 3. 第二轮
            byte[] round2Data = MPCNative.refreshRound2(handle, receivedRound1Data);
            if (round2Data == null) {
                System.err.println("密钥刷新第二轮失败");
                return false;
            }
            System.out.println("刷新第二轮完成，数据长度: " + round2Data.length);
            
            // 模拟接收其他方的数据
            byte[] receivedRound2Data = simulateReceiveData(round2Data);
            
            // 4. 第三轮
            byte[] newKeyData = MPCNative.refreshRound3(handle, receivedRound2Data);
            if (newKeyData == null) {
                System.err.println("密钥刷新第三轮失败");
                return false;
            }
            
            // 更新密钥数据
            keyData = newKeyData;
            System.out.println("密钥刷新完成，新密钥长度: " + keyData.length);
            
            return true;
            
        } finally {
            // 5. 清理资源
            MPCNative.refreshDestroy(handle);
        }
    }
    
    /**
     * 执行Ed25519两方签名
     * @param message 待签名消息
     * @return 签名结果 [r, s]
     */
    public String[] performEd25519Signing(byte[] message) {
        if (keyData == null) {
            System.err.println("没有可用的密钥数据");
            return null;
        }
        
        System.out.println("开始Ed25519签名，消息长度: " + message.length);
        
        // 两方签名：服务器和Android应用
        int[] partList = {PARTY_SERVER, PARTY_ANDROID};
        
        // 1. 初始化签名
        long handle = MPCNative.ed25519SignInit(myPartyID, THRESHOLD, partList, keyData, message);
        if (handle == 0) {
            System.err.println("Ed25519签名初始化失败");
            return null;
        }
        
        try {
            // 2. 第一轮
            byte[] round1Data = MPCNative.ed25519SignRound1(handle);
            if (round1Data == null) {
                System.err.println("Ed25519签名第一轮失败");
                return null;
            }
            System.out.println("签名第一轮完成，数据长度: " + round1Data.length);
            
            // 模拟接收对方的数据
            byte[] receivedRound1Data = simulateReceiveData(round1Data);
            
            // 3. 第二轮
            byte[] round2Data = MPCNative.ed25519SignRound2(handle, receivedRound1Data);
            if (round2Data == null) {
                System.err.println("Ed25519签名第二轮失败");
                return null;
            }
            System.out.println("签名第二轮完成，数据长度: " + round2Data.length);
            
            // 模拟接收对方的数据
            byte[] receivedRound2Data = simulateReceiveData(round2Data);
            
            // 4. 第三轮
            String[] signature = MPCNative.ed25519SignRound3(handle, receivedRound2Data);
            if (signature == null || signature.length != 2) {
                System.err.println("Ed25519签名第三轮失败");
                return null;
            }
            
            System.out.println("Ed25519签名完成:");
            System.out.println("R: " + signature[0]);
            System.out.println("S: " + signature[1]);
            
            return signature;
            
        } finally {
            // 5. 清理资源
            MPCNative.ed25519SignDestroy(handle);
        }
    }
    
    /**
     * 执行ECDSA两方签名
     * @param message 待签名消息
     * @param peerID 对方ID
     * @return 是否成功
     */
    public boolean performECDSASigning(byte[] message, int peerID) {
        if (keyData == null) {
            System.err.println("没有可用的密钥数据");
            return false;
        }
        
        System.out.println("开始ECDSA签名，对方ID: " + peerID);
        
        // 1. 初始化签名
        long handle = MPCNative.ecdsaSignInitComplex(myPartyID, peerID, keyData, message);
        if (handle == 0) {
            System.err.println("ECDSA签名初始化失败");
            return false;
        }
        
        try {
            // 2. 第一步
            byte[] commitData = MPCNative.ecdsaSignStep1(handle);
            if (commitData == null) {
                System.err.println("ECDSA签名第一步失败");
                return false;
            }
            System.out.println("ECDSA签名第一步完成，承诺数据长度: " + commitData.length);
            
            // 注意：ECDSA签名需要更多步骤，这里只演示第一步
            // 实际应用中需要实现完整的ECDSA签名流程
            
            return true;
            
        } finally {
            // 3. 清理资源
            MPCNative.ecdsaSignDestroy(handle);
        }
    }
    
    /**
     * 模拟接收其他方的数据
     * 实际应用中应该通过WebSocket或其他网络通信方式接收
     */
    private byte[] simulateReceiveData(byte[] sentData) {
        // 这里只是简单返回相同的数据作为示例
        // 实际应用中需要实现真正的网络通信
        return Arrays.copyOf(sentData, sentData.length);
    }
    
    /**
     * 获取当前密钥数据
     */
    public byte[] getKeyData() {
        return keyData;
    }
    
    /**
     * 设置密钥数据
     */
    public void setKeyData(byte[] keyData) {
        this.keyData = keyData;
    }
    
    /**
     * 示例主函数
     */
    public static void main(String[] args) {
        // 创建Android客户端实例
        AndroidMPCExample client = new AndroidMPCExample(PARTY_ANDROID);
        
        try {
            // 1. 执行三方密钥生成
            System.out.println("=== 开始三方密钥生成 ===");
            boolean keygenSuccess = client.performKeyGeneration(CURVE_ED25519);
            if (!keygenSuccess) {
                System.err.println("密钥生成失败");
                return;
            }
            
            // 2. 执行密钥刷新
            System.out.println("\n=== 开始密钥刷新 ===");
            boolean refreshSuccess = client.performKeyRefresh(CURVE_ED25519);
            if (!refreshSuccess) {
                System.err.println("密钥刷新失败");
                return;
            }
            
            // 3. 执行Ed25519签名
            System.out.println("\n=== 开始Ed25519签名 ===");
            String message = "Hello, MPC World!";
            String[] signature = client.performEd25519Signing(message.getBytes());
            if (signature == null) {
                System.err.println("Ed25519签名失败");
                return;
            }
            
            // 4. 执行ECDSA签名
            System.out.println("\n=== 开始ECDSA签名 ===");
            boolean ecdsaSuccess = client.performECDSASigning(message.getBytes(), PARTY_SERVER);
            if (!ecdsaSuccess) {
                System.err.println("ECDSA签名失败");
                return;
            }
            
            System.out.println("\n=== 所有MPC操作完成 ===");
            
        } catch (Exception e) {
            System.err.println("MPC操作异常: " + e.getMessage());
            e.printStackTrace();
        }
    }
}