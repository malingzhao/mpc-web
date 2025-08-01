package protocol

import (
	"encoding/json"
	"time"
)

// MessageType 消息类型
type MessageType string

const (
	// 操作类型
	MsgTypeKeygenInit    MessageType = "keygen_init"
	MsgTypeKeygenRound1  MessageType = "keygen_round1"
	MsgTypeKeygenRound2  MessageType = "keygen_round2"
	MsgTypeKeygenRound3  MessageType = "keygen_round3"
	MsgTypeKeygenResult  MessageType = "keygen_result"
	MsgTypeReshareInit   MessageType = "reshare_init"
	MsgTypeReshareRound  MessageType = "reshare_round"
	MsgTypeReshareResult MessageType = "reshare_result"
	MsgTypeSignInit      MessageType = "sign_init"
	MsgTypeSignRound     MessageType = "sign_round"
	MsgTypeSignResult    MessageType = "sign_result"

	// 2方签名专用消息类型
	MsgTypeTwoPartySignInit   MessageType = "two_party_sign_init"
	MsgTypePreParams          MessageType = "pre_params"
	MsgTypePreParamsAck       MessageType = "pre_params_ack"
	MsgTypeKeygenP1Data       MessageType = "keygen_p1_data"
	MsgTypeKeygenP2Data       MessageType = "keygen_p2_data"
	MsgTypeTwoPartySignRound1 MessageType = "two_party_sign_round1"
	MsgTypeTwoPartySignRound2 MessageType = "two_party_sign_round2"
	MsgTypeTwoPartySignRound3 MessageType = "two_party_sign_round3"
	MsgTypeTwoPartySignResult MessageType = "two_party_sign_result"

	// 控制类型
	MsgTypeError       MessageType = "error"
	MsgTypeAck         MessageType = "ack"
	MsgTypeHeartbeat   MessageType = "heartbeat"
	MsgTypeSessionSync MessageType = "session_sync"
)

// Message WebSocket消息结构
type Message struct {
	Type      MessageType `json:"type"`
	SessionID string      `json:"session_id"`
	From      string      `json:"from"`
	To        string      `json:"to,omitempty"` // 空表示广播
	Round     int         `json:"round,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// KeygenInitData 密钥生成初始化数据
type KeygenInitData struct {
	Threshold    int      `json:"threshold"`
	Participants []string `json:"participants"`
}

// KeygenRoundData 密钥生成轮次数据
type KeygenRoundData struct {
	Round int         `json:"round"`
	Data  interface{} `json:"data"`
	From  string      `json:"from,omitempty"` // 发送者信息
}

// KeygenResultData 密钥生成结果数据
type KeygenResultData struct {
	Success   bool   `json:"success"`
	PublicKey string `json:"public_key,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ReshareInitData 密钥重分享初始化数据
type ReshareInitData struct {
	SessionID       string   `json:"session_id"`
	NewThreshold    int      `json:"new_threshold"`
	NewParticipants []string `json:"new_participants"`
}

// ReshareRoundData 密钥重分享轮次数据
type ReshareRoundData struct {
	Round int         `json:"round"`
	Data  interface{} `json:"data"`
}

// ReshareResultData 密钥重分享结果数据
type ReshareResultData struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// SignInitData 签名初始化数据
type SignInitData struct {
	SessionID string   `json:"session_id"`
	Message   string   `json:"message"`
	Signers   []string `json:"signers"`
}

// SignRoundData 签名轮次数据
type SignRoundData struct {
	Round int         `json:"round"`
	Data  interface{} `json:"data"`
	From  string      `json:"from,omitempty"` // 发送者信息
}

// ECDSA签名具体轮次数据结构

// SignRound1Data P1发送承诺，P2发送Schnorr证明
type SignRound1Data struct {
	// P1 -> P2: 承诺
	Commitment *CommitmentData `json:"commitment,omitempty"`
	// P2 -> P1: Schnorr证明和R2点
	SchnorrProof *SchnorrProofData `json:"schnorr_proof,omitempty"`
	R2Point      *ECPointData      `json:"r2_point,omitempty"`
}

// SignRound2Data P1发送Schnorr证明和承诺开启，P2发送仿射证明
type SignRound2Data struct {
	// P1 -> P2: Schnorr证明和承诺开启
	SchnorrProof      *SchnorrProofData `json:"schnorr_proof,omitempty"`
	CommitmentWitness *WitnessData      `json:"commitment_witness,omitempty"`
	// P2 -> P1: 加密值和仿射证明
	EncryptedValue *string          `json:"encrypted_value,omitempty"`
	AffineProof    *AffineProofData `json:"affine_proof,omitempty"`
}

// SignRound3Data P1计算最终签名
type SignRound3Data struct {
	// P1计算的最终签名
	R *string `json:"r,omitempty"`
	S *string `json:"s,omitempty"`
}

// 辅助数据结构
type CommitmentData struct {
	C string `json:"c"`
}

type WitnessData struct {
	SessionID string `json:"session_id"`
	X         string `json:"x"`
	Y         string `json:"y"`
}

type SchnorrProofData struct {
	E string `json:"e"`
	S string `json:"s"`
}

type ECPointData struct {
	X string `json:"x"`
	Y string `json:"y"`
}

type AffineProofData struct {
	X string `json:"x"`
	Y string `json:"y"`
	E string `json:"e"`
	S string `json:"s"`
}

// SignResultData 签名结果数据
type SignResultData struct {
	Success   bool   `json:"success"`
	Signature string `json:"signature,omitempty"`
	Error     string `json:"error,omitempty"`
}

// 2方签名专用数据结构

// TwoPartySignInitData 2方签名初始化数据
type TwoPartySignInitData struct {
	SignSessionID string `json:"sign_session_id"`
	KeygenID      string `json:"keygen_id"`
	Message       string `json:"message"`
	Initiator     string `json:"initiator"`
}

// PreParamsData 预参数数据
type PreParamsData struct {
	Params interface{} `json:"params"`
	Proof  interface{} `json:"proof"`
}

// KeygenP1Data P1密钥生成数据
type KeygenP1Data struct {
	P1Dto interface{} `json:"p1_dto"`
	E_x1  string      `json:"e_x1"`
}

// KeygenP2Data P2密钥生成数据
type KeygenP2Data struct {
	P2SaveData interface{} `json:"p2_save_data"`
}

// TwoPartySignRound1Data 2方签名第1轮数据
type TwoPartySignRound1Data struct {
	Commitment   *CommitmentData   `json:"commitment,omitempty"`
	SchnorrProof *SchnorrProofData `json:"schnorr_proof,omitempty"`
	R2Point      *ECPointData      `json:"r2_point,omitempty"`
}

// TwoPartySignRound2Data 2方签名第2轮数据
type TwoPartySignRound2Data struct {
	SchnorrProof      *SchnorrProofData `json:"schnorr_proof,omitempty"`
	CommitmentWitness *WitnessData      `json:"commitment_witness,omitempty"`
	EncryptedValue    *string           `json:"encrypted_value,omitempty"`
	AffineProof       *AffineProofData  `json:"affine_proof,omitempty"`
}

// TwoPartySignRound3Data 2方签名第3轮数据
type TwoPartySignRound3Data struct {
	R *string `json:"r,omitempty"`
	S *string `json:"s,omitempty"`
}

// TwoPartySignResultData 2方签名结果数据
type TwoPartySignResultData struct {
	Success   bool   `json:"success"`
	Signature string `json:"signature,omitempty"`
	R         string `json:"r,omitempty"`
	S         string `json:"s,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ErrorData 错误数据
type ErrorData struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// SessionSyncData 会话同步数据
type SessionSyncData struct {
	SessionID    string                 `json:"session_id"`
	Type         string                 `json:"type"`
	Status       string                 `json:"status"`
	Participants []string               `json:"participants"`
	Threshold    int                    `json:"threshold"`
	CurrentRound int                    `json:"current_round"`
	Data         map[string]interface{} `json:"data"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// NewMessage 创建新消息
func NewMessage(msgType MessageType, sessionID, from, to string, data interface{}) *Message {
	return &Message{
		Type:      msgType,
		SessionID: sessionID,
		From:      from,
		To:        to,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// ToJSON 转换为JSON
func (m *Message) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// FromJSON 从JSON解析
func FromJSON(data []byte) (*Message, error) {
	var msg Message
	err := json.Unmarshal(data, &msg)
	return &msg, err
}
