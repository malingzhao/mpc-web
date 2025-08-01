package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// CoordinatorServer 协调服务器
type CoordinatorServer struct {
	clients   map[string]*Client
	sessions  map[string]*Session
	mu        sync.RWMutex
	upgrader  websocket.Upgrader
}

// Client 客户端连接
type Client struct {
	ID       string
	Conn     *websocket.Conn
	Server   *CoordinatorServer
	Send     chan []byte
	mu       sync.Mutex
}

// Session MPC会话
type Session struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Status       string                 `json:"status"`
	Participants []string               `json:"participants"`
	Threshold    int                    `json:"threshold"`
	TotalParties int                    `json:"total_parties"`
	CreatedAt    time.Time              `json:"created_at"`
	Data         map[string]interface{} `json:"data"`
	mu           sync.RWMutex
}

// Message WebSocket消息
type Message struct {
	Type         string `json:"type"`
	SessionID    string `json:"session_id,omitempty"`
	FromParty    int    `json:"from_party,omitempty"`
	Data         string `json:"data,omitempty"`
	ClientID     string `json:"client_id,omitempty"`
	Threshold    int    `json:"threshold,omitempty"`
	TotalParties int    `json:"total_parties,omitempty"`
	SessionType  string `json:"session_type,omitempty"`
	Success      bool   `json:"success,omitempty"`
	Error        string `json:"error,omitempty"`
}

// NewCoordinatorServer 创建新的协调服务器
func NewCoordinatorServer() *CoordinatorServer {
	return &CoordinatorServer{
		clients:  make(map[string]*Client),
		sessions: make(map[string]*Session),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许所有来源
			},
		},
	}
}

// handleWebSocket 处理WebSocket连接
func (s *CoordinatorServer) handleWebSocket(c *gin.Context) {
	clientID := c.Query("client_id")
	log.Printf("🔍 收到WebSocket连接请求，客户端ID: %s", clientID)
	
	if clientID == "" {
		log.Printf("❌ WebSocket连接失败: 缺少client_id参数")
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少client_id参数"})
		return
	}

	log.Printf("🔄 正在升级WebSocket连接，客户端: %s", clientID)
	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("❌ WebSocket升级失败，客户端 %s: %v", clientID, err)
		return
	}

	log.Printf("✅ WebSocket升级成功，客户端: %s", clientID)

	log.Printf("🔧 正在创建客户端对象，客户端: %s", clientID)
	client := &Client{
		ID:     clientID,
		Conn:   conn,
		Server: s,
		Send:   make(chan []byte, 256),
	}

	log.Printf("🔒 正在获取锁以添加客户端，客户端: %s", clientID)
	s.mu.Lock()
	log.Printf("🔓 已获取锁，正在添加客户端到map，客户端: %s", clientID)
	s.clients[clientID] = client
	clientCount := len(s.clients)
	log.Printf("📊 客户端已添加到map，当前客户端数: %d，客户端: %s", clientCount, clientID)
	s.mu.Unlock()
	log.Printf("🔓 已释放锁，客户端: %s", clientID)

	log.Printf("🔗 客户端 %s 已连接，当前总客户端数: %d", clientID, clientCount)

	// 启动客户端处理协程
	go client.writePump()
	go client.readPump()
}

// readPump 读取客户端消息
func (c *Client) readPump() {
	defer func() {
		c.Server.mu.Lock()
		delete(c.Server.clients, c.ID)
		c.Server.mu.Unlock()
		c.Conn.Close()
		log.Printf("🔌 客户端 %s 已断开连接", c.ID)
	}()

	c.Conn.SetReadLimit(512 * 1024) // 512KB
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg Message
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("❌ 客户端 %s WebSocket错误: %v", c.ID, err)
			}
			break
		}

		c.Server.handleMessage(c, &msg)
	}
}

// writePump 向客户端发送消息
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("❌ 向客户端 %s 发送消息失败: %v", c.ID, err)
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage 处理客户端消息
func (s *CoordinatorServer) handleMessage(client *Client, msg *Message) {
	log.Printf("📨 收到来自客户端 %s 的消息: %s", client.ID, msg.Type)

	switch msg.Type {
	case "create_session":
		s.handleCreateSession(client, msg)
	case "keygen_round1":
		s.handleKeygenRound(client, msg, 1)
	case "keygen_round2":
		s.handleKeygenRound(client, msg, 2)
	case "keygen_complete":
		s.handleKeygenComplete(client, msg)
	default:
		log.Printf("⚠️ 未知消息类型: %s", msg.Type)
	}
}

