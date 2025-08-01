package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"mpc-server/internal/config"
	"mpc-server/internal/mpc"
	"mpc-server/internal/peer"
	"mpc-server/internal/protocol"
	ws "mpc-server/internal/websocket"
	"net/http"
	"strconv"
)

// Handler 处理器结构
type Handler struct {
	config     *config.ServerConfig
	mpcManager *mpc.MPCManager
	hub        *ws.Hub
	peerClient *peer.PeerClient
}

// KeygenRequest 密钥生成请求
type KeygenRequest struct {
	Threshold    int      `json:"threshold" binding:"required"`
	Participants []string `json:"participants" binding:"required"`
}

// KeygenResponse 密钥生成响应
type KeygenResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// ReshareRequest 密钥重分享请求
type ReshareRequest struct {
	SessionID       string   `json:"session_id" binding:"required"`
	NewThreshold    int      `json:"new_threshold" binding:"required"`
	NewParticipants []string `json:"new_participants" binding:"required"`
}

// ReshareResponse 密钥重分享响应
type ReshareResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// SignRequest 签名请求
type SignRequest struct {
	SessionID string   `json:"session_id" binding:"required"`
	Message   string   `json:"message" binding:"required"`
	Signers   []string `json:"signers" binding:"required"`
}

// SignResponse 签名响应
type SignResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// NewHandler 创建新的处理器
func NewHandler(serverID string, mpcManager *mpc.MPCManager, hub *ws.Hub) (*Handler, error) {
	cfg, err := config.GetServerConfig(serverID)
	if err != nil {
		return nil, err
	}

	// 创建PeerClient
	peerClient := peer.NewPeerClient(serverID, cfg)

	handler := &Handler{
		config:     cfg,
		mpcManager: mpcManager, // 允许为nil
		hub:        hub,
		peerClient: peerClient,
	}

	// 设置PeerClient的消息处理器
	peerClient.SetMessageHandler(handler)

	return handler, nil
}

// GetPeerClient 获取PeerClient
func (h *Handler) GetPeerClient() *peer.PeerClient {
	return h.peerClient
}

// SetMPCManager 设置MPCManager
func (h *Handler) SetMPCManager(mpcManager *mpc.MPCManager) {
	h.mpcManager = mpcManager
}

// GetServerInfo 获取服务器信息
func (h *Handler) GetServerInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"server_id":    h.config.ID,
		"name":         h.config.Name,
		"port":         h.config.Port,
		"capabilities": h.config.Capabilities,
		"peers":        h.config.Peers,
	})
}

