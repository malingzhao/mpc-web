package com.example.distributed;

import java.net.URI;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.ConcurrentHashMap;
import java.util.Map;
import java.util.List;
import java.util.ArrayList;
import java.util.Base64;
import java.io.IOException;
import java.nio.ByteBuffer;

import org.java_websocket.client.WebSocketClient;
import org.java_websocket.handshake.ServerHandshake;
import com.google.gson.Gson;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import com.example.mpctest.MPCNative;

/**
 * Java分布式MPC客户端
 * 通过WebSocket与Go客户端协作完成密钥生成和签名
 */
public class DistributedMPCClient {
    
    // 客户端配置
    private final String clientId;
    private final String serverUrl;
    private final int partyId;
    private final int threshold;
    private final int totalParties;
    
    // WebSocket连接
    private WebSocketClient webSocketClient;
    private boolean connected = false;
    
    // MPC状态
    private String sessionId;
    private long keygenHandle = 0;
    private byte[] dkgKey;
    private Map<String, Object> messageBuffer = new ConcurrentHashMap<>();
    
    // 同步工具
    private CountDownLatch connectionLatch = new CountDownLatch(1);
    private CountDownLatch keygenLatch = new CountDownLatch(1);
    private CountDownLatch signLatch = new CountDownLatch(1);
    
    // JSON处理
    private Gson gson = new Gson();
    
    public DistributedMPCClient(String clientId, String serverUrl, int partyId, int threshold, int totalParties) {
        this.clientId = clientId;
        this.serverUrl = serverUrl;
        this.partyId = partyId;
        this.threshold = threshold;
        this.totalParties = totalParties;
    }
    
    /**
     * 连接到协调服务器
     */
    public boolean connect() {
        try {
            URI serverUri = new URI(serverUrl + "?client_id=" + clientId);
            
            webSocketClient = new WebSocketClient(serverUri) {
                @Override
                public void onOpen(ServerHandshake handshake) {
                    System.out.println("🔗 Java客户端 " + clientId + " 已连接到服务器");
                    connected = true;
                    connectionLatch.countDown();
                }
                
                @Override
                public void onMessage(String message) {
                    handleMessage(message);
                }
                
                @Override
                public void onClose(int code, String reason, boolean remote) {
                    System.out.println("🔌 Java客户端 " + clientId + " 连接已关闭: " + reason);
                    connected = false;
                }
                
                @Override
                public void onError(Exception ex) {
                    System.err.println("❌ Java客户端 " + clientId + " WebSocket错误: " + ex.getMessage());
                }
            };
            
            webSocketClient.connect();
            
            // 等待连接建立
            return connectionLatch.await(10, TimeUnit.SECONDS);
            
        } catch (Exception e) {
            System.err.println("❌ 连接失败: " + e.getMessage());
            return false;
        }
    }
    
    /**
     * 处理收到的消息
     */
    private void handleMessage(String message) {
        try {
            JsonObject jsonMessage = JsonParser.parseString(message).getAsJsonObject();
            String type = jsonMessage.get("type").getAsString();
            
            System.out.println("📨 Java客户端 " + clientId + " 收到消息: " + type);
            
            switch (type) {
                case "session_created":
                    handleSessionCreated(jsonMessage);
                    break;
                case "start_keygen":
                    handleStartKeygen(jsonMessage);
                    break;
                case "keygen_round1":
                    handleKeygenRound1Data(jsonMessage);
                    break;
                case "keygen_round2":
                    handleKeygenRound2Data(jsonMessage);
                    break;
                case "keygen_complete":
                    handleKeygenComplete(jsonMessage);
                    break;
                default:
                    System.out.println("⚠️ 未知消息类型: " + type);
            }
        } catch (Exception e) {
            System.err.println("❌ 处理消息失败: " + e.getMessage());
            e.printStackTrace();
        }
    }
    
    /**
     * 发送消息到服务器
     */
    private void sendMessage(JsonObject message) {
        if (connected && webSocketClient != null) {
            webSocketClient.send(gson.toJson(message));
        }
    }
    