// handleCreateSession 处理创建会话请求
func (s *CoordinatorServer) handleCreateSession(client *Client, msg *Message) {
	log.Printf("📥 收到来自客户端 %s 的create_session请求: SessionType=%s, Threshold=%d, TotalParties=%d", 
		client.ID, msg.SessionType, msg.Threshold, msg.TotalParties)
	
	s.mu.Lock()
	// 检查是否已经有活跃的会话
	var existingSession *Session
	for sessionID, session := range s.sessions {
		if session.Type == msg.SessionType && 
		   session.Status != "completed" &&
		   session.Threshold == msg.Threshold &&
		   session.TotalParties == msg.TotalParties &&
		   len(session.Participants) < session.TotalParties {
			existingSession = session
			log.Printf("🔄 找到匹配的现有会话 %s (当前参与者: %d/%d)", 
				sessionID, len(session.Participants), session.TotalParties)
			break
		}
	}
	
	if existingSession != nil {
		// 加入现有会话
		existingSession.mu.Lock()
		// 检查客户端是否已经在参与者列表中
		found := false
		for _, participant := range existingSession.Participants {
			if participant == client.ID {
				found = true
				log.Printf("⚠️ 客户端 %s 已经在会话 %s 中", client.ID, existingSession.ID)
				break
			}
		}
		if !found {
			existingSession.Participants = append(existingSession.Participants, client.ID)
			log.Printf("👥 客户端 %s 已加入现有会话 %s，当前参与者数量: %d/%d", 
				client.ID, existingSession.ID, len(existingSession.Participants), existingSession.TotalParties)
		}
		
		// 检查是否所有参与者都已连接
		allParticipantsReady := len(existingSession.Participants) >= existingSession.TotalParties
		if allParticipantsReady {
			existingSession.Status = "ready"
		}
		participants := make([]string, len(existingSession.Participants))
		copy(participants, existingSession.Participants)
		existingSession.mu.Unlock()
		
		log.Printf("📋 客户端 %s 加入现有会话 %s", client.ID, existingSession.ID)
		
		// 释放主锁后再发送消息
		s.mu.Unlock()
		
		// 通知客户端会话已创建
		response := &Message{
			Type:      "session_created",
			SessionID: existingSession.ID,
		}
		
		log.Printf("📤 向客户端 %s 发送session_created消息: SessionID=%s", 
			client.ID, response.SessionID)
		
		s.sendToClient(client.ID, response)
		
		if allParticipantsReady {
			log.Printf("🎯 会话 %s 准备就绪，参与者: %v", existingSession.ID, participants)
			
			// 通知所有参与者会话准备就绪
			for _, participant := range participants {
				response := &Message{
					Type:      "session_created",
					SessionID: existingSession.ID,
				}
				s.sendToClient(participant, response)
			}
		} else {
			log.Printf("⏳ 会话 %s 等待更多参与者加入 (%d/%d)", 
				existingSession.ID, len(participants), existingSession.TotalParties)
		}
		return
	}
	
	// 创建新会话
	sessionID := fmt.Sprintf("session_%d", time.Now().Unix())

	session := &Session{
		ID:           sessionID,
		Type:         msg.SessionType,
		Status:       "created",
		Participants: []string{client.ID},
		Threshold:    msg.Threshold,
		TotalParties: msg.TotalParties,
		CreatedAt:    time.Now(),
		Data:         make(map[string]interface{}),
	}

	s.sessions[sessionID] = session

	log.Printf("🆕 创建新会话 %s，类型: %s，阈值: %d，总参与方: %d", 
		sessionID, msg.SessionType, msg.Threshold, msg.TotalParties)

	// 释放主锁后再发送消息
	s.mu.Unlock()

	// 通知客户端会话已创建
	response := &Message{
		Type:      "session_created",
		SessionID: sessionID,
	}

	log.Printf("📤 向客户端 %s 发送session_created消息: SessionID=%s", 
		client.ID, response.SessionID)

	s.sendToClient(client.ID, response)

	log.Printf("✅ 成功向客户端 %s 发送session_created消息", client.ID)

	// 等待其他参与者加入
	go s.waitForParticipants(sessionID)
}