// InitKeygen 初始化密钥生成
func (h *Handler) InitKeygen(c *gin.Context) {
	log.Printf("=== ENTERING InitKeygen FUNCTION ===")
	var req KeygenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("=== ERROR BINDING JSON: %v ===", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	log.Printf("=== JSON BINDING SUCCESSFUL ===")

	// 检查服务器是否支持keygen
	if !h.config.HasCapability("keygen") {
		log.Printf("=== SERVER DOES NOT SUPPORT KEYGEN ===")
		c.JSON(http.StatusForbidden, gin.H{
			"error": "This server does not support keygen operation",
		})
		return
	}

	log.Printf("=== SERVER SUPPORTS KEYGEN ===")

	// 创建会话
	session, err := h.mpcManager.CreateSession(mpc.TypeKeygen, req.Participants, req.Threshold)
	if err != nil {
		log.Printf("=== ERROR CREATING SESSION: %v ===", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("=== SESSION CREATED SUCCESSFULLY: %s ===", session.ID)

	// 广播keygen初始化消息给所有参与者
	initData := &protocol.KeygenInitData{
		Threshold:    req.Threshold,
		Participants: req.Participants,
	}

	// 同步会话到所有peer服务器
	log.Printf("About to sync session %s to peers", session.ID)
	h.syncSessionToPeers(session)
	log.Printf("Finished syncing session %s to peers", session.ID)

	log.Printf("About to broadcast to participants for session %s", session.ID)
	//通知其他服务启动setup
	h.broadcastToParticipants(session.ID, req.Participants, protocol.MsgTypeKeygenInit, initData)
	log.Printf("Finished broadcasting to participants for session %s", session.ID)

	// 启动MPC协议自动执行
	log.Printf("About to start keygen protocol for session %s", session.ID)
	//自身启动setup
	if err := h.mpcManager.StartKeygenProtocol(session.ID); err != nil {
		log.Printf("Failed to start keygen protocol for session %s: %v", session.ID, err)
	} else {
		log.Printf("Successfully started keygen protocol for session %s", session.ID)
	}
	h.broadcastToParticipants(session.ID, req.Participants, protocol.MsgTypeKeygenRound1, nil)
	log.Printf("success to send msgType 1 message\n")
	h.mpcManager.ProcessRound(session.ID, 1)
	//
	c.JSON(http.StatusOK, KeygenResponse{
		SessionID: session.ID,
		Status:    string(session.Status),
		Message:   "Keygen session initiated successfully",
	})
}

// InitReshare 初始化密钥重分享
func (h *Handler) InitReshare(c *gin.Context) {
	var req ReshareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查服务器是否支持reshare
	if !h.config.HasCapability("reshare") {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "This server does not support reshare operation",
		})
		return
	}

	// 创建会话
	session, err := h.mpcManager.CreateSession(mpc.TypeReshare, req.NewParticipants, req.NewThreshold)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 广播reshare初始化消息给所有参与者
	initData := &protocol.ReshareInitData{
		SessionID:       req.SessionID,
		NewThreshold:    req.NewThreshold,
		NewParticipants: req.NewParticipants,
	}

	h.broadcastToParticipants(session.ID, req.NewParticipants, protocol.MsgTypeReshareInit, initData)

	// 同步会话到所有peer服务器
	h.syncSessionToPeers(session)

	// 启动MPC协议自动执行
	if err := h.mpcManager.StartReshareProtocol(session.ID); err != nil {
		log.Printf("Failed to start reshare protocol for session %s: %v", session.ID, err)
	}

	c.JSON(http.StatusOK, ReshareResponse{
		SessionID: session.ID,
		Status:    string(session.Status),
		Message:   "Reshare session initiated successfully",
	})
}

// InitSign 初始化签名
func (h *Handler) InitSign(c *gin.Context) {
	var req SignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查服务器是否支持sign
	if !h.config.HasCapability("sign") {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "This server does not support sign operation",
		})
		return
	}

	// 创建会话
	session, err := h.mpcManager.CreateSession(mpc.TypeSign, req.Signers, len(req.Signers))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 创建sign初始化数据
	initData := &protocol.SignInitData{
		SessionID: req.SessionID,
		Message:   req.Message,
		Signers:   req.Signers,
	}

	// 首先在本地处理SignInitData，设置消息到会话中
	if err := h.mpcManager.ProcessSignInit(session.ID, initData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to initialize sign session: %v", err)})
		return
	}

	// 广播sign初始化消息给其他参与者
	h.broadcastToParticipants(session.ID, req.Signers, protocol.MsgTypeSignInit, initData)

	// 同步会话到所有peer服务器
	h.syncSessionToPeers(session)

	// 启动MPC协议自动执行
	if err := h.mpcManager.StartSignProtocol(session.ID); err != nil {
		log.Printf("Failed to start sign protocol for session %s: %v", session.ID, err)
	}

	c.JSON(http.StatusOK, SignResponse{
		SessionID: session.ID,
		Status:    string(session.Status),
		Message:   "Sign session initiated successfully",
	})
}

// GetSessionStatus 获取会话状态
func (h *Handler) GetSessionStatus(c *gin.Context) {
	sessionID := c.Param("sessionId")
	session, err := h.mpcManager.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id":   session.ID,
		"type":         session.Type,
		"status":       session.Status,
		"participants": session.Participants,
		"threshold":    session.Threshold,
		"data":         session.Data,
		"created_at":   session.CreatedAt,
		"updated_at":   session.UpdatedAt,
	})
}

// ListSessions 列出所有会话
func (h *Handler) ListSessions(c *gin.Context) {
	// 获取查询参数
	statusFilter := c.Query("status")
	typeFilter := c.Query("type")
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	sessions := h.mpcManager.ListSessions(statusFilter, typeFilter, limit, offset)

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"total":    len(sessions),
	})
}

// broadcastToParticipants 向参与者广播消息
func (h *Handler) broadcastToParticipants(sessionID string, participants []string, msgType protocol.MessageType, data interface{}) {
	for _, participant := range participants {
		if participant != h.config.ID {
			msg := protocol.NewMessage(msgType, sessionID, h.config.ID, participant, data)
			msgBytes, err := msg.ToJSON()
			if err != nil {
				log.Printf("Failed to marshal message: %v", err)
				continue
			}

			// 尝试发送给指定的peer服务器
			if err := h.peerClient.SendToPeer(participant, msgBytes); err != nil {
				log.Printf("Failed to send message to peer %s: %v", participant, err)
				// 如果peer连接失败，尝试通过本地WebSocket广播
				h.hub.BroadcastMessage(msgBytes)
			}
		}
	}
}