    /**
     * 启动密钥生成
     */
    public boolean startKeygen() {
        try {
            System.out.println("🔑 Java客户端 " + clientId + " 开始密钥生成...");
            
            // 请求创建会话
            JsonObject request = new JsonObject();
            request.addProperty("type", "create_session");
            request.addProperty("client_id", clientId);
            request.addProperty("session_type", "keygen");
            request.addProperty("threshold", threshold);
            request.addProperty("total_parties", totalParties);
            
            sendMessage(request);
            
            // 等待密钥生成完成
            return keygenLatch.await(60, TimeUnit.SECONDS);
            
        } catch (Exception e) {
            System.err.println("❌ 密钥生成失败: " + e.getMessage());
            return false;
        }
    }
    
    /**
     * 处理会话创建
     */
    private void handleSessionCreated(JsonObject message) {
        sessionId = message.get("session_id").getAsString();
        System.out.println("📋 会话已创建: " + sessionId + "，等待协调服务器开始信号");
        
        // 初始化密钥生成但不立即开始
        try {
            keygenHandle = MPCNative.keygenInit(0, partyId, threshold, totalParties); // 0 = SECP256K1
            if (keygenHandle == 0) {
                throw new RuntimeException("密钥生成初始化失败");
            }
            System.out.println("⏳ Java客户端 " + clientId + " 等待协调服务器发送开始密钥生成信号");
        } catch (Exception e) {
            System.err.println("❌ 密钥生成初始化失败: " + e.getMessage());
        }
    }
    
    /**
     * 处理开始密钥生成信号
     */
    private void handleStartKeygen(JsonObject message) {
        System.out.println("🚀 Java客户端 " + clientId + " 收到开始密钥生成信号，开始第一轮");
        
        try {
            // 执行第一轮
            byte[] round1Output = MPCNative.keygenRound1(keygenHandle);
            if (round1Output == null) {
                throw new RuntimeException("第一轮密钥生成失败");
            }
            
            // 发送第一轮数据
            JsonObject response = new JsonObject();
            response.addProperty("type", "keygen_round1");
            response.addProperty("session_id", sessionId);
            response.addProperty("from_party", partyId);
            response.addProperty("data", Base64.getEncoder().encodeToString(round1Output));
            
            sendMessage(response);
            
        } catch (Exception e) {
            System.err.println("❌ 处理开始密钥生成失败: " + e.getMessage());
        }
    }
    
    /**
     * 处理第一轮密钥生成数据
     */
    private void handleKeygenRound1Data(JsonObject message) {
        try {
            int fromParty = message.get("from_party").getAsInt();
            String data = message.get("data").getAsString();
            
            System.out.println("📨 Java客户端 " + clientId + " 收到来自参与方 " + fromParty + " 的第一轮数据");
            
            // 存储其他参与方的第一轮数据
            messageBuffer.put("round1_" + fromParty, Base64.getDecoder().decode(data));
            
            // 检查是否收集到所有第一轮数据
            if (hasAllRound1Data()) {
                System.out.println("✅ Java客户端 " + clientId + " 已收集到所有第一轮数据，开始第二轮");
                executeRound2();
            }
            
        } catch (Exception e) {
            System.err.println("❌ 处理第一轮数据失败: " + e.getMessage());
            e.printStackTrace();
        }
    }
    
    /**
     * 检查是否收集到所有第一轮数据
     */
    private boolean hasAllRound1Data() {
        int expectedCount = totalParties - 1; // 除了自己
        int actualCount = 0;
        
        for (String key : messageBuffer.keySet()) {
            if (key.startsWith("round1_")) {
                actualCount++;
            }
        }
        
        return actualCount >= expectedCount;
    }
    