// waitForParticipants 等待其他参与者加入
func (s *CoordinatorServer) waitForParticipants(sessionID string) {
	s.mu.RLock()
	session := s.sessions[sessionID]
	s.mu.RUnlock()

	if session == nil {
		return
	}

	expectedParticipants := session.TotalParties
	log.Printf("⏳ 等待 %d 个参与者连接到会话 %s", expectedParticipants, sessionID)

	// 等待所有参与者连接，最多等待30秒
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			log.Printf("⚠️ 会话 %s 等待参与者超时", sessionID)
			return
		case <-ticker.C:
			s.mu.RLock()
			clientCount := len(s.clients)
			clients := make([]*Client, 0)
			for _, client := range s.clients {
				clients = append(clients, client)
			}
			s.mu.RUnlock()

			if clientCount >= expectedParticipants {
				// 更新参与者列表
				session.mu.Lock()
				session.Participants = make([]string, 0)
				for _, client := range clients {
					session.Participants = append(session.Participants, client.ID)
				}
				session.Status = "ready"
				session.mu.Unlock()

				log.Printf("🎯 会话 %s 准备就绪，参与者: %v", sessionID, session.Participants)

				// 通知所有参与者开始密钥生成
				for _, client := range clients {
					response := &Message{
						Type:      "start_keygen",
						SessionID: sessionID,
					}
					log.Printf("📤 向客户端 %s 发送start_keygen信号", client.ID)
					s.sendToClient(client.ID, response)
				}
				return
			}
			log.Printf("⏳ 会话 %s 等待参与者: %d/%d", sessionID, clientCount, expectedParticipants)
		}
	}
}

// handleKeygenRound 处理密钥生成轮次
func (s *CoordinatorServer) handleKeygenRound(client *Client, msg *Message, round int) {
	log.Printf("🔄 处理来自客户端 %s 的第 %d 轮密钥生成数据", client.ID, round)

	s.mu.RLock()
	session := s.sessions[msg.SessionID]
	s.mu.RUnlock()

	if session == nil {
		log.Printf("❌ 会话 %s 不存在", msg.SessionID)
		return
	}

	// 存储轮次数据
	session.mu.Lock()
	if session.Data == nil {
		session.Data = make(map[string]interface{})
	}
	key := fmt.Sprintf("round%d_%s", round, client.ID)
	session.Data[key] = msg.Data
	session.mu.Unlock()

	// 检查是否收集到所有参与者的数据
	if s.hasAllRoundData(session, round) {
		s.broadcastRoundData(session, round)
	}
}

// hasAllRoundData 检查是否收集到所有轮次数据
func (s *CoordinatorServer) hasAllRoundData(session *Session, round int) bool {
	session.mu.RLock()
	defer session.mu.RUnlock()

	expectedCount := len(session.Participants)
	actualCount := 0

	for key := range session.Data {
		prefix := fmt.Sprintf("round%d_", round)
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			actualCount++
		}
	}

	return actualCount >= expectedCount
}

// broadcastRoundData 广播轮次数据
func (s *CoordinatorServer) broadcastRoundData(session *Session, round int) {
	log.Printf("📡 广播会话 %s 第 %d 轮数据", session.ID, round)

	session.mu.RLock()
	participants := session.Participants
	data := session.Data
	session.mu.RUnlock()

	// 向每个参与者发送其他参与者的数据
	for _, participant := range participants {
		for _, otherParticipant := range participants {
			if participant != otherParticipant {
				key := fmt.Sprintf("round%d_%s", round, otherParticipant)
				if roundData, exists := data[key]; exists {
					// 解析参与方ID
					partyID := s.getPartyID(otherParticipant)

					response := &Message{
						Type:      fmt.Sprintf("keygen_round%d", round),
						SessionID: session.ID,
						FromParty: partyID,
						Data:      roundData.(string),
					}

					s.sendToClient(participant, response)
				}
			}
		}
	}
}

// getPartyID 获取参与方ID
func (s *CoordinatorServer) getPartyID(clientID string) int {
	// 基于客户端ID分配参与方ID，与启动参数匹配
	switch clientID {
	case "go-client-1":
		return 1
	case "go-client-2":
		return 2
	case "java-client":
		return 3
	default:
		return 1
	}
}