// HandleWebSocketMessage 处理WebSocket消息
func (h *Handler) HandleWebSocketMessage(client *ws.Client, messageBytes []byte) {
	// 解析消息
	msg, err := protocol.FromJSON(messageBytes)
	if err != nil {
		log.Printf("Failed to parse message: %v", err)
		h.sendError(client, "", "Invalid message format")
		return
	}

	log.Printf("Received message from %s: type=%s, session=%s,msg=%v", client.ID, msg.Type, msg.SessionID, msg)

	// 根据消息类型处理
	switch msg.Type {
	case protocol.MsgTypeKeygenInit:
		h.handleKeygenInit(client, msg)
	case protocol.MsgTypeKeygenRound1:
		h.mpcManager.ProcessRound(msg.SessionID, 1)
	case protocol.MsgTypeKeygenRound2:
		h.handleKeygenRound(client, msg)
	case protocol.MsgTypeKeygenRound3:
		h.handleKeygenRound(client, msg)
	case protocol.MsgTypeReshareInit:
		h.handleReshareInit(client, msg)
	case protocol.MsgTypeReshareRound:
		h.handleReshareRound(client, msg)
	case protocol.MsgTypeSignInit:
		h.handleSignInit(client, msg)
	case protocol.MsgTypeSignRound:
		h.handleSignRound(client, msg)
	case protocol.MsgTypeHeartbeat:
		h.handleHeartbeat(client, msg)
	case protocol.MsgTypeSessionSync:
		h.handleSessionSync(client, msg)
	default:
		log.Printf("Unknown message type: %s", msg.Type)
		h.sendError(client, msg.SessionID, "Unknown message type")
	}
}

// handleKeygenInit 处理密钥生成初始化
func (h *Handler) handleKeygenInit(client *ws.Client, msg *protocol.Message) {
	if !h.config.HasCapability("keygen") {
		h.sendError(client, msg.SessionID, "Server does not support keygen")
		return
	}

	// 解析初始化数据
	var initData protocol.KeygenInitData
	dataBytes, _ := json.Marshal(msg.Data)
	if err := json.Unmarshal(dataBytes, &initData); err != nil {
		h.sendError(client, msg.SessionID, fmt.Sprintf("Invalid keygen init data: %v", err))
		return
	}
	h.mpcManager.StartKeygenProtocol(msg.SessionID)
}

// handleKeygenRound 处理密钥生成轮次
func (h *Handler) handleKeygenRound(client *ws.Client, msg *protocol.Message) {
	var roundData protocol.KeygenRoundData
	dataBytes, _ := json.Marshal(msg.Data)
	if err := json.Unmarshal(dataBytes, &roundData); err != nil {
		h.sendError(client, msg.SessionID, fmt.Sprintf("Invalid keygen round data: %v", err))
		return
	}

	h.mpcManager.ProcessKeygenRound(msg.SessionID, &roundData)
}

// handleReshareInit 处理密钥重分享初始化
func (h *Handler) handleReshareInit(client *ws.Client, msg *protocol.Message) {
	if !h.config.HasCapability("reshare") {
		h.sendError(client, msg.SessionID, "Server does not support reshare")
		return
	}

	// 解析初始化数据
	var initData protocol.ReshareInitData
	dataBytes, _ := json.Marshal(msg.Data)
	if err := json.Unmarshal(dataBytes, &initData); err != nil {
		h.sendError(client, msg.SessionID, fmt.Sprintf("Invalid reshare init data: %v", err))
		return
	}

	// 开始密钥重分享过程
	go h.mpcManager.ProcessReshareRound(msg.SessionID, &protocol.ReshareRoundData{
		Round: 1,
		Data:  msg.Data,
	})
}

// handleReshareRound 处理密钥重分享轮次
func (h *Handler) handleReshareRound(client *ws.Client, msg *protocol.Message) {
	var roundData protocol.ReshareRoundData
	dataBytes, _ := json.Marshal(msg.Data)
	if err := json.Unmarshal(dataBytes, &roundData); err != nil {
		h.sendError(client, msg.SessionID, fmt.Sprintf("Invalid reshare round data: %v", err))
		return
	}

	go h.mpcManager.ProcessReshareRound(msg.SessionID, &roundData)
}

// handleSignInit 处理签名初始化
func (h *Handler) handleSignInit(client *ws.Client, msg *protocol.Message) {
	if !h.config.HasCapability("sign") {
		h.sendError(client, msg.SessionID, "Server does not support sign")
		return
	}

	// 解析初始化数据
	var initData protocol.SignInitData
	dataBytes, _ := json.Marshal(msg.Data)
	if err := json.Unmarshal(dataBytes, &initData); err != nil {
		h.sendError(client, msg.SessionID, fmt.Sprintf("Invalid sign init data: %v", err))
		return
	}

	// 开始签名过程
	go h.mpcManager.ProcessSignRound(msg.SessionID, &protocol.SignRoundData{
		Round: 1,
		Data:  msg.Data,
	})
}