    /**
     * 执行第二轮密钥生成
     */
    private void executeRound2() {
        try {
            // 构造第二轮输入
            byte[] round2Input = constructRound2Input();
            
            // 执行第二轮
            byte[] round2Output = MPCNative.keygenRound2(keygenHandle, round2Input);
            if (round2Output == null) {
                throw new RuntimeException("第二轮密钥生成失败");
            }
            
            // 保存自己的第二轮输出
            messageBuffer.put("round2_" + partyId, round2Output);
            System.out.println("💾 Java客户端 " + clientId + " 保存自己的第二轮数据，长度: " + round2Output.length);
            
            // 发送第二轮数据
            JsonObject response = new JsonObject();
            response.addProperty("type", "keygen_round2");
            response.addProperty("session_id", sessionId);
            response.addProperty("from_party", partyId);
            response.addProperty("data", Base64.getEncoder().encodeToString(round2Output));

            sendMessage(response);
            
            // 检查是否收集到所有第二轮数据（包括自己的）
            if (hasAllRound2Data()) {
                System.out.println("✅ Java客户端 " + clientId + " 已收集到所有第二轮数据，开始第三轮");
                executeRound3();
            }
            
        } catch (Exception e) {
            System.err.println("❌ 执行第二轮失败: " + e.getMessage());
        }
    }
    
    /**
     * 构造第二轮输入数据
     */
    private byte[] constructRound2Input() {
        try {
            StringBuilder result = new StringBuilder("[");
            int messageCount = 0;
            
            for (int i = 1; i <= totalParties; i++) {
                if (i != partyId) {
                    byte[] data = (byte[]) messageBuffer.get("round1_" + i);
                    if (data != null) {
                        String output = new String(data, "UTF-8");
                        
                        // 查找目标参与方的消息
                        String targetKey = "\"" + partyId + "\":";
                        int targetPos = output.indexOf(targetKey);
                        if (targetPos != -1) {
                            // 找到消息的开始和结束位置
                            int msgStart = output.indexOf('{', targetPos);
                            if (msgStart != -1) {
                                int braceCount = 0;
                                int msgEnd = msgStart;
                                while (msgEnd < output.length()) {
                                    char c = output.charAt(msgEnd);
                                    if (c == '{') braceCount++;
                                    else if (c == '}') braceCount--;
                                    msgEnd++;
                                    if (braceCount == 0) break;
                                }
                                
                                if (braceCount == 0) {
                                    if (messageCount > 0) {
                                        result.append(",");
                                    }
                                    result.append(output.substring(msgStart, msgEnd));
                                    messageCount++;
                                }
                            }
                        }
                    }
                }
            }
            
            result.append("]");
            
            System.out.println("🔄 Java客户端 " + clientId + " 为第二轮构造消息，长度: " + result.length());
            
            return result.toString().getBytes("UTF-8");
            
        } catch (Exception e) {
            System.err.println("❌ 构造第二轮输入失败: " + e.getMessage());
            return null;
        }
    }
    
    /**
     * 处理第二轮密钥生成数据
     */
    private void handleKeygenRound2Data(JsonObject message) {
        try {
            int fromParty = message.get("from_party").getAsInt();
            String data = message.get("data").getAsString();
            
            System.out.println("📨 Java客户端 " + clientId + " 收到来自参与方 " + fromParty + " 的第二轮数据");
            
            // 解码Base64数据
            byte[] decodedData = Base64.getDecoder().decode(data);
            String dataStr = new String(decodedData, "UTF-8");
            
            // 存储第二轮输出数据，用于后续转换
            String messageKey = "round2_output_" + fromParty;
            messageBuffer.put(messageKey, dataStr);
            
            System.out.println("✅ 存储来自参与方" + fromParty + "的第二轮输出数据");
            System.out.println("   数据长度: " + dataStr.length());
            System.out.println("   数据预览: " + dataStr.substring(0, Math.min(100, dataStr.length())) + "...");
            
            System.out.println("📊 第二轮消息收集状态: " + countRound2Messages() + "/" + (totalParties - 1));
            
            // 检查是否收集到所有第二轮数据
            if (hasAllRound2Data()) {
                System.out.println("✅ Java客户端 " + clientId + " 已收集到所有第二轮数据，开始第三轮");
                executeRound3();
            }
            
        } catch (Exception e) {
            System.err.println("❌ 处理第二轮数据失败: " + e.getMessage());
            e.printStackTrace();
        }
    }
    
    /**
     * 检查是否收集到所有第二轮数据
     */
    private boolean hasAllRound2Data() {
        int expectedCount = totalParties - 1;  // 不包括自己的数据
        int actualCount = countRound2Messages();
        
        System.out.println("🔍 检查第二轮数据收集状态: " + actualCount + "/" + expectedCount);
        return actualCount >= expectedCount;
    }
    