// handleKeygenComplete 处理密钥生成完成
func (s *CoordinatorServer) handleKeygenComplete(client *Client, msg *Message) {
	log.Printf("✅ 客户端 %s 密钥生成完成", client.ID)

	s.mu.RLock()
	session := s.sessions[msg.SessionID]
	s.mu.RUnlock()

	if session == nil {
		return
	}

	// 存储完成状态
	session.mu.Lock()
	if session.Data == nil {
		session.Data = make(map[string]interface{})
	}
	session.Data["complete_"+client.ID] = true
	session.mu.Unlock()

	// 检查是否所有参与者都完成
	if s.allParticipantsComplete(session) {
		s.broadcastKeygenComplete(session)
	}
}

// allParticipantsComplete 检查是否所有参与者都完成
func (s *CoordinatorServer) allParticipantsComplete(session *Session) bool {
	session.mu.RLock()
	defer session.mu.RUnlock()

	for _, participant := range session.Participants {
		key := "complete_" + participant
		if _, exists := session.Data[key]; !exists {
			return false
		}
	}

	return true
}

// broadcastKeygenComplete 广播密钥生成完成
func (s *CoordinatorServer) broadcastKeygenComplete(session *Session) {
	log.Printf("🎉 会话 %s 所有参与者密钥生成完成", session.ID)

	session.mu.Lock()
	session.Status = "completed"
	session.mu.Unlock()

	response := &Message{
		Type:      "keygen_complete",
		SessionID: session.ID,
		Success:   true,
	}

	s.mu.RLock()
	participants := session.Participants
	s.mu.RUnlock()

	for _, participant := range participants {
		s.sendToClient(participant, response)
	}
}

// sendToClient 向指定客户端发送消息
func (s *CoordinatorServer) sendToClient(clientID string, msg *Message) {
	s.mu.RLock()
	client := s.clients[clientID]
	s.mu.RUnlock()

	if client == nil {
		log.Printf("⚠️ 客户端 %s 不存在", clientID)
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("❌ 序列化消息失败: %v", err)
		return
	}

	select {
	case client.Send <- data:
	default:
		close(client.Send)
		s.mu.Lock()
		delete(s.clients, clientID)
		s.mu.Unlock()
	}
}

// getStatus 获取服务器状态
func (s *CoordinatorServer) getStatus(c *gin.Context) {
	s.mu.RLock()
	clientCount := len(s.clients)
	sessionCount := len(s.sessions)
	s.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"status":   "running",
		"clients":  clientCount,
		"sessions": sessionCount,
		"time":     time.Now(),
	})
}

// getSessions 获取会话列表
func (s *CoordinatorServer) getSessions(c *gin.Context) {
	s.mu.RLock()
	sessions := make([]*Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}
	s.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
	})
}

func main() {
	// 创建协调服务器
	server := NewCoordinatorServer()

	// 设置Gin模式
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 添加CORS中间件
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// WebSocket端点
	r.GET("/ws", server.handleWebSocket)

	// 健康检查端点
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 状态端点
	r.GET("/api/v1/status", func(c *gin.Context) {
		server.mu.RLock()
		status := gin.H{
			"status":   "running",
			"clients":  len(server.clients),
			"sessions": len(server.sessions),
			"time":     time.Now(),
		}
		server.mu.RUnlock()
		c.JSON(http.StatusOK, status)
	})

	// 会话端点
	r.GET("/api/v1/sessions", func(c *gin.Context) {
		server.mu.RLock()
		sessions := make([]gin.H, 0, len(server.sessions))
		for id, session := range server.sessions {
			sessions = append(sessions, gin.H{
				"id":           id,
				"type":         session.Type,
				"status":       session.Status,
				"participants": len(session.Participants),
				"created_at":   session.CreatedAt,
			})
		}
		server.mu.RUnlock()
		c.JSON(http.StatusOK, gin.H{"sessions": sessions})
	})

	port := ":8080"
	log.Printf("🚀 协调服务器启动在端口%s", port)
	log.Printf("📡 WebSocket端点: ws://localhost%s/ws", port)
	log.Printf("🔍 健康检查: http://localhost%s/health", port)
	log.Printf("🔍 状态端点: http://localhost%s/api/v1/status", port)

	if err := r.Run(port); err != nil {
		log.Fatalf("❌ 服务器启动失败: %v", err)
	}
}