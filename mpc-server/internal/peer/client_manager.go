package peer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ClientManager 客户端连接管理器
type ClientManager struct {
	clientID       string
	serverURL      string
	conn           *websocket.Conn
	connected      bool
	mutex          sync.RWMutex
	writeMutex     sync.Mutex // 添加写入锁
	messageHandler func([]byte)
	autoDisconnect bool
}

// NewClientManager 创建客户端管理器
func NewClientManager(clientID, serverURL string, autoDisconnect bool) *ClientManager {
	return &ClientManager{
		clientID:       clientID,
		serverURL:      serverURL,
		autoDisconnect: autoDisconnect,
		connected:      false,
	}
}

// SetMessageHandler 设置消息处理器
func (cm *ClientManager) SetMessageHandler(handler func([]byte)) {
	cm.messageHandler = handler
}

// ConnectToServer 连接到服务器
func (cm *ClientManager) ConnectToServer() error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.connected {
		return nil // 已经连接
	}

	// 建立WebSocket连接
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	// 添加客户端ID到URL参数
	url := fmt.Sprintf("%s?client_id=%s", cm.serverURL, cm.clientID)

	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %v", err)
	}

	cm.conn = conn
	cm.connected = true

	log.Printf("Client %s connected to server %s", cm.clientID, cm.serverURL)

	// 启动消息接收循环
	go cm.messageLoop()

	return nil
}

// messageLoop 消息接收循环
func (cm *ClientManager) messageLoop() {
	defer func() {
		cm.mutex.Lock()
		cm.connected = false
		if cm.conn != nil {
			cm.conn.Close()
			cm.conn = nil
		}
		cm.mutex.Unlock()
	}()

	for {
		cm.mutex.RLock()
		conn := cm.conn
		cm.mutex.RUnlock()

		if conn == nil {
			break
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Client %s read error: %v", cm.clientID, err)
			break
		}

		// 处理消息
		if cm.messageHandler != nil {
			cm.messageHandler(message)
		}

		// 检查是否需要自动断开连接
		if cm.autoDisconnect && cm.shouldDisconnect(message) {
			log.Printf("Client %s auto-disconnecting after session completion", cm.clientID)
			cm.Disconnect()
			break
		}
	}
}

// shouldDisconnect 检查是否应该断开连接
func (cm *ClientManager) shouldDisconnect(message []byte) bool {
	// 解析消息以检查会话状态
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		return false
	}

	// 检查是否是会话完成消息
	if msgType, ok := msg["type"].(string); ok {
		if msgType == "session_complete" || msgType == "session_failed" {
			return true
		}

		// 检查keygen、reshare或sign完成
		if msgType == "keygen_complete" || msgType == "reshare_complete" || msgType == "sign_complete" {
			return true
		}
	}

	return false
}

// SendMessage 发送消息到服务器
func (cm *ClientManager) SendMessage(message []byte) error {
	cm.mutex.RLock()
	if !cm.connected || cm.conn == nil {
		cm.mutex.RUnlock()
		return fmt.Errorf("not connected to server")
	}
	conn := cm.conn
	cm.mutex.RUnlock()

	// 使用写入锁保护WebSocket写入操作
	cm.writeMutex.Lock()
	defer cm.writeMutex.Unlock()

	return conn.WriteMessage(websocket.TextMessage, message)
}

// Disconnect 断开连接
func (cm *ClientManager) Disconnect() error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if !cm.connected || cm.conn == nil {
		return nil
	}

	err := cm.conn.Close()
	cm.conn = nil
	cm.connected = false

	log.Printf("Client %s disconnected from server", cm.clientID)
	return err
}

// IsConnected 检查是否已连接
func (cm *ClientManager) IsConnected() bool {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.connected
}

// RequestOperation 请求操作（通过HTTP API）
func (cm *ClientManager) RequestOperation(operation string, params map[string]interface{}) (map[string]interface{}, error) {
	// 构建HTTP请求到enterprise服务器
	// 这里假设enterprise服务器在8082端口提供HTTP API
	apiURL := "http://localhost:8082/api/v1/" + operation

	// 将参数转换为JSON
	jsonData, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %v", err)
	}

	// 发送HTTP POST请求
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// 解析响应
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	// 如果请求成功，建立WebSocket连接
	if resp.StatusCode == http.StatusOK {
		if !cm.IsConnected() {
			if err := cm.ConnectToServer(); err != nil {
				log.Printf("Failed to connect to server after successful API request: %v", err)
			}
		}
	}

	return result, nil
}

// StartKeygen 启动密钥生成
func (cm *ClientManager) StartKeygen(participants []string, threshold int) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"participants": participants,
		"threshold":    threshold,
	}
	return cm.RequestOperation("keygen", params)
}

// StartReshare 启动密钥重分享
func (cm *ClientManager) StartReshare(sessionID string, newParticipants []string, newThreshold int) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"session_id":       sessionID,
		"new_participants": newParticipants,
		"new_threshold":    newThreshold,
	}
	return cm.RequestOperation("reshare", params)
}

// StartSign 启动签名操作
func (cm *ClientManager) StartSign(sessionID string, message string, signers []string) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"session_id": sessionID,
		"message":    message,
		"signers":    signers,
	}
	return cm.RequestOperation("sign", params)
}