    /**
     * 统计收集到的第二轮消息数量
     */
    private int countRound2Messages() {
        int count = 0;
        for (String key : messageBuffer.keySet()) {
            if (key.startsWith("round2_output_")) {
                count++;
            }
        }
        return count;
    }
    
    /**
     * 执行第三轮密钥生成
     */
    private void executeRound3() {
        try {
            System.out.println("🔄 开始执行第三轮密钥生成");
            
            // 构造第三轮输入
            byte[] round3Input = constructRound3Input();
            
            if (round3Input == null) {
                System.err.println("❌ 第三轮输入构造失败，无法继续");
                throw new RuntimeException("第三轮输入构造失败");
            }
            
            System.out.println("✅ 第三轮输入构造成功，长度: " + round3Input.length);
            
            // 添加详细的调试信息
            String inputContent = new String(round3Input, "UTF-8");
            System.out.println("📋 第三轮输入详细信息:");
            System.out.println("   长度: " + round3Input.length + " 字节");
            System.out.println("   内容预览: " + inputContent);
            System.out.println("   KeygenHandle: " + keygenHandle);
            System.out.println("   PartyId: " + partyId);
            System.out.println("   Threshold: " + threshold);
            System.out.println("   TotalParties: " + totalParties);
            
            System.out.println("🔄 调用MPCNative.keygenRound3...");
        
        // 执行第三轮
        dkgKey = MPCNative.keygenRound3(keygenHandle, round3Input);
        
        if (dkgKey == null) {
            System.err.println("❌ MPCNative.keygenRound3返回null");
            System.err.println("   这可能是由于以下原因:");
            System.err.println("   1. 第三轮输入格式不正确");
            System.err.println("   2. KeygenHandle状态异常");
            System.err.println("   3. 底层MPC库错误");
            System.err.println("   完整的第三轮输入内容:");
            System.err.println(inputContent);
            throw new RuntimeException("第三轮密钥生成失败 - JNI调用返回null");
        }
            
            System.out.println("✅ Java客户端 " + clientId + " 密钥生成完成，密钥长度: " + dkgKey.length);
            
            // 通知密钥生成完成
            JsonObject response = new JsonObject();
            response.addProperty("type", "keygen_complete");
            response.addProperty("session_id", sessionId);
            response.addProperty("from_party", partyId);
            response.addProperty("success", true);
            
            sendMessage(response);
            keygenLatch.countDown();
            
        } catch (Exception e) {
            System.err.println("❌ 执行第三轮失败: " + e.getMessage());
            e.printStackTrace();
            keygenLatch.countDown();
        }
    }
    