// handleSignRound 处理签名轮次
func (h *Handler) handleSignRound(client *ws.Client, msg *protocol.Message) {
	var roundData protocol.SignRoundData
	dataBytes, _ := json.Marshal(msg.Data)
	if err := json.Unmarshal(dataBytes, &roundData); err != nil {
		h.sendError(client, msg.SessionID, fmt.Sprintf("Invalid sign round data: %v", err))
		return
	}

	go h.mpcManager.ProcessSignRound(msg.SessionID, &roundData)
}

// handleHeartbeat 处理心跳
func (h *Handler) handleHeartbeat(client *ws.Client, msg *protocol.Message) {
	// 回复心跳
	response := protocol.NewMessage(protocol.MsgTypeAck, "", h.config.ID, msg.From, nil)
	responseBytes, _ := response.ToJSON()
	client.Send <- responseBytes
}

// sendError 发送错误消息
func (h *Handler) sendError(client *ws.Client, sessionID, errorMsg string) {
	errorData := &protocol.ErrorData{
		Code:    500,
		Message: errorMsg,
	}
	msg := protocol.NewMessage(protocol.MsgTypeError, sessionID, h.config.ID, "", errorData)
	msgBytes, _ := msg.ToJSON()
	client.Send <- msgBytes
}

// handleSessionSync 处理会话同步消息
func (h *Handler) handleSessionSync(client *ws.Client, msg *protocol.Message) {
	var syncData protocol.SessionSyncData
	dataBytes, _ := json.Marshal(msg.Data)
	if err := json.Unmarshal(dataBytes, &syncData); err != nil {
		log.Printf("Invalid session sync data: %v", err)
		return
	}

	log.Printf("Received session sync for session %s from %s", syncData.SessionID, msg.From)

	// 同步会话到本地
	err := h.mpcManager.SyncSession(&syncData)
	if err != nil {
		log.Printf("Failed to sync session %s: %v", syncData.SessionID, err)
	} else {
		log.Printf("Successfully synced session %s", syncData.SessionID)
	}
}

// HandlePeerMessage 处理来自peer的消息
func (h *Handler) HandlePeerMessage(peerID string, messageBytes []byte) {
	// 解析消息
	msg, err := protocol.FromJSON(messageBytes)
	if err != nil {
		log.Printf("Failed to parse peer message from %s: %v", peerID, err)
		return
	}

	log.Printf("Received peer message from %s: type=%s, session=%s", peerID, msg.Type, msg.SessionID)

	// 根据消息类型处理
	switch msg.Type {
	case protocol.MsgTypeSessionSync:
		h.handlePeerSessionSync(peerID, msg)
	case protocol.MsgTypeKeygenInit:
		h.handlePeerKeygenInit(peerID, msg)
	case protocol.MsgTypeReshareInit:
		h.handlePeerReshareInit(peerID, msg)
	case protocol.MsgTypeSignInit:
		h.handlePeerSignInit(peerID, msg)
	default:
		log.Printf("Unknown peer message type from %s: %s", peerID, msg.Type)
	}
}

// handlePeerSessionSync 处理来自peer的会话同步
func (h *Handler) handlePeerSessionSync(peerID string, msg *protocol.Message) {
	var syncData protocol.SessionSyncData
	dataBytes, _ := json.Marshal(msg.Data)
	if err := json.Unmarshal(dataBytes, &syncData); err != nil {
		log.Printf("Invalid session sync data from peer %s: %v", peerID, err)
		return
	}

	log.Printf("Received session sync for session %s from peer %s", syncData.SessionID, peerID)

	// 同步会话到本地
	err := h.mpcManager.SyncSession(&syncData)
	if err != nil {
		log.Printf("Failed to sync session %s from peer %s: %v", syncData.SessionID, peerID, err)
	} else {
		log.Printf("Successfully synced session %s from peer %s", syncData.SessionID, peerID)
	}
}

// handlePeerKeygenInit 处理来自peer的keygen初始化
func (h *Handler) handlePeerKeygenInit(peerID string, msg *protocol.Message) {
	if !h.config.HasCapability("keygen") {
		log.Printf("Received keygen init from peer %s but this server doesn't support keygen", peerID)
		return
	}

	var initData protocol.KeygenInitData
	dataBytes, _ := json.Marshal(msg.Data)
	if err := json.Unmarshal(dataBytes, &initData); err != nil {
		log.Printf("Invalid keygen init data from peer %s: %v", peerID, err)
		return
	}

	log.Printf("Received keygen init for session %s from peer %s", msg.SessionID, peerID)

	// 创建本地会话
	session, err := h.mpcManager.CreateSession(mpc.TypeKeygen, initData.Participants, initData.Threshold)
	if err != nil {
		log.Printf("Failed to create keygen session %s: %v", msg.SessionID, err)
		return
	}

	// 设置会话ID为接收到的ID
	h.mpcManager.SetSessionID(session.ID, msg.SessionID)

	// 启动MPC协议自动执行
	if err := h.mpcManager.StartKeygenProtocol(msg.SessionID); err != nil {
		log.Printf("Failed to start keygen protocol for session %s: %v", msg.SessionID, err)
	}

	// 开始密钥生成过程
	//go h.mpcManager.ProcessKeygenRound(msg.SessionID, &protocol.KeygenRoundData{
	//	Round: 1,
	//	Data:  msg.Data,
	//})
}

// handlePeerReshareInit 处理来自peer的reshare初始化
func (h *Handler) handlePeerReshareInit(peerID string, msg *protocol.Message) {
	if !h.config.HasCapability("reshare") {
		log.Printf("Received reshare init from peer %s but this server doesn't support reshare", peerID)
		return
	}

	var initData protocol.ReshareInitData
	dataBytes, _ := json.Marshal(msg.Data)
	if err := json.Unmarshal(dataBytes, &initData); err != nil {
		log.Printf("Invalid reshare init data from peer %s: %v", peerID, err)
		return
	}

	log.Printf("Received reshare init for session %s from peer %s", msg.SessionID, peerID)

	// 创建本地会话
	session, err := h.mpcManager.CreateSession(mpc.TypeReshare, initData.NewParticipants, initData.NewThreshold)
	if err != nil {
		log.Printf("Failed to create reshare session %s: %v", msg.SessionID, err)
		return
	}

	// 设置会话ID为接收到的ID
	h.mpcManager.SetSessionID(session.ID, msg.SessionID)

	// 启动MPC协议自动执行
	if err := h.mpcManager.StartReshareProtocol(msg.SessionID); err != nil {
		log.Printf("Failed to start reshare protocol for session %s: %v", msg.SessionID, err)
	}
}

// handlePeerSignInit 处理来自peer的sign初始化
func (h *Handler) handlePeerSignInit(peerID string, msg *protocol.Message) {
	if !h.config.HasCapability("sign") {
		log.Printf("Received sign init from peer %s but this server doesn't support sign", peerID)
		return
	}

	var initData protocol.SignInitData
	dataBytes, _ := json.Marshal(msg.Data)
	if err := json.Unmarshal(dataBytes, &initData); err != nil {
		log.Printf("Invalid sign init data from peer %s: %v", peerID, err)
		return
	}

	log.Printf("Received sign init for session %s from peer %s", msg.SessionID, peerID)

	// 创建本地会话
	session, err := h.mpcManager.CreateSession(mpc.TypeSign, initData.Signers, len(initData.Signers))
	if err != nil {
		log.Printf("Failed to create sign session %s: %v", msg.SessionID, err)
		return
	}

	// 设置会话ID为接收到的ID
	h.mpcManager.SetSessionID(session.ID, msg.SessionID)

	// 启动MPC协议自动执行
	if err := h.mpcManager.StartSignProtocol(msg.SessionID); err != nil {
		log.Printf("Failed to start sign protocol for session %s: %v", msg.SessionID, err)
	}
}

// syncSessionToPeers 同步会话到所有peer
func (h *Handler) syncSessionToPeers(session *mpc.Session) {
	syncData := &protocol.SessionSyncData{
		SessionID:    session.ID,
		Type:         string(session.Type),
		Status:       string(session.Status),
		Participants: session.Participants,
		Threshold:    session.Threshold,
		CurrentRound: session.CurrentRound,
		Data:         session.Data,
		CreatedAt:    session.CreatedAt,
		UpdatedAt:    session.UpdatedAt,
	}

	h.peerClient.SendSessionSync(session.ID, syncData)
}

// ConnectToPeers 连接到所有peer服务器
func (h *Handler) ConnectToPeers() error {
	return h.peerClient.ConnectToPeers(h.config.Peers)
}

// GetConnectedPeers 获取已连接的peer列表
func (h *Handler) GetConnectedPeers() []string {
	return h.peerClient.GetConnectedPeers()
}
