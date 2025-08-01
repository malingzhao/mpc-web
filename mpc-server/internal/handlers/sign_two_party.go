package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/okx/threshold-lib/crypto/paillier"
	"github.com/okx/threshold-lib/tss/ecdsa/keygen"
	"github.com/okx/threshold-lib/tss/ecdsa/sign"
	"mpc-server/internal/mpc"
	"mpc-server/internal/protocol"
)

// TwoPartySignRequest 2方签名请求
type TwoPartySignRequest struct {
	SessionID string `json:"session_id" binding:"required"` // 密钥生成会话ID
	Message   string `json:"message" binding:"required"`    // 要签名的消息
	Partner   string `json:"partner" binding:"required"`    // 签名伙伴服务器ID
}

// TwoPartySignResponse 2方签名响应
type TwoPartySignResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Signature string `json:"signature,omitempty"`
}

// SignStep 签名步骤枚举
type SignStep int

const (
	StepPreParams SignStep = iota + 1
	StepKeygenP1
	StepKeygenP2
	StepSignRound1
	StepSignRound2
	StepSignRound3
	StepCompleted
)

// TwoPartySignSession 2方签名会话数据
type TwoPartySignSession struct {
	SessionID   string   `json:"session_id"`
	KeygenID    string   `json:"keygen_id"`
	Message     string   `json:"message"`
	Partner     string   `json:"partner"`
	IsInitiator bool     `json:"is_initiator"`
	CurrentStep SignStep `json:"current_step"`

	// 密钥生成阶段数据
	PreParams  *keygen.PreParamsWithDlnProof `json:"pre_params,omitempty"`
	PaiPrivate *paillier.PrivateKey          `json:"pai_private,omitempty"`
	P1Data     interface{}                   `json:"p1_data,omitempty"`
	P2SaveData interface{}                   `json:"p2_save_data,omitempty"`

	// 签名阶段数据
	P1Context *sign.P1Context `json:"p1_context,omitempty"`
	P2Context *sign.P2Context `json:"p2_context,omitempty"`

	// 中间结果
	IntermediateData map[string]interface{} `json:"intermediate_data"`
}

// InitTwoPartySign 初始化2方签名
func (h *Handler) InitTwoPartySign(c *gin.Context) {
	var req TwoPartySignRequest
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

	// 验证密钥生成会话是否存在且已完成
	keygenSession, err := h.mpcManager.GetSession(req.SessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Keygen session not found: %v", err),
		})
		return
	}

	if keygenSession.Status != mpc.StatusCompleted {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Keygen session must be completed before signing",
		})
		return
	}

	// 创建新的签名会话
	participants := []string{h.config.ID, req.Partner}
	signSession, err := h.mpcManager.CreateSession(mpc.TypeSign, participants, 2)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 创建2方签名会话数据
	twoPartySession := &TwoPartySignSession{
		SessionID:        signSession.ID,
		KeygenID:         req.SessionID,
		Message:          req.Message,
		Partner:          req.Partner,
		IsInitiator:      true,
		CurrentStep:      StepPreParams,
		IntermediateData: make(map[string]interface{}),
	}

	// 保存会话数据
	signSession.Data["two_party_session"] = twoPartySession

	// 发送签名初始化消息给伙伴
	initData := &protocol.TwoPartySignInitData{
		SignSessionID: signSession.ID,
		KeygenID:      req.SessionID,
		Message:       req.Message,
		Initiator:     h.config.ID,
	}

	h.sendToParticipant(req.Partner, protocol.MsgTypeTwoPartySignInit, signSession.ID, initData)

	// 开始签名流程
	go h.processTwoPartySign(signSession.ID)

	c.JSON(http.StatusOK, TwoPartySignResponse{
		SessionID: signSession.ID,
		Status:    string(signSession.Status),
		Message:   "Two-party sign session initiated successfully",
	})
}

// processTwoPartySign 处理2方签名流程
func (h *Handler) processTwoPartySign(sessionID string) {
	session, err := h.mpcManager.GetSession(sessionID)
	if err != nil {
		log.Printf("Failed to get session %s: %v", sessionID, err)
		return
	}

	twoPartyData, ok := session.Data["two_party_session"].(*TwoPartySignSession)
	if !ok {
		log.Printf("Invalid two-party session data for session %s", sessionID)
		return
	}

	// 根据当前步骤执行相应逻辑
	switch twoPartyData.CurrentStep {
	case StepPreParams:
		h.handlePreParamsStep(session, twoPartyData)
	case StepKeygenP1:
		h.handleKeygenP1Step(session, twoPartyData)
	case StepKeygenP2:
		h.handleKeygenP2Step(session, twoPartyData)
	case StepSignRound1:
		h.handleSignRound1Step(session, twoPartyData)
	case StepSignRound2:
		h.handleSignRound2Step(session, twoPartyData)
	case StepSignRound3:
		h.handleSignRound3Step(session, twoPartyData)
	}
}

// handlePreParamsStep 处理预参数生成步骤
func (h *Handler) handlePreParamsStep(session *mpc.Session, twoPartyData *TwoPartySignSession) {
	log.Printf("Handling pre-params step for session %s", session.ID)

	if twoPartyData.IsInitiator {
		// 作为发起者，生成预参数
		preParams := keygen.GeneratePreParamsWithDlnProof()
		twoPartyData.PreParams = preParams

		// 生成Paillier密钥对
		paiPrivate, _, err := paillier.NewKeyPair(8)
		if err != nil {
			log.Printf("Failed to generate Paillier key pair: %v", err)
			return
		}
		twoPartyData.PaiPrivate = paiPrivate

		// 发送预参数给伙伴
		preParamsData := &protocol.PreParamsData{
			Params: preParams.Params,
			Proof:  preParams.Proof,
		}

		h.sendToParticipant(twoPartyData.Partner, protocol.MsgTypePreParams, session.ID, preParamsData)

		// 更新步骤
		twoPartyData.CurrentStep = StepKeygenP1
		session.Data["two_party_session"] = twoPartyData
	}
}

// handleKeygenP1Step 处理密钥生成P1步骤
func (h *Handler) handleKeygenP1Step(session *mpc.Session, twoPartyData *TwoPartySignSession) {
	log.Printf("Handling keygen P1 step for session %s", session.ID)

	// 等待接收到伙伴的预参数确认
	if _, exists := twoPartyData.IntermediateData["partner_pre_params"]; exists {
		// 从密钥生成会话获取必要数据
		_, err := h.mpcManager.GetSession(twoPartyData.KeygenID)
		if err != nil {
			log.Printf("Failed to get keygen session: %v", err)
			return
		}

		// 这里需要从keygenSession中获取p1Data和p2Data
		// 实际实现中需要根据具体的密钥生成结果来获取这些数据

		// 执行P1密钥协商
		// p1Dto, E_x1, err := keygen.P1(...)

		// 发送P1数据给伙伴
		// ...

		twoPartyData.CurrentStep = StepKeygenP2
		session.Data["two_party_session"] = twoPartyData
	}
}

// handleKeygenP2Step 处理密钥生成P2步骤
func (h *Handler) handleKeygenP2Step(session *mpc.Session, twoPartyData *TwoPartySignSession) {
	log.Printf("Handling keygen P2 step for session %s", session.ID)

	// 等待P1数据并执行P2密钥协商
	// ...

	twoPartyData.CurrentStep = StepSignRound1
	session.Data["two_party_session"] = twoPartyData
}

// handleSignRound1Step 处理签名第1轮
func (h *Handler) handleSignRound1Step(session *mpc.Session, twoPartyData *TwoPartySignSession) {
	log.Printf("Handling sign round 1 for session %s", session.ID)

	// 创建签名上下文并执行第1轮
	// ...

	twoPartyData.CurrentStep = StepSignRound2
	session.Data["two_party_session"] = twoPartyData
}

// handleSignRound2Step 处理签名第2轮
func (h *Handler) handleSignRound2Step(session *mpc.Session, twoPartyData *TwoPartySignSession) {
	log.Printf("Handling sign round 2 for session %s", session.ID)

	// 执行第2轮签名
	// ...

	twoPartyData.CurrentStep = StepSignRound3
	session.Data["two_party_session"] = twoPartyData
}

// handleSignRound3Step 处理签名第3轮
func (h *Handler) handleSignRound3Step(session *mpc.Session, twoPartyData *TwoPartySignSession) {
	log.Printf("Handling sign round 3 for session %s", session.ID)

	// 完成签名并保存结果
	// ...

	twoPartyData.CurrentStep = StepCompleted
	session.Status = mpc.StatusCompleted
	session.Data["two_party_session"] = twoPartyData
}

// sendToParticipant 发送消息给特定参与者
func (h *Handler) sendToParticipant(participantID string, msgType protocol.MessageType, sessionID string, data interface{}) {
	if participantID != h.config.ID {
		msg := protocol.NewMessage(msgType, sessionID, h.config.ID, participantID, data)
		msgBytes, err := msg.ToJSON()
		if err != nil {
			log.Printf("Failed to marshal message: %v", err)
			return
		}

		if err := h.peerClient.SendToPeer(participantID, msgBytes); err != nil {
			log.Printf("Failed to send message to %s: %v", participantID, err)
		}
	}
}
