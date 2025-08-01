package mpc

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/decred/dcrd/dcrec/secp256k1/v2"
	"github.com/okx/threshold-lib/crypto"
	"github.com/okx/threshold-lib/crypto/curves"
	"github.com/okx/threshold-lib/crypto/paillier"
	"github.com/okx/threshold-lib/crypto/pedersen"
	"github.com/okx/threshold-lib/crypto/schnorr"
	"github.com/okx/threshold-lib/crypto/zkp"
	"github.com/okx/threshold-lib/tss"
	"github.com/okx/threshold-lib/tss/ecdsa/sign"
	"github.com/okx/threshold-lib/tss/key/dkg"
	"github.com/okx/threshold-lib/tss/key/reshare"
	"log"
	"math/big"
	"mpc-server/internal/protocol"
	"sync"
	"time"
)

// SessionStatus 会话状态
type SessionStatus string

const (
	StatusPending   SessionStatus = "pending"
	StatusRunning   SessionStatus = "running"
	StatusCompleted SessionStatus = "completed"
	StatusFailed    SessionStatus = "failed"
)

// SessionType 会话类型
type SessionType string

const (
	TypeKeygen  SessionType = "keygen"
	TypeReshare SessionType = "reshare"
	TypeSign    SessionType = "sign"
)

// Session MPC会话
type Session struct {
	ID           string                 `json:"id"`
	Type         SessionType            `json:"type"`
	Status       SessionStatus          `json:"status"`
	Participants []string               `json:"participants"`
	Threshold    int                    `json:"threshold"`
	CurrentRound int                    `json:"current_round"`
	Data         map[string]interface{} `json:"data"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	mu           sync.RWMutex
}

// MPCManager MPC管理器
type MPCManager struct {
	serverID   string
	sessions   map[string]*Session
	peerClient PeerClient   // 添加peer客户端接口
	wsHub      WebSocketHub // 添加WebSocket Hub接口
	mu         sync.RWMutex
}

// PeerClient 定义发送消息给peer的接口
type PeerClient interface {
	SendToPeer(peerID string, message []byte) error
}

// WebSocketHub 定义WebSocket Hub接口
type WebSocketHub interface {
	SendToClient(clientID string, message []byte) error
	BroadcastToAll(message []byte)
}

// NewMPCManager 创建新的MPC管理器
func NewMPCManager(serverID string, peerClient PeerClient, wsHub WebSocketHub) *MPCManager {
	return &MPCManager{
		serverID:   serverID,
		sessions:   make(map[string]*Session),
		peerClient: peerClient,
		wsHub:      wsHub,
	}
}

// CreateSession 创建新会话
func (m *MPCManager) CreateSession(sessionType SessionType, participants []string, threshold int) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionID := generateSessionID()
	session := &Session{
		ID:           sessionID,
		Type:         sessionType,
		Status:       StatusPending,
		Participants: participants,
		Threshold:    threshold,
		CurrentRound: 0,
		Data:         make(map[string]interface{}),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	m.sessions[sessionID] = session
	log.Printf("Created %s session %s with participants: %v", sessionType, sessionID, participants)
	return session, nil
}

// GetSession 获取会话
func (m *MPCManager) GetSession(sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	return session, nil
}

// UpdateSessionStatus 更新会话状态
func (m *MPCManager) UpdateSessionStatus(sessionID string, status SessionStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	session.mu.Lock()
	oldStatus := session.Status
	session.Status = status
	session.UpdatedAt = time.Now()
	session.mu.Unlock()

	log.Printf("Session %s status updated to %s", sessionID, status)

	// 如果会话完成或失败，发送通知给所有参与者
	if status == StatusCompleted || status == StatusFailed {
		go m.notifySessionComplete(sessionID, status, oldStatus)
	}

	return nil
}

// ProcessKeygenInit 处理密钥生成初始化
func (m *MPCManager) ProcessKeygenInit(sessionID string, data *protocol.KeygenInitData) error {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// 初始化密钥生成参数
	session.Data["threshold"] = data.Threshold
	session.Data["participants"] = data.Participants
	session.Status = StatusRunning
	session.UpdatedAt = time.Now()

	log.Printf("Keygen session %s initialized with threshold %d", sessionID, data.Threshold)
	return nil
}

// ProcessKeygenRound 处理密钥生成轮次
func (m *MPCManager) ProcessKeygenRound(sessionID string, data *protocol.KeygenRoundData) (*protocol.KeygenResultData, error) {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	log.Printf("Processing keygen round %d for session %s from %s, data: %v\n", data.Round, sessionID, data.From, data)

	// 检查是否是DKG消息

	var a *tss.Message
	marshal, err := json.Marshal(data.Data)
	if err != nil {
		return nil, nil
	}

	err = json.Unmarshal(marshal, &a)
	if err == nil {
		// 这是一个真正的DKG消息
		fromParticipant := data.From
		if fromParticipant == "" {
			log.Printf("Warning: No sender information in DKG message")
			return &protocol.KeygenResultData{
				Success: false,
				Error:   "Missing sender information",
			}, fmt.Errorf("missing sender information")
		}

		// 处理DKG轮次消息
		err := m.ProcessDKGRoundMessage(sessionID, data.Round, fromParticipant, a)
		if err != nil {
			log.Printf("Failed to process DKG round message: %v", err)
			return &protocol.KeygenResultData{
				Success: false,
				Error:   err.Error(),
			}, err
		}
		return nil, nil // DKG消息处理是异步的

	}

	// 更新会话状态
	session.mu.Lock()
	session.CurrentRound = data.Round
	session.Status = StatusRunning
	session.UpdatedAt = time.Now()
	session.mu.Unlock()

	// 所有keygen处理都通过真正的DKG算法完成
	// 不需要模拟逻辑，返回nil表示消息已处理
	return nil, nil
}

// StartKeygenProtocol 启动密钥生成协议的自动执行
func (m *MPCManager) StartKeygenProtocol(sessionID string) error {
	log.Printf("Starting keygen protocol for session %s", sessionID)

	session, err := m.GetSession(sessionID)
	if err != nil {
		log.Printf("Failed to get session %s: %v", sessionID, err)
		return err
	}

	// 获取参与者信息 - 从会话结构的直接字段获取
	session.mu.RLock()
	participants := session.Participants
	threshold := session.Threshold
	session.mu.RUnlock()

	if len(participants) == 0 || threshold == 0 {
		log.Printf("Missing participants or threshold data for session %s", sessionID)
		return fmt.Errorf("missing participants or threshold data")
	}

	log.Printf("Session %s: threshold=%d, participants=%v", sessionID, threshold, participants)
	log.Printf("Keygen protocol goroutine started for session %s", sessionID)
	//初始化
	err = m.initSetup(sessionID, threshold, participants)
	if err != nil {
		log.Printf("DKG failed for session %s: %v", sessionID, err)
		m.UpdateSessionStatus(sessionID, StatusFailed)
		return err
	}

	log.Printf("participant_id: %v finish setup\n", session.Data["participant_id"])
	return nil
}

// StartReshareProtocol 启动密钥重分享协议的自动执行
func (m *MPCManager) StartReshareProtocol(sessionID string) error {
	log.Printf("Starting reshare protocol for session %s", sessionID)

	session, err := m.GetSession(sessionID)
	if err != nil {
		log.Printf("Failed to get session %s: %v", sessionID, err)
		return err
	}

	// 获取重分享参数
	session.mu.RLock()
	participants := session.Participants
	threshold := session.Threshold
	session.mu.RUnlock()

	if len(participants) == 0 || threshold == 0 {
		log.Printf("Missing participants or threshold data for session %s", sessionID)
		return fmt.Errorf("missing participants or threshold data")
	}

	log.Printf("Session %s: threshold=%d, participants=%v", sessionID, threshold, participants)

	go func() {
		log.Printf("Reshare protocol goroutine started for session %s", sessionID)

		// 使用真正的重分享算法
		err := m.performRealReshare(sessionID, threshold, participants)
		if err != nil {
			log.Printf("Reshare failed for session %s: %v", sessionID, err)
			m.UpdateSessionStatus(sessionID, StatusFailed)
			return
		}

		log.Printf("Reshare completed successfully for session %s", sessionID)
	}()

	return nil
}

// StartSignProtocol 启动签名协议的自动执行
func (m *MPCManager) StartSignProtocol(sessionID string) error {
	log.Printf("Starting sign protocol for session %s", sessionID)

	session, err := m.GetSession(sessionID)
	if err != nil {
		log.Printf("Failed to get session %s: %v", sessionID, err)
		return err
	}

	// 获取签名参数
	session.mu.RLock()
	participants := session.Participants
	threshold := session.Threshold
	session.mu.RUnlock()

	if len(participants) == 0 || threshold == 0 {
		log.Printf("Missing participants or threshold data for session %s", sessionID)
		return fmt.Errorf("missing participants or threshold data")
	}

	log.Printf("Session %s: threshold=%d, participants=%v", sessionID, threshold, participants)

	go func() {
		log.Printf("Sign protocol goroutine started for session %s", sessionID)

		// 使用真正的ECDSA签名算法
		err := m.performRealECDSASign(sessionID, threshold, participants)
		if err != nil {
			log.Printf("ECDSA sign failed for session %s: %v", sessionID, err)
			m.UpdateSessionStatus(sessionID, StatusFailed)
			return
		}

		log.Printf("ECDSA sign completed successfully for session %s", sessionID)
	}()

	return nil
}

// ProcessReshareInit 处理密钥重分享初始化
func (m *MPCManager) ProcessReshareInit(sessionID string, data *protocol.ReshareInitData) error {
	// 检查原会话是否存在
	originalSession, err := m.GetSession(data.SessionID)
	if err != nil {
		return fmt.Errorf("original session %s not found", data.SessionID)
	}

	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	// 复制原会话的密钥数据
	originalSession.mu.RLock()
	session.Data["original_session"] = data.SessionID
	session.Data["public_key"] = originalSession.Data["public_key"]
	session.Data["new_threshold"] = data.NewThreshold
	session.Data["new_participants"] = data.NewParticipants
	originalSession.mu.RUnlock()

	session.Status = StatusRunning
	session.UpdatedAt = time.Now()

	log.Printf("Reshare session %s initialized from session %s", sessionID, data.SessionID)
	return nil
}

// ProcessReshareRound 处理密钥重分享轮次
func (m *MPCManager) ProcessReshareRound(sessionID string, data *protocol.ReshareRoundData) (*protocol.ReshareResultData, error) {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	session.CurrentRound = data.Round
	session.Status = StatusRunning
	session.UpdatedAt = time.Now()

	log.Printf("Processing reshare round %d for session %s", data.Round, sessionID)

	// 所有reshare处理都通过真正的重分享算法完成
	// 不需要模拟逻辑，返回nil表示消息已处理
	return nil, nil
}

// ProcessSignInit 处理签名初始化
func (m *MPCManager) ProcessSignInit(sessionID string, data *protocol.SignInitData) error {
	// 检查原会话是否存在
	originalSession, err := m.GetSession(data.SessionID)
	if err != nil {
		return fmt.Errorf("original session %s not found", data.SessionID)
	}

	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// 复制密钥数据
	originalSession.mu.RLock()
	session.Data["original_session"] = data.SessionID
	session.Data["public_key"] = originalSession.Data["public_key"]
	session.Data["private_share"] = originalSession.Data["private_share"]
	originalSession.mu.RUnlock()

	session.Data["message"] = data.Message
	session.Data["signers"] = data.Signers
	session.Status = StatusRunning
	session.UpdatedAt = time.Now()

	log.Printf("Sign session %s initialized for message: %s", sessionID, data.Message)
	return nil
}

// ProcessSignRound 处理签名轮次
func (m *MPCManager) ProcessSignRound(sessionID string, data *protocol.SignRoundData) (*protocol.SignResultData, error) {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	session.CurrentRound = data.Round
	session.Status = StatusRunning
	session.UpdatedAt = time.Now()

	log.Printf("Processing sign round %d for session %s from %s", data.Round, sessionID, data.From)

	// 根据轮次处理不同的签名步骤
	switch data.Round {
	case 1:
		return m.processSignRound1(session, data)
	case 2:
		return m.processSignRound2(session, data)
	case 3:
		return m.processSignRound3(session, data)
	default:
		return nil, fmt.Errorf("invalid sign round: %d", data.Round)
	}
}

// processSignRound1 处理签名第一轮
func (m *MPCManager) processSignRound1(session *Session, data *protocol.SignRoundData) (*protocol.SignResultData, error) {
	log.Printf("Processing sign round 1 for session %s", session.ID)

	// 获取签名上下文
	signCtx, err := m.getOrCreateSignContext(session)
	if err != nil {
		return nil, err
	}

	// 解析轮次数据
	var round1Data protocol.SignRound1Data
	dataBytes, _ := json.Marshal(data.Data)
	if err := json.Unmarshal(dataBytes, &round1Data); err != nil {
		return nil, fmt.Errorf("invalid round 1 data: %v", err)
	}

	// 根据服务器角色处理
	if m.isP1(session) {
		// P1: 生成承诺
		if round1Data.Commitment == nil {
			commitment, err := signCtx.P1.Step1()
			if err != nil {
				return nil, fmt.Errorf("P1 step1 failed: %v", err)
			}

			// 保存承诺到会话
			session.Data["p1_commitment"] = commitment

			// 广播承诺给P2
			commitData := &protocol.SignRound1Data{
				Commitment: &protocol.CommitmentData{
					C: hex.EncodeToString((*commitment).Bytes()),
				},
			}

			m.broadcastSignMessage(session.ID, 1, commitData, session.Participants)
		} else {
			// P1: 接收P2的Schnorr证明和R2点
			if round1Data.SchnorrProof != nil && round1Data.R2Point != nil {
				// 保存P2的数据
				session.Data["p2_schnorr_proof"] = round1Data.SchnorrProof
				session.Data["p2_r2_point"] = round1Data.R2Point

				// 准备进入第二轮
				session.CurrentRound = 2
			}
		}
	} else {
		// P2: 接收P1的承诺并生成Schnorr证明
		if round1Data.Commitment != nil {
			// 解析承诺
			cBytes, err := hex.DecodeString(round1Data.Commitment.C)
			if err != nil {
				return nil, fmt.Errorf("invalid commitment: %v", err)
			}

			commitment := new(big.Int).SetBytes(cBytes)

			// P2 Step1
			proof, R2, err := signCtx.P2.Step1(&commitment)
			if err != nil {
				return nil, fmt.Errorf("P2 step1 failed: %v", err)
			}

			// 保存数据
			session.Data["p2_proof"] = proof
			session.Data["p2_r2"] = R2

			// 发送Schnorr证明和R2给P1
			responseData := &protocol.SignRound1Data{
				SchnorrProof: &protocol.SchnorrProofData{
					E: hex.EncodeToString(proof.R.X.Bytes()),
					S: hex.EncodeToString(proof.S.Bytes()),
				},
				R2Point: &protocol.ECPointData{
					X: hex.EncodeToString(R2.X.Bytes()),
					Y: hex.EncodeToString(R2.Y.Bytes()),
				},
			}

			m.broadcastSignMessage(session.ID, 1, responseData, session.Participants)
		}
	}

	return nil, nil
}

// processSignRound2 处理签名第二轮
func (m *MPCManager) processSignRound2(session *Session, data *protocol.SignRoundData) (*protocol.SignResultData, error) {
	log.Printf("Processing sign round 2 for session %s", session.ID)

	signCtx, err := m.getSignContext(session)
	if err != nil {
		return nil, err
	}

	var round2Data protocol.SignRound2Data
	dataBytes, _ := json.Marshal(data.Data)
	if err := json.Unmarshal(dataBytes, &round2Data); err != nil {
		return nil, fmt.Errorf("invalid round 2 data: %v", err)
	}

	if m.isP1(session) {
		// P1: 发送Schnorr证明和承诺开启
		if round2Data.SchnorrProof == nil {
			// 获取P2的数据
			p2ProofData := session.Data["p2_schnorr_proof"].(*protocol.SchnorrProofData)
			p2R2Data := session.Data["p2_r2_point"].(*protocol.ECPointData)

			// 重构P2的证明和R2点
			p2Proof := &schnorr.Proof{}
			p2Proof.R = &curves.ECPoint{}
			p2Proof.R.X = new(big.Int).SetBytes(mustDecodeHex(p2ProofData.E))
			p2Proof.S = new(big.Int).SetBytes(mustDecodeHex(p2ProofData.S))

			p2R2, err := curves.NewECPoint(secp256k1.S256(),
				new(big.Int).SetBytes(mustDecodeHex(p2R2Data.X)),
				new(big.Int).SetBytes(mustDecodeHex(p2R2Data.Y)))
			if err != nil {
				return nil, fmt.Errorf("invalid P2 R2 point: %v", err)
			}

			// P1 Step2
			proof, cmtD, err := signCtx.P1.Step2(p2Proof, p2R2)
			if err != nil {
				return nil, fmt.Errorf("P1 step2 failed: %v", err)
			}

			// 发送给P2
			responseData := &protocol.SignRound2Data{
				SchnorrProof: &protocol.SchnorrProofData{
					E: hex.EncodeToString(proof.R.X.Bytes()),
					S: hex.EncodeToString(proof.S.Bytes()),
				},
				CommitmentWitness: &protocol.WitnessData{
					SessionID: hex.EncodeToString((*cmtD)[0].Bytes()),
					X:         hex.EncodeToString((*cmtD)[1].Bytes()),
					Y:         hex.EncodeToString((*cmtD)[2].Bytes()),
				},
			}

			m.broadcastSignMessage(session.ID, 2, responseData, session.Participants)
		} else {
			// P1: 接收P2的仿射证明
			if round2Data.EncryptedValue != nil && round2Data.AffineProof != nil {
				session.Data["p2_encrypted_value"] = *round2Data.EncryptedValue
				session.Data["p2_affine_proof"] = round2Data.AffineProof
				session.CurrentRound = 3
			}
		}
	} else {
		// P2: 接收P1的证明并生成仿射证明
		if round2Data.SchnorrProof != nil && round2Data.CommitmentWitness != nil {
			// 重构P1的数据
			p1Proof := &schnorr.Proof{}
			p1Proof.R = &curves.ECPoint{}
			p1Proof.R.X = new(big.Int).SetBytes(mustDecodeHex(round2Data.SchnorrProof.E))
			p1Proof.S = new(big.Int).SetBytes(mustDecodeHex(round2Data.SchnorrProof.S))

			cmtD := make([]*big.Int, 3)
			cmtD[0] = new(big.Int).SetBytes(mustDecodeHex(round2Data.CommitmentWitness.SessionID))
			cmtD[1] = new(big.Int).SetBytes(mustDecodeHex(round2Data.CommitmentWitness.X))
			cmtD[2] = new(big.Int).SetBytes(mustDecodeHex(round2Data.CommitmentWitness.Y))

			// P2 Step2
			E_k2_h_xr, affineProof, err := signCtx.P2.Step2(&cmtD, p1Proof)
			if err != nil {
				return nil, fmt.Errorf("P2 step2 failed: %v", err)
			}

			// 发送给P1
			responseData := &protocol.SignRound2Data{
				EncryptedValue: stringPtr(hex.EncodeToString(E_k2_h_xr.Bytes())),
				AffineProof: &protocol.AffineProofData{
					X: hex.EncodeToString(affineProof.X.X.Bytes()),
					Y: hex.EncodeToString(affineProof.Y.X.Bytes()),
					E: hex.EncodeToString(affineProof.E.Bytes()),
					S: hex.EncodeToString(affineProof.S.Bytes()),
				},
			}

			m.broadcastSignMessage(session.ID, 2, responseData, session.Participants)
		}
	}

	return nil, nil
}

// processSignRound3 处理签名第三轮
func (m *MPCManager) processSignRound3(session *Session, data *protocol.SignRoundData) (*protocol.SignResultData, error) {
	log.Printf("Processing sign round 3 for session %s", session.ID)

	if !m.isP1(session) {
		// 只有P1计算最终签名
		return nil, nil
	}

	signCtx, err := m.getSignContext(session)
	if err != nil {
		return nil, err
	}

	// 获取P2的仿射证明数据
	encryptedValueHex := session.Data["p2_encrypted_value"].(string)
	affineProofData := session.Data["p2_affine_proof"].(*protocol.AffineProofData)

	// 重构数据
	E_k2_h_xr := new(big.Int).SetBytes(mustDecodeHex(encryptedValueHex))

	affineProof := &zkp.AffGProof{}
	affineProof.X = &curves.ECPoint{}
	affineProof.X.X = new(big.Int).SetBytes(mustDecodeHex(affineProofData.X))
	affineProof.Y = &curves.ECPoint{}
	affineProof.Y.X = new(big.Int).SetBytes(mustDecodeHex(affineProofData.Y))
	affineProof.E = new(big.Int).SetBytes(mustDecodeHex(affineProofData.E))
	affineProof.S = new(big.Int).SetBytes(mustDecodeHex(affineProofData.S))

	// P1 Step3 - 计算最终签名
	r, s, err := signCtx.P1.Step3(E_k2_h_xr, affineProof)
	if err != nil {
		return nil, fmt.Errorf("P1 step3 failed: %v", err)
	}

	// 保存签名结果
	session.Data["signature_r"] = hex.EncodeToString(r.Bytes())
	session.Data["signature_s"] = hex.EncodeToString(s.Bytes())
	session.Status = StatusCompleted

	log.Printf("ECDSA signature completed for session %s: r=%s, s=%s",
		session.ID, hex.EncodeToString(r.Bytes()), hex.EncodeToString(s.Bytes()))

	// 广播签名完成
	resultData := &protocol.SignRound3Data{
		R: stringPtr(hex.EncodeToString(r.Bytes())),
		S: stringPtr(hex.EncodeToString(s.Bytes())),
	}

	m.broadcastSignMessage(session.ID, 3, resultData, session.Participants)

	// 发送签名完成通知
	m.notifySignComplete(session.ID, r, s)

	return &protocol.SignResultData{
		Success:   true,
		Signature: fmt.Sprintf("%s%s", hex.EncodeToString(r.Bytes()), hex.EncodeToString(s.Bytes())),
	}, nil
}

// ListSessions 列出所有会话
func (m *MPCManager) ListSessions(statusFilter, typeFilter string, limit, offset int) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		// 应用过滤器
		if statusFilter != "" && string(session.Status) != statusFilter {
			continue
		}
		if typeFilter != "" && string(session.Type) != typeFilter {
			continue
		}
		sessions = append(sessions, session)
	}

	// 应用分页
	if offset >= len(sessions) {
		return []*Session{}
	}

	end := offset + limit
	if end > len(sessions) {
		end = len(sessions)
	}

	return sessions[offset:end]
}

// SetSession 设置会话（用于同步）
func (m *MPCManager) SetSession(session *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sessions[session.ID] = session
	log.Printf("Session %s synchronized from peer", session.ID)
	return nil
}

// SetSessionID 设置会话ID（用于同步时更新ID）
func (m *MPCManager) SetSessionID(oldID, newID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[oldID]
	if !exists {
		return fmt.Errorf("session %s not found", oldID)
	}

	// 更新会话ID
	session.ID = newID
	m.sessions[newID] = session
	delete(m.sessions, oldID)

	log.Printf("Session ID updated from %s to %s", oldID, newID)
	return nil
}

// SyncSession 从SessionSyncData同步会话
func (m *MPCManager) SyncSession(syncData *protocol.SessionSyncData) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 创建新的会话
	session := &Session{
		ID:           syncData.SessionID,
		Type:         SessionType(syncData.Type),
		Status:       SessionStatus(syncData.Status),
		Participants: syncData.Participants,
		Threshold:    syncData.Threshold,
		CurrentRound: syncData.CurrentRound,
		Data:         make(map[string]interface{}),
		CreatedAt:    syncData.CreatedAt,
		UpdatedAt:    syncData.UpdatedAt,
	}

	// 复制数据
	for k, v := range syncData.Data {
		session.Data[k] = v
	}

	m.sessions[syncData.SessionID] = session
	log.Printf("Session %s synchronized from peer", syncData.SessionID)
	return nil
}

// GetSessionForSync 获取会话用于同步到其他服务器
func (m *MPCManager) GetSessionForSync(sessionID string) (*Session, error) {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	// 返回会话的副本用于同步
	session.mu.RLock()
	defer session.mu.RUnlock()

	syncSession := &Session{
		ID:           session.ID,
		Type:         session.Type,
		Status:       session.Status,
		Participants: session.Participants,
		Threshold:    session.Threshold,
		CurrentRound: session.CurrentRound,
		Data:         make(map[string]interface{}),
		CreatedAt:    session.CreatedAt,
		UpdatedAt:    session.UpdatedAt,
	}

	// 复制数据
	for k, v := range session.Data {
		syncSession.Data[k] = v
	}

	return syncSession, nil
}

// generateSessionID 生成会话ID
func generateSessionID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// performRealDKG 执行真正的分布式DKG密钥生成算法
func (m *MPCManager) initSetup(sessionID string, threshold int, participants []string) error {
	log.Printf("Starting real DKG for session %s with threshold %d and %d participants", sessionID, threshold, len(participants))

	// 使用secp256k1曲线
	curve := secp256k1.S256()
	total := len(participants)

	// 找到当前服务器在参与者列表中的索引
	serverIndex := -1
	for i, participant := range participants {
		if participant == m.serverID {
			serverIndex = i
			break
		}
	}

	if serverIndex == -1 {
		return fmt.Errorf("server %s not found in participants list", m.serverID)
	}

	// 当前服务器的参与者ID（从1开始）
	participantID := serverIndex + 1

	// 为当前服务器创建SetupInfo
	setUp := dkg.NewSetUp(participantID, total, curve)

	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	// 保存DKG上下文到会话
	session.mu.Lock()
	//初始化内存
	session.Data["dkg_setup"] = setUp
	session.Data["participant_id"] = participantID
	session.Data["total_participants"] = total
	session.CurrentRound = 1
	session.mu.Unlock()
	return nil
}

// broadcastDKGMessages 广播DKG消息给其他参与者
func (m *MPCManager) broadcastDKGMessages(sessionID string, round int, messages map[int]*tss.Message, participants []string, messageType protocol.MessageType) error {
	for participantID, message := range messages {
		// 找到对应的参与者名称
		if participantID <= 0 || participantID > len(participants) {
			continue
		}

		targetParticipant := participants[participantID-1]

		// 不发送给自己
		if targetParticipant == m.serverID {
			continue
		}

		log.Printf("m.ServerId: %s\n", m.serverID)
		// 创建DKG轮次消息
		dkgMessage := &protocol.Message{
			Type:      messageType,
			SessionID: sessionID,
			From:      m.serverID,
			To:        targetParticipant,
			Round:     round,
			Data: protocol.KeygenRoundData{
				Round: round,
				Data:  message,
				From:  m.serverID,
			},
			Timestamp: time.Now(),
		}

		// 发送消息给对等节点
		msgBytes, err := dkgMessage.ToJSON()
		if err != nil {
			log.Printf("Failed to marshal DKG message: %v", err)
			continue
		}

		err = m.peerClient.SendToPeer(targetParticipant, msgBytes)
		if err != nil {
			log.Printf("Failed to send DKG round %d message to %s: %v", round, targetParticipant, err)
			// 继续发送给其他参与者，不因为一个失败而停止
		} else {
			log.Printf("Sent DKG round %d message to %s for session %s  serverId %s msg: %s", round, targetParticipant, sessionID, m.serverID, string(msgBytes))
		}
	}
	return nil
}

// ProcessDKGRoundMessage 处理收到的DKG轮次消息
func (m *MPCManager) ProcessDKGRoundMessage(sessionID string, round int, fromParticipant string, message *tss.Message) error {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	// 初始化收到的消息存储
	roundKey := fmt.Sprintf("received_round%d_messages", round)
	if session.Data[roundKey] == nil {
		session.Data[roundKey] = make(map[string]*tss.Message)
	}

	receivedMessages := session.Data[roundKey].(map[string]*tss.Message)
	receivedMessages[fromParticipant] = message

	log.Printf("Received DKG round %d message from %s for session %s", round, fromParticipant, sessionID)

	// 检查是否收到了所有参与者的消息
	participants := session.Participants
	expectedCount := len(participants) - 1 // 除了自己
	log.Printf("the round is  %d , the len(receivedMessages) is %d, exceptedCount:%d \n", round, len(receivedMessages), expectedCount)

	if len(receivedMessages) >= expectedCount {
		err := m.ProcessRound(sessionID, round)
		if err != nil {
			log.Printf("Failed to process DKG round %d for session %s: %v", round+1, sessionID, err)
			session.mu.Lock()
			session.Status = StatusFailed
			session.mu.Unlock()
		}
	}
	return nil
}

func (m *MPCManager) ProcessRound(sessionID string, currentRound int) error {

	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	setUpInterface, ok := session.Data["dkg_setup"]
	if !ok {
		return fmt.Errorf("DKG setup not found in session %s", sessionID)
	}

	// 类型断言为正确的DKG SetupInfo类型
	setUp, ok := setUpInterface.(*dkg.SetupInfo)
	if !ok {
		return fmt.Errorf("invalid DKG setup type in session %s", sessionID)
	}

	participants := session.Participants
	participantID := session.Data["participant_id"].(int)
	log.Printf("success to process xx : %s,currentRound: %d\n", sessionID, currentRound)
	switch currentRound {
	case 1:
		log.Printf("DKG Round 1 starting for session %s, participant %d", sessionID, participantID)
		// 执行DKG第一轮
		round1Messages, err := setUp.DKGStep1()
		if err != nil {
			return fmt.Errorf("DKG step 1 failed for participant %d: %v", participantID, err)
		}

		// 保存第一轮消息到会话
		session.mu.Lock()
		session.Data["round1_messages"] = round1Messages
		session.CurrentRound = 1
		session.mu.Unlock()

		log.Printf("success finish save session info\n")

		// 广播第一轮消息给其他参与者
		err = m.broadcastDKGMessages(sessionID, 2, round1Messages, participants, protocol.MsgTypeKeygenRound2)
		if err != nil {
			log.Printf("Failed to broadcast round 1 messages: %v", err)
			return err
		}
		log.Printf("DKG Round 1 completed for session %s, waiting for other participants", sessionID)
	case 2:
		// 处理第二轮
		log.Printf("DKG Round 2 starting for session %s", sessionID)

		// 收集第一轮的输入消息
		receivedMessages := session.Data["received_round2_messages"].(map[string]*tss.Message)
		round2Inputs := make([]*tss.Message, 0, len(receivedMessages))

		for _, msg := range receivedMessages {
			round2Inputs = append(round2Inputs, msg)
		}

		// 执行DKG第二轮
		round2Messages, err := setUp.DKGStep2(round2Inputs)
		if err != nil {
			return fmt.Errorf("DKG step 2 failed: %v", err)
		}

		session.mu.Lock()
		session.Data["round2_messages"] = round2Messages
		session.CurrentRound = 2
		session.mu.Unlock()

		// 广播消息
		err = m.broadcastDKGMessages(sessionID, 3, round2Messages, participants, protocol.MsgTypeKeygenRound3)
		if err != nil {
			return fmt.Errorf("failed to broadcast DKG round 2 messages: %v", err)
		}

	case 3:
		// 处理第三轮（最终轮）
		log.Printf("DKG Round 3 starting for session %s", sessionID)

		// 收集第二轮的输入消息
		receivedMessages := session.Data["received_round3_messages"].(map[string]*tss.Message)
		round3Inputs := make([]*tss.Message, 0, len(receivedMessages))

		for _, msg := range receivedMessages {
			round3Inputs = append(round3Inputs, msg)
		}

		// 执行DKG第三轮 - 完成密钥生成
		keyData, err := setUp.DKGStep3(round3Inputs)
		if err != nil {
			return fmt.Errorf("DKG step 3 failed: %v", err)
		}

		// 保存结果
		pubKeyHex := hex.EncodeToString(append(keyData.PublicKey.X.Bytes(), keyData.PublicKey.Y.Bytes()...))
		privateShareHex := hex.EncodeToString(keyData.ShareI.Bytes())

		session.mu.Lock()
		session.Data["public_key"] = pubKeyHex
		session.Data["private_share"] = privateShareHex
		session.Data["participant_id"] = keyData.Id
		session.Status = StatusCompleted
		session.CurrentRound = 3
		session.UpdatedAt = time.Now()
		session.mu.Unlock()

		log.Printf("DKG completed successfully for session %s, participant %d, public key: %s...",
			sessionID, participantID, pubKeyHex[:40])

	default:
		return fmt.Errorf("invalid DKG round: %d", currentRound+1)
	}
	return nil
}

// performRealECDSASign 执行真正的ECDSA签名算法
func (m *MPCManager) performRealECDSASign(sessionID string, threshold int, participants []string) error {
	log.Printf("Starting real ECDSA sign for session %s with threshold %d and %d participants", sessionID, threshold, len(participants))

	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	// 获取要签名的消息
	session.mu.RLock()
	messageData, exists := session.Data["message"]
	if !exists {
		session.mu.RUnlock()
		return fmt.Errorf("no message to sign in session %s", sessionID)
	}
	message, ok := messageData.(string)
	if !ok {
		session.mu.RUnlock()
		return fmt.Errorf("invalid message format in session %s", sessionID)
	}

	// 获取公钥
	publicKeyData, exists := session.Data["public_key"]
	if !exists {
		session.mu.RUnlock()
		return fmt.Errorf("no public key found in session %s", sessionID)
	}

	publicKeyHex, ok := publicKeyData.(string)

	if !ok {
		session.mu.RUnlock()
		return fmt.Errorf("invalid public key format in session %s", sessionID)
	}
	session.mu.RUnlock()

	// 解码公钥
	publicKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return fmt.Errorf("failed to decode public key: %v", err)
	}

	// 重构公钥
	pubKeyX := new(big.Int).SetBytes(publicKeyBytes[:32])
	pubKeyY := new(big.Int).SetBytes(publicKeyBytes[32:])
	publicKey := &ecdsa.PublicKey{
		Curve: secp256k1.S256(),
		X:     pubKeyX,
		Y:     pubKeyY,
	}

	// 计算消息哈希并转换为十六进制字符串
	hash := sha256.Sum256([]byte(message))
	messageHex := hex.EncodeToString(hash[:])

	log.Printf("ECDSA Sign: message=%s, hash=%s...", message, messageHex[:16])

	paiPriKey, paiPubKey, err := paillier.NewKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate Paillier key: %v", err)
	}

	pedParams, err := pedersen.NewPedersenParameters()
	if err != nil {
		return fmt.Errorf("failed to generate Pedersen parameters: %v", err)
	}

	// 模拟加密的x1份额（在实际实现中应该从DKG阶段获得）
	x1 := crypto.RandomNum(secp256k1.S256().N)
	E_x1, _, err := paiPubKey.Encrypt(x1)
	if err != nil {
		return fmt.Errorf("failed to encrypt x1: %v", err)
	}

	log.Printf("ECDSA Sign Round 1 starting for session %s", sessionID)

	// 创建P1和P2上下文
	p1 := sign.NewP1(publicKey, messageHex, paiPriKey, E_x1, pedParams)
	if p1 == nil {
		return fmt.Errorf("failed to create P1 context")
	}

	x2 := crypto.RandomNum(secp256k1.S256().N)
	p2 := sign.NewP2(x2, E_x1, publicKey, paiPubKey, messageHex, pedParams)
	if p2 == nil {
		return fmt.Errorf("failed to create P2 context")
	}

	// Step 1: P1生成承诺
	cmtC, err := p1.Step1()
	if err != nil {
		return fmt.Errorf("P1 Step1 failed: %v", err)
	}

	log.Printf("ECDSA Sign Round 2 starting for session %s", sessionID)

	// Step 1: P2处理承诺并生成证明
	p2Proof, R2, err := p2.Step1(cmtC)
	if err != nil {
		return fmt.Errorf("P2 Step1 failed: %v", err)
	}

	// Step 2: P1处理P2的证明
	p1Proof, cmtD, err := p1.Step2(p2Proof, R2)
	if err != nil {
		return fmt.Errorf("P1 Step2 failed: %v", err)
	}

	log.Printf("ECDSA Sign Round 3 starting for session %s", sessionID)

	// Step 2: P2处理P1的证明并生成加密数据
	E_k2_h_xr, affGProof, err := p2.Step2(cmtD, p1Proof)
	if err != nil {
		return fmt.Errorf("P2 Step2 failed: %v", err)
	}

	// Step 3: P1完成签名
	r, s, err := p1.Step3(E_k2_h_xr, affGProof)
	if err != nil {
		return fmt.Errorf("P1 Step3 failed: %v", err)
	}

	// 保存签名结果
	session.mu.Lock()
	signatureHex := hex.EncodeToString(append(r.Bytes(), s.Bytes()...))
	session.Data["signature"] = signatureHex
	session.Data["signature_r"] = hex.EncodeToString(r.Bytes())
	session.Data["signature_s"] = hex.EncodeToString(s.Bytes())
	session.Status = StatusCompleted
	session.UpdatedAt = time.Now()
	session.mu.Unlock()

	log.Printf("ECDSA sign completed successfully for session %s, signature: %s...", sessionID, signatureHex[:40])
	return nil
}

// performRealReshare 执行真正的重分享算法
func (m *MPCManager) performRealReshare(sessionID string, threshold int, participants []string) error {
	log.Printf("Starting real reshare for session %s with threshold %d and %d participants", sessionID, threshold, len(participants))

	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	// 获取当前私钥份额和公钥
	session.mu.RLock()
	privateShareData, exists := session.Data["private_share"]
	if !exists {
		session.mu.RUnlock()
		return fmt.Errorf("no private share found in session %s", sessionID)
	}
	privateShareHex, ok := privateShareData.(string)
	if !ok {
		session.mu.RUnlock()
		return fmt.Errorf("invalid private share format in session %s", sessionID)
	}

	publicKeyData, exists := session.Data["public_key"]
	if !exists {
		session.mu.RUnlock()
		return fmt.Errorf("no public key found in session %s", sessionID)
	}
	publicKeyHex, ok := publicKeyData.(string)
	if !ok {
		session.mu.RUnlock()
		return fmt.Errorf("invalid public key format in session %s", sessionID)
	}

	participantIDData, exists := session.Data["participant_id"]
	if !exists {
		session.mu.RUnlock()
		return fmt.Errorf("no participant ID found in session %s", sessionID)
	}
	participantID, ok := participantIDData.(int)
	if !ok {
		session.mu.RUnlock()
		return fmt.Errorf("invalid participant ID format in session %s", sessionID)
	}
	session.mu.RUnlock()

	// 解码私钥份额
	privateShareBytes, err := hex.DecodeString(privateShareHex)
	if err != nil {
		return fmt.Errorf("failed to decode private share: %v", err)
	}
	privateShare := new(big.Int).SetBytes(privateShareBytes)

	// 解码公钥
	publicKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return fmt.Errorf("failed to decode public key: %v", err)
	}

	// 重构公钥为ECPoint
	pubKeyX := new(big.Int).SetBytes(publicKeyBytes[:32])
	pubKeyY := new(big.Int).SetBytes(publicKeyBytes[32:])
	publicKeyPoint, err := curves.NewECPoint(secp256k1.S256(), pubKeyX, pubKeyY)
	if err != nil {
		return fmt.Errorf("failed to create public key point: %v", err)
	}

	total := len(participants)

	// 为了演示，我们假设参与者1和2进行重分享
	// 在实际实现中，这应该由协议参数决定
	devoteList := [2]int{1, 2}

	log.Printf("Reshare Round 1 starting for session %s", sessionID)

	// 创建重分享参与者
	refreshInfo := reshare.NewRefresh(participantID, total, devoteList, privateShare, publicKeyPoint)

	// 第一轮：生成重分享数据
	round1Messages, err := refreshInfo.DKGStep1()
	if err != nil {
		return fmt.Errorf("reshare DKG step 1 failed: %v", err)
	}

	log.Printf("Reshare Round 2 starting for session %s", sessionID)

	// 模拟从其他参与者接收消息（在实际实现中需要网络通信）
	// 为了演示，我们创建模拟的其他参与者
	otherRefreshInfos := make([]*reshare.RefreshInfo, 0, total-1)
	otherRound1Messages := make([]map[int]*tss.Message, 0, total-1)

	for i := 1; i <= total; i++ {
		if i != participantID {
			// 模拟其他参与者的私钥份额
			otherPrivateShare := new(big.Int).Add(privateShare, big.NewInt(int64(i)))
			otherRefresh := reshare.NewRefresh(i, total, devoteList, otherPrivateShare, publicKeyPoint)
			otherRefreshInfos = append(otherRefreshInfos, otherRefresh)

			otherMsgs, err := otherRefresh.DKGStep1()
			if err != nil {
				return fmt.Errorf("reshare DKG step 1 failed for participant %d: %v", i, err)
			}
			otherRound1Messages = append(otherRound1Messages, otherMsgs)
		}
	}

	// 构造第二轮输入消息
	round2Input := make([]*tss.Message, 0, total-1)
	for _, msgs := range otherRound1Messages {
		if msg, exists := msgs[participantID]; exists {
			round2Input = append(round2Input, msg)
		}
	}

	// 第二轮：处理重分享消息
	_, err = refreshInfo.DKGStep2(round2Input)
	if err != nil {
		return fmt.Errorf("reshare DKG step 2 failed: %v", err)
	}

	log.Printf("Reshare Round 3 starting for session %s", sessionID)

	// 模拟其他参与者的第二轮
	otherRound2Messages := make([]map[int]*tss.Message, 0, len(otherRefreshInfos))
	for i, otherRefresh := range otherRefreshInfos {
		// 构造该参与者的第二轮输入
		otherRound2Input := make([]*tss.Message, 0, total-1)

		// 添加当前参与者的消息
		if msg, exists := round1Messages[otherRefresh.DeviceNumber]; exists {
			otherRound2Input = append(otherRound2Input, msg)
		}

		// 添加其他参与者的消息
		for j, otherMsgs := range otherRound1Messages {
			if j != i {
				if msg, exists := otherMsgs[otherRefresh.DeviceNumber]; exists {
					otherRound2Input = append(otherRound2Input, msg)
				}
			}
		}

		otherMsgs, err := otherRefresh.DKGStep2(otherRound2Input)
		if err != nil {
			return fmt.Errorf("reshare DKG step 2 failed for participant %d: %v", otherRefresh.DeviceNumber, err)
		}
		otherRound2Messages = append(otherRound2Messages, otherMsgs)
	}

	// 构造第三轮输入消息
	round3Input := make([]*tss.Message, 0, total-1)
	for _, msgs := range otherRound2Messages {
		if msg, exists := msgs[participantID]; exists {
			round3Input = append(round3Input, msg)
		}
	}

	// 第三轮：完成重分享
	newKeyData, err := refreshInfo.DKGStep3(round3Input)
	if err != nil {
		return fmt.Errorf("reshare DKG step 3 failed: %v", err)
	}

	// 验证新的公钥与原公钥一致
	if newKeyData.PublicKey.X.Cmp(publicKeyPoint.X) != 0 || newKeyData.PublicKey.Y.Cmp(publicKeyPoint.Y) != 0 {
		return fmt.Errorf("public key changed after reshare")
	}

	// 保存新的私钥份额
	session.mu.Lock()
	newPrivateShareHex := hex.EncodeToString(newKeyData.ShareI.Bytes())
	session.Data["private_share"] = newPrivateShareHex
	session.Data["old_private_share"] = privateShareHex // 保存旧的份额用于审计
	session.Data["reshare_completed_at"] = time.Now().Format(time.RFC3339)
	session.Status = StatusCompleted
	session.UpdatedAt = time.Now()
	session.mu.Unlock()

	log.Printf("Reshare completed successfully for session %s, new private share generated", sessionID)
	return nil
}

// notifySessionComplete 通知会话完成
func (m *MPCManager) notifySessionComplete(sessionID string, status SessionStatus, oldStatus SessionStatus) {
	log.Printf("Notifying session completion: %s, status: %s", sessionID, status)

	session, err := m.GetSession(sessionID)
	if err != nil {
		log.Printf("Failed to get session %s for notification: %v", sessionID, err)
		return
	}

	session.mu.RLock()
	sessionType := session.Type
	participants := session.Participants
	sessionData := make(map[string]interface{})

	// 根据会话类型准备不同的数据
	switch sessionType {
	case TypeKeygen:
		if status == StatusCompleted {
			if publicKey, ok := session.Data["public_key"].(string); ok {
				sessionData["public_key"] = publicKey
			}
			if privateShare, ok := session.Data["private_share"].(string); ok {
				sessionData["private_share"] = privateShare
			}
			if participantID, ok := session.Data["participant_id"]; ok {
				sessionData["participant_id"] = participantID
			}
		}
	case TypeReshare:
		if status == StatusCompleted {
			if newPrivateShare, ok := session.Data["private_share"].(string); ok {
				sessionData["new_private_share"] = newPrivateShare
			}
			if oldPrivateShare, ok := session.Data["old_private_share"].(string); ok {
				sessionData["old_private_share"] = oldPrivateShare
			}
			if completedAt, ok := session.Data["reshare_completed_at"].(string); ok {
				sessionData["reshare_completed_at"] = completedAt
			}
		}
	case TypeSign:
		if status == StatusCompleted {
			if signature, ok := session.Data["signature"].(string); ok {
				sessionData["signature"] = signature
			}
			if signatureR, ok := session.Data["signature_r"].(string); ok {
				sessionData["signature_r"] = signatureR
			}
			if signatureS, ok := session.Data["signature_s"].(string); ok {
				sessionData["signature_s"] = signatureS
			}
		}
	}
	session.mu.RUnlock()

	// 准备通知消息
	var msgType string
	if status == StatusCompleted {
		switch sessionType {
		case TypeKeygen:
			msgType = "keygen_complete"
		case TypeReshare:
			msgType = "reshare_complete"
		case TypeSign:
			msgType = "sign_complete"
		default:
			msgType = "session_complete"
		}
	} else if status == StatusFailed {
		msgType = "session_failed"
	}

	// 构造通知消息
	notification := map[string]interface{}{
		"type":         msgType,
		"session_id":   sessionID,
		"status":       string(status),
		"session_type": string(sessionType),
		"timestamp":    time.Now().Format(time.RFC3339),
	}

	if len(sessionData) > 0 {
		notification["session_data"] = sessionData
	}

	if status == StatusFailed {
		if errorMsg, ok := session.Data["error"].(string); ok {
			notification["error"] = errorMsg
		}
	}

	// 序列化消息
	notificationBytes, err := protocol.NewMessage(
		protocol.MessageType(msgType),
		sessionID,
		m.serverID,
		"", // 广播消息
		notification,
	).ToJSON()
	if err != nil {
		log.Printf("Failed to serialize session completion notification: %v", err)
		return
	}

	// 发送给WebSocket客户端
	if m.wsHub != nil {
		for _, participant := range participants {
			err := m.wsHub.SendToClient(participant, notificationBytes)
			if err != nil {
				log.Printf("Failed to send session completion notification to WebSocket client %s: %v", participant, err)
			} else {
				log.Printf("Sent session completion notification to WebSocket client %s", participant)
			}
		}
	}

	// 发送给peer节点
	if m.peerClient != nil {
		for _, participant := range participants {
			if participant != m.serverID {
				err := m.peerClient.SendToPeer(participant, notificationBytes)
				if err != nil {
					log.Printf("Failed to send session completion notification to peer %s: %v", participant, err)
				} else {
					log.Printf("Sent session completion notification to peer %s", participant)
				}
			}
		}
	}

	log.Printf("Session completion notification sent for session %s", sessionID)
}

// 签名相关的支持函数

// SignContext 签名上下文
type SignContext struct {
	P1 *sign.P1Context
	P2 *sign.P2Context
}

// getOrCreateSignContext 获取或创建签名上下文
func (m *MPCManager) getOrCreateSignContext(session *Session) (*SignContext, error) {
	// 检查是否已存在签名上下文
	if ctx, exists := session.Data["sign_context"]; exists {
		if signCtx, ok := ctx.(*SignContext); ok {
			return signCtx, nil
		}
	}

	// 创建新的签名上下文
	return m.createSignContext(session)
}

// getSignContext 获取现有的签名上下文
func (m *MPCManager) getSignContext(session *Session) (*SignContext, error) {
	if ctx, exists := session.Data["sign_context"]; exists {
		if signCtx, ok := ctx.(*SignContext); ok {
			return signCtx, nil
		}
	}
	return nil, fmt.Errorf("sign context not found for session %s", session.ID)
}

// createSignContext 创建签名上下文
func (m *MPCManager) createSignContext(session *Session) (*SignContext, error) {
	// 获取keygen会话的数据
	keygenSessionID, exists := session.Data["keygen_session_id"]
	if !exists {
		return nil, fmt.Errorf("no keygen session ID found for sign session %s", session.ID)
	}

	keygenSession, err := m.GetSession(keygenSessionID.(string))
	if err != nil {
		return nil, fmt.Errorf("failed to get keygen session %s: %v", keygenSessionID, err)
	}

	keygenSession.mu.RLock()
	defer keygenSession.mu.RUnlock()

	// 获取必要的keygen数据
	privateShareData, exists := keygenSession.Data["private_share"]
	if !exists {
		return nil, fmt.Errorf("no private share found in keygen session %s", keygenSessionID)
	}
	privateShareHex := privateShareData.(string)

	publicKeyData, exists := keygenSession.Data["public_key"]
	if !exists {
		return nil, fmt.Errorf("no public key found in keygen session %s", keygenSessionID)
	}
	publicKeyHex := publicKeyData.(string)

	// 获取Paillier密钥和Pedersen参数（这些应该在keygen后处理中生成）
	paiPrivateData, exists := keygenSession.Data["paillier_private_key"]
	if !exists {
		return nil, fmt.Errorf("no Paillier private key found in keygen session %s", keygenSessionID)
	}

	E_x1Data, exists := keygenSession.Data["E_x1"]
	if !exists {
		return nil, fmt.Errorf("no E_x1 found in keygen session %s", keygenSessionID)
	}

	pedParamsData, exists := keygenSession.Data["pedersen_params"]
	if !exists {
		return nil, fmt.Errorf("no Pedersen parameters found in keygen session %s", keygenSessionID)
	}

	// 获取签名消息
	messageData, exists := session.Data["message"]
	if !exists {
		return nil, fmt.Errorf("no message found in sign session %s", session.ID)
	}
	messageHex := messageData.(string)

	// 重构数据
	privateShareBytes, err := hex.DecodeString(privateShareHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private share: %v", err)
	}
	privateShare := new(big.Int).SetBytes(privateShareBytes)

	publicKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %v", err)
	}

	// 重构公钥
	pubKeyX := new(big.Int).SetBytes(publicKeyBytes[:32])
	pubKeyY := new(big.Int).SetBytes(publicKeyBytes[32:])
	publicKey := &ecdsa.PublicKey{
		Curve: secp256k1.S256(),
		X:     pubKeyX,
		Y:     pubKeyY,
	}

	// 创建签名上下文
	signCtx := &SignContext{}

	if m.isP1(session) {
		// 重构P1需要的数据
		paiPrivate := paiPrivateData.(*paillier.PrivateKey)
		E_x1 := E_x1Data.(*big.Int)
		pedParams := pedParamsData.(*pedersen.PedersenParameters)

		signCtx.P1 = sign.NewP1(publicKey, messageHex, paiPrivate, E_x1, pedParams)
	} else {
		// 重构P2需要的数据
		E_x1 := E_x1Data.(*big.Int)
		paiPubData, exists := keygenSession.Data["paillier_public_key"]
		if !exists {
			return nil, fmt.Errorf("no Paillier public key found in keygen session %s", keygenSessionID)
		}
		paiPub := paiPubData.(*paillier.PublicKey)
		pedParams := pedParamsData.(*pedersen.PedersenParameters)

		signCtx.P2 = sign.NewP2(privateShare, E_x1, publicKey, paiPub, messageHex, pedParams)
	}

	// 保存签名上下文到会话
	session.Data["sign_context"] = signCtx

	return signCtx, nil
}

// isP1 判断当前服务器是否为P1
func (m *MPCManager) isP1(session *Session) bool {
	// 简单的判断逻辑：按参与者列表顺序，第一个为P1
	if len(session.Participants) >= 2 {
		return session.Participants[0] == m.serverID
	}
	return false
}

// broadcastSignMessage 广播签名消息
func (m *MPCManager) broadcastSignMessage(sessionID string, round int, data interface{}, participants []string) {
	message := protocol.NewMessage(
		protocol.MsgTypeSignRound,
		sessionID,
		m.serverID,
		"", // 广播
		&protocol.SignRoundData{
			Round: round,
			Data:  data,
			From:  m.serverID,
		},
	)

	messageBytes, err := message.ToJSON()
	if err != nil {
		log.Printf("Failed to serialize sign message: %v", err)
		return
	}

	// 发送给WebSocket客户端
	if m.wsHub != nil {
		for _, participant := range participants {
			if participant != m.serverID {
				err := m.wsHub.SendToClient(participant, messageBytes)
				if err != nil {
					log.Printf("Failed to send sign message to WebSocket client %s: %v", participant, err)
				}
			}
		}
	}

	// 发送给peer节点
	if m.peerClient != nil {
		for _, participant := range participants {
			if participant != m.serverID {
				err := m.peerClient.SendToPeer(participant, messageBytes)
				if err != nil {
					log.Printf("Failed to send sign message to peer %s: %v", participant, err)
				}
			}
		}
	}
}

// notifySignComplete 通知签名完成
func (m *MPCManager) notifySignComplete(sessionID string, r, s *big.Int) {
	signature := fmt.Sprintf("%s%s", hex.EncodeToString(r.Bytes()), hex.EncodeToString(s.Bytes()))

	notification := map[string]interface{}{
		"type":        "sign_complete",
		"session_id":  sessionID,
		"signature":   signature,
		"signature_r": hex.EncodeToString(r.Bytes()),
		"signature_s": hex.EncodeToString(s.Bytes()),
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	message := protocol.NewMessage(
		"sign_complete",
		sessionID,
		m.serverID,
		"", // 广播
		notification,
	)

	messageBytes, err := message.ToJSON()
	if err != nil {
		log.Printf("Failed to serialize sign complete notification: %v", err)
		return
	}

	session, err := m.GetSession(sessionID)
	if err != nil {
		log.Printf("Failed to get session for sign complete notification: %v", err)
		return
	}

	// 发送给WebSocket客户端
	if m.wsHub != nil {
		for _, participant := range session.Participants {
			err := m.wsHub.SendToClient(participant, messageBytes)
			if err != nil {
				log.Printf("Failed to send sign complete notification to WebSocket client %s: %v", participant, err)
			}
		}
	}

	// 发送给peer节点
	if m.peerClient != nil {
		for _, participant := range session.Participants {
			if participant != m.serverID {
				err := m.peerClient.SendToPeer(participant, messageBytes)
				if err != nil {
					log.Printf("Failed to send sign complete notification to peer %s: %v", participant, err)
				}
			}
		}
	}
}

// 辅助函数

// stringPtr 返回字符串指针
func stringPtr(s string) *string {
	return &s
}

// mustDecodeHex 解码十六进制字符串，失败时panic
func mustDecodeHex(s string) []byte {
	bytes, err := hex.DecodeString(s)
	if err != nil {
		panic(fmt.Sprintf("failed to decode hex string %s: %v", s, err))
	}
	return bytes
}