    /**
     * 构造第三轮输入数据 - 从第二轮输出中提取发送给当前参与方的消息
     * 按照SDK的方式构造消息数组
     */
    private byte[] constructRound3Input() {
        try {
            System.out.println("🔍 开始构造第三轮输入，当前PartyID: " + partyId);
            
            // 构造消息数组，只包含发送给当前参与方的消息
            StringBuilder result = new StringBuilder("[");
            int messageCount = 0;
            
            // 遍历所有第二轮输出数据，提取发送给当前参与方的消息
            for (String key : messageBuffer.keySet()) {
                if (key.startsWith("round2_output_")) {
                    // 提取发送方的PartyID
                    String fromPartyStr = key.substring("round2_output_".length());
                    int fromParty = Integer.parseInt(fromPartyStr);
                    
                    // 跳过自己发送给自己的消息
                    if (fromParty == partyId) {
                        System.out.println("⏭️ 跳过自己(参与方" + fromParty + ")发送给自己的消息");
                        continue;
                    }
                    
                    String outputData = (String) messageBuffer.get(key);
                    
                    try {
                        // 查找目标参与方的消息，使用与SimpleKeygenTest相同的逻辑
                        String targetKey = "\"" + partyId + "\":";
                        int targetPos = outputData.indexOf(targetKey);
                        if (targetPos == -1) continue;
                        
                        // 找到消息的开始和结束位置
                        int msgStart = outputData.indexOf('{', targetPos);
                        if (msgStart == -1) continue;
                        
                        int braceCount = 0;
                        int msgEnd = msgStart;
                        while (msgEnd < outputData.length()) {
                            char c = outputData.charAt(msgEnd);
                            if (c == '{') braceCount++;
                            else if (c == '}') braceCount--;
                            msgEnd++;
                            if (braceCount == 0) break;
                        }
                        
                        if (braceCount != 0) continue;
                        
                        if (messageCount > 0) {
                            result.append(",");
                        }
                        
                        String messageData = outputData.substring(msgStart, msgEnd);
                        result.append(messageData);
                        messageCount++;
                        
                        System.out.println("✅ 从" + key + "中提取到发送给参与方" + partyId + "的消息");
                        System.out.println("   消息长度: " + messageData.length());
                        System.out.println("   消息预览: " + messageData.substring(0, Math.min(200, messageData.length())) + "...");
                        
                    } catch (Exception e) {
                        System.err.println("❌ 解析第二轮输出数据失败: " + key + ", 错误: " + e.getMessage());
                        System.err.println("   数据内容: " + outputData.substring(0, Math.min(100, outputData.length())) + "...");
                    }
                }
            }
            
            result.append("]");
            
            System.out.println("🔍 最终构造的第三轮输入:");
            System.out.println("   消息数量: " + messageCount);
            System.out.println("   总长度: " + result.length());
            System.out.println("   内容预览: " + result.toString().substring(0, Math.min(300, result.length())) + "...");
            
            if (messageCount == 0) {
                System.err.println("❌ 未找到有效的第二轮消息");
                return null;
            }
            
            return result.toString().getBytes("UTF-8");
            
        } catch (Exception e) {
            System.err.println("❌ 构造第三轮输入失败: " + e.getMessage());
            e.printStackTrace();
            return null;
        }
    }
    
    /**
     * 处理密钥生成完成
     */
    private void handleKeygenComplete(JsonObject message) {
        System.out.println("🎉 所有参与方密钥生成完成");
    }
    
    /**
     * 处理签名请求
     */
    private void handleSignRequest(JsonObject message) {
        // TODO: 实现签名逻辑
        System.out.println("📝 收到签名请求");
    }
    
    /**
     * 处理签名完成
     */
    private void handleSignComplete(JsonObject message) {
        System.out.println("✍️ 签名完成");
        signLatch.countDown();
    }
    
    /**
     * 断开连接
     */
    public void disconnect() {
        if (webSocketClient != null) {
            webSocketClient.close();
        }
        
        // 清理MPC资源
        if (keygenHandle != 0) {
            MPCNative.keygenDestroy(keygenHandle);
            keygenHandle = 0;
        }
    }
    
    /**
     * 获取DKG密钥
     */
    public byte[] getDkgKey() {
        return dkgKey;
    }
    
    /**
     * 主函数 - 测试入口
     */
    public static void main(String[] args) {
        if (args.length < 3) {
            System.out.println("用法: java DistributedMPCClient <client_id> <server_url> <party_id>");
            System.exit(1);
        }
        
        String clientId = args[0];
        String serverUrl = args[1];
        int partyId = Integer.parseInt(args[2]);
        
        DistributedMPCClient client = new DistributedMPCClient(
            clientId, serverUrl, partyId, 2, 3  // threshold=2, totalParties=3
        );
        
        try {
            // 连接到服务器
            if (!client.connect()) {
                System.err.println("❌ 连接失败");
                System.exit(1);
            }
            
            // 启动密钥生成
            if (client.startKeygen()) {
                byte[] dkgKey = client.getDkgKey();
                if (dkgKey != null) {
                    System.out.println("✅ 密钥生成成功");
                    System.out.println("DKG密钥长度: " + dkgKey.length);
                } else {
                    System.err.println("❌ 密钥生成失败 - 密钥为null");
                }
            } else {
                System.err.println("❌ 密钥生成失败");
            }
            
            // 保持连接一段时间
            Thread.sleep(5000);
            
        } catch (Exception e) {
            System.err.println("❌ 运行错误: " + e.getMessage());
            e.printStackTrace();
        } finally {
            client.disconnect();
        }
    }
}