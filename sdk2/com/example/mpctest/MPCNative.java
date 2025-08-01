package com.example.mpctest;

/**
 * Android JNI接口，直接调用libmpc.h中的MPC函数
 * 支持三方密钥生成、密钥刷新和两方签名
 */
public class MPCNative {
    
    static {
        try {
            System.loadLibrary("mpcjni");
            System.out.println("✅ MPC JNI库加载成功");
        } catch (UnsatisfiedLinkError e) {
            System.err.println("❌ MPC JNI库加载失败: " + e.getMessage());
            throw e;
        }
    }
    
    // ==================== 密钥生成 (Key Generation) ====================
    
    /**
     * 初始化密钥生成
     * @param curve 曲线类型 (0=secp256k1, 1=ed25519)
     * @param partyID 当前方ID
     * @param threshold 阈值
     * @param totalParties 总参与方数量
     * @return 会话句柄指针
     */
    public static native long keygenInit(int curve, int partyID, int threshold, int totalParties);
    
    /**
     * 密钥生成第一轮
     * @param handle 会话句柄
     * @return 输出数据
     */
    public static native byte[] keygenRound1(long handle);
    
    /**
     * 密钥生成第二轮
     * @param handle 会话句柄
     * @param inData 输入数据
     * @return 输出数据
     */
    public static native byte[] keygenRound2(long handle, byte[] inData);
    
    /**
     * 密钥生成第三轮
     * @param handle 会话句柄
     * @param inData 输入数据
     * @return 生成的密钥数据
     */
    public static native byte[] keygenRound3(long handle, byte[] inData);
    
    /**
     * 销毁密钥生成会话
     * @param handle 会话句柄
     */
    public static native void keygenDestroy(long handle);
    
    // ==================== 密钥刷新 (Key Refresh) ====================
    
    /**
     * 初始化密钥刷新
     * @param curve 曲线类型
     * @param partyID 当前方ID
     * @param threshold 阈值
     * @param devoteList 参与方列表
     * @param keyData 现有密钥数据
     * @return 会话句柄指针
     */
    public static native long refreshInit(int curve, int partyID, int threshold, int[] devoteList, byte[] keyData);
    
    /**
     * 密钥刷新第一轮
     * @param handle 会话句柄
     * @return 输出数据
     */
    public static native byte[] refreshRound1(long handle);
    
    /**
     * 密钥刷新第二轮
     * @param handle 会话句柄
     * @param inData 输入数据
     * @return 输出数据
     */
    public static native byte[] refreshRound2(long handle, byte[] inData);
    
    /**
     * 密钥刷新第三轮
     * @param handle 会话句柄
     * @param inData 输入数据
     * @return 刷新后的密钥数据
     */
    public static native byte[] refreshRound3(long handle, byte[] inData);
    
    /**
     * 销毁密钥刷新会话
     * @param handle 会话句柄
     */
    public static native void refreshDestroy(long handle);
    
    // ==================== Ed25519签名 ====================
    
    /**
     * 初始化Ed25519签名
     * @param partyID 当前方ID
     * @param threshold 阈值
     * @param partList 参与方列表
     * @param keyData 密钥数据
     * @param message 待签名消息
     * @return 会话句柄指针
     */
    public static native long ed25519SignInit(int partyID, int threshold, int[] partList, byte[] keyData, byte[] message);
    
    /**
     * Ed25519签名第一轮
     * @param handle 会话句柄
     * @return 输出数据
     */
    public static native byte[] ed25519SignRound1(long handle);
    
    /**
     * Ed25519签名第二轮
     * @param handle 会话句柄
     * @param inData 输入数据
     * @return 输出数据
     */
    public static native byte[] ed25519SignRound2(long handle, byte[] inData);
    
    /**
     * Ed25519签名第三轮
     * @param handle 会话句柄
     * @param inData 输入数据
     * @return 签名结果 [r, s]
     */
    public static native String[] ed25519SignRound3(long handle, byte[] inData);
    
    /**
     * 销毁Ed25519签名会话
     * @param handle 会话句柄
     */
    public static native void ed25519SignDestroy(long handle);
    
    // ==================== ECDSA Keygen ====================
    
    /**
     * 生成ECDSA P2参数
     * @return P2参数数据
     */
    public static native byte[] ecdsaKeygenGenerateP2Params();
    
    /**
     * ECDSA密钥生成 P1方
     * @param keyData 密钥数据
     * @param peerId 对方ID
     * @param p2Params P2参数
     * @return [signData, messageData] P1签名数据和消息数据
     */
    public static native byte[][] ecdsaKeygenP1(byte[] keyData, int peerId, byte[] p2Params);
    
    /**
     * ECDSA密钥生成 P2方
     * @param keyData 密钥数据
     * @param p1Id P1方ID
     * @param p1Message P1消息
     * @param p2Params P2参数
     * @return 生成结果
     */
    public static native byte[] ecdsaKeygenP2(byte[] keyData, int p1Id, byte[] p1Message, byte[] p2Params);
    
    // ==================== ECDSA签名 ====================
    
    /**
     * 初始化ECDSA签名 P1 (复杂版本)
     * @param partyID 当前方ID
     * @param peerID 对方ID
     * @param keyData 密钥数据
     * @param message 待签名消息
     * @return 会话句柄指针
     */
    public static native long ecdsaSignInitP1Complex(int partyID, int peerID, byte[] keyData, byte[] message);
    
    /**
     * 初始化ECDSA签名 P2 (复杂版本)
     * @param partyID 当前方ID
     * @param peerID 对方ID
     * @param keyData 密钥数据
     * @param message 待签名消息
     * @return 会话句柄指针
     */
    public static native long ecdsaSignInitP2Complex(int partyID, int peerID, byte[] keyData, byte[] message);
    
    /**
     * ECDSA签名步骤1 (P1)
     * @param handle 会话句柄
     * @return 承诺数据
     */
    public static native byte[] ecdsaSignStep1(long handle);
    
    /**
     * ECDSA签名 P2 步骤1
     * @param handle 会话句柄
     * @param commitData 承诺数据
     * @return [proofData, r2Data]
     */
    public static native byte[][] ecdsaSignP2Step1(long handle, byte[] commitData);
    
    /**
     * ECDSA签名 P1 步骤2
     * @param handle 会话句柄
     * @param proofData 证明数据
     * @param r2Data R2数据
     * @return [p1ProofData, cmtDData]
     */
    public static native byte[][] ecdsaSignP1Step2(long handle, byte[] proofData, byte[] r2Data);
    
    /**
     * ECDSA签名 P2 步骤2
     * @param handle 会话句柄
     * @param cmtDData 承诺D数据
     * @param p1ProofData P1证明数据
     * @return [ekData, affineProofData]
     */
    public static native byte[][] ecdsaSignP2Step2(long handle, byte[] cmtDData, byte[] p1ProofData);
    
    /**
     * ECDSA签名 P1 步骤3 (最终签名)
     * @param handle 会话句柄
     * @param ekData EK数据
     * @param affineProofData 仿射证明数据
     * @return [r, s] 签名结果
     */
    public static native String[] ecdsaSignP1Step3(long handle, byte[] ekData, byte[] affineProofData);
    
    /**
     * 销毁ECDSA签名会话
     * @param handle 会话句柄
     */
    public static native void ecdsaSignDestroy(long handle);
    
    // ==================== 辅助函数 ====================
    
    /**
     * 获取错误信息
     * @param errorCode 错误码
     * @return 错误描述
     */
    public static native String getErrorString(int errorCode);
    
    /**
     * 分配字符串内存
     * @param src 源字符串
     * @return 分配的字符串指针
     */
    public static native long allocString(String src);
    
    /**
     * 释放字符串内存
     * @param ptr 字符串指针
     */
    public static native void freeString(long ptr);
}