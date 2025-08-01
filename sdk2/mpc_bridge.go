package main

/*
#include <stdlib.h>
#include <string.h>

typedef struct {
    int from;
    int to;
    char* data;
    int data_len;
} MPCMessage;

typedef struct {
    MPCMessage* messages;
    int count;
} MPCMessageArray;

typedef struct {
    char* r_str;
    char* s_str;
    int error_code;
} SignatureResult;
*/
import "C"
import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"sync"
	"unsafe"

	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/decred/dcrd/dcrec/secp256k1/v2"
	"github.com/okx/threshold-lib/crypto/commitment"
	"github.com/okx/threshold-lib/crypto/curves"
	"github.com/okx/threshold-lib/crypto/paillier"
	"github.com/okx/threshold-lib/crypto/pedersen"
	"github.com/okx/threshold-lib/crypto/schnorr"
	"github.com/okx/threshold-lib/crypto/zkp"
	"github.com/okx/threshold-lib/tss"
	"github.com/okx/threshold-lib/tss/ecdsa/keygen"
	"github.com/okx/threshold-lib/tss/ecdsa/sign"
	ed25519Sign "github.com/okx/threshold-lib/tss/ed25519/sign"
	"github.com/okx/threshold-lib/tss/key/dkg"
	"github.com/okx/threshold-lib/tss/key/reshare"
)

// 完整的ECDSA签名所需数据结构
type ECDSASignData struct {
	// DKG阶段的数据
	KeyStep3Data tss.KeyStep3Data `json:"key_step3_data"`

	// P1特有的数据
	PaiPrivate *paillier.PrivateKey         `json:"pai_private,omitempty"`
	E_x1       *big.Int                     `json:"e_x1,omitempty"`
	P1Ped      *pedersen.PedersenParameters `json:"p1_ped,omitempty"`

	// P2特有的数据 (来自P2SaveData)
	P2SaveData *keygen.P2SaveData `json:"p2_save_data,omitempty"`
}

// 会话管理
type MPCSession struct {
	Handle      unsafe.Pointer
	SessionType string
	Type        string      // 新增：会话类型
	Context     interface{} // 新增：会话上下文
}

// ECDSA 签名会话数据
type ECDSASignSession struct {
	P1Context *sign.P1Context
	P2Context *sign.P2Context
	PartyID   int
	PeerID    int
	IsP1      bool
}

var (
	sessions      = make(map[int]*MPCSession)
	sessionMutex  sync.RWMutex
	nextSessionID = 1
)

// 添加会话
func addSession(handle unsafe.Pointer, sessionType string) int {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	sessionID := nextSessionID
	nextSessionID++
	sessions[sessionID] = &MPCSession{
		Handle:      handle,
		SessionType: sessionType,
		Context:     handle, // 将 handle 也存储在 Context 中
	}
	return sessionID
}

// 添加会话（带上下文）
func addSessionWithContext(context interface{}, sessionType string) int {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	sessionID := nextSessionID
	nextSessionID++
	sessions[sessionID] = &MPCSession{
		Handle:      unsafe.Pointer(&context),
		SessionType: sessionType,
		Context:     context,
	}
	return sessionID
}

// 获取会话
func getSession(id int) *MPCSession {
	sessionMutex.RLock()
	defer sessionMutex.RUnlock()
	return sessions[id]
}

// 删除会话
func removeSession(id int) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	delete(sessions, id)
}

// 消息转换函数
func convertMessagesToC(messages map[int]*tss.Message) *C.MPCMessageArray {
	if len(messages) == 0 {
		return nil
	}

	// 分配C数组
	cMessages := (*C.MPCMessage)(C.malloc(C.size_t(len(messages)) * C.sizeof_MPCMessage))
	cArray := (*[1 << 30]C.MPCMessage)(unsafe.Pointer(cMessages))[:len(messages):len(messages)]

	i := 0
	for _, msg := range messages {
		cArray[i].from = C.int(msg.From)
		cArray[i].to = C.int(msg.To)
		cArray[i].data = C.CString(msg.Data)
		cArray[i].data_len = C.int(len(msg.Data))
		i++
	}

	result := (*C.MPCMessageArray)(C.malloc(C.sizeof_MPCMessageArray))
	result.messages = cMessages
	result.count = C.int(len(messages))

	return result
}

func convertMessagesFromC(cArray *C.MPCMessageArray) []*tss.Message {
	if cArray == nil || cArray.count == 0 {
		return nil
	}

	count := int(cArray.count)
	cMessages := (*[1 << 30]C.MPCMessage)(unsafe.Pointer(cArray.messages))[:count:count]

	messages := make([]*tss.Message, count)
	for i := 0; i < count; i++ {
		messages[i] = &tss.Message{
			From: int(cMessages[i].from),
			To:   int(cMessages[i].to),
			Data: C.GoStringN(cMessages[i].data, cMessages[i].data_len),
		}
	}

	return messages
}

// ================================
// 密钥生成相关函数
// ================================

//export go_keygen_init
func go_keygen_init(curve int, partyID int, threshold int, totalParties int, handle *unsafe.Pointer) C.int {
	var curveType elliptic.Curve
	if curve == 0 {
		curveType = secp256k1.S256()
	} else {
		curveType = edwards.Edwards()
	}

	setUp := dkg.NewSetUp(partyID, totalParties, curveType)
	sessionID := addSession(unsafe.Pointer(setUp), "keygen")
	*handle = unsafe.Pointer(uintptr(sessionID))

	return 0 // MPC_SUCCESS
}

//export go_keygen_round1
func go_keygen_round1(handle unsafe.Pointer, outData **C.char, outLen *C.int) C.int {
	sessionID := int(uintptr(handle))
	session := getSession(sessionID)
	if session == nil || session.SessionType != "keygen" {
		return -1 // MPC_ERROR_INVALID_PARAM
	}

	setUp := (*dkg.SetupInfo)(session.Handle)

	msgs, err := setUp.DKGStep1()
	if err != nil {
		return -3 // MPC_ERROR_CRYPTO
	}

	// 序列化消息
	data, err := json.Marshal(msgs)
	if err != nil {
		return -3
	}

	// 分配C字符串
	cStr := C.CString(string(data))
	*outData = cStr
	*outLen = C.int(len(data))

	return 0
}

//export go_keygen_round2
func go_keygen_round2(handle unsafe.Pointer, inData *C.char, inLen C.int, outData **C.char, outLen *C.int) C.int {
	sessionID := int(uintptr(handle))
	session := getSession(sessionID)
	if session == nil || session.SessionType != "keygen" {
		return -1
	}

	setUp := (*dkg.SetupInfo)(session.Handle)

	// 反序列化输入消息
	goData := C.GoStringN(inData, inLen)
	var msgs []*tss.Message
	err := json.Unmarshal([]byte(goData), &msgs)
	if err != nil {
		return -3
	}

	// 执行第二轮
	outMsgs, err := setUp.DKGStep2(msgs)
	if err != nil {
		return -3
	}

	// 序列化输出消息
	data, err := json.Marshal(outMsgs)
	if err != nil {
		return -3
	}

	cStr := C.CString(string(data))
	*outData = cStr
	*outLen = C.int(len(data))

	return 0
}

//export go_keygen_round3
func go_keygen_round3(handle unsafe.Pointer, inData *C.char, inLen C.int, keyData **C.char, keyLen *C.int) C.int {
	sessionID := int(uintptr(handle))
	session := getSession(sessionID)
	if session == nil || session.SessionType != "keygen" {
		return -1
	}

	setUp := (*dkg.SetupInfo)(session.Handle)

	// 反序列化输入消息
	goData := C.GoStringN(inData, inLen)
	var msgs []*tss.Message
	err := json.Unmarshal([]byte(goData), &msgs)
	if err != nil {
		return -3
	}

	// 执行第三轮
	saveData, err := setUp.DKGStep3(msgs)
	if err != nil {
		return -3
	}

	// 序列化密钥数据
	data, err := json.Marshal(saveData)
	if err != nil {
		return -3
	}

	cStr := C.CString(string(data))
	*keyData = cStr
	*keyLen = C.int(len(data))

	return 0
}

//export go_keygen_destroy
func go_keygen_destroy(handle unsafe.Pointer) {
	sessionID := int(uintptr(handle))
	removeSession(sessionID)
}

// ================================
// 密钥刷新相关函数
// ================================

//export go_refresh_init
func go_refresh_init(curve int, partyID int, threshold int, devoteList *C.int, devoteCount C.int, keyData *C.char, keyLen C.int, handle *unsafe.Pointer) C.int {
	// 转换参数
	goPartyID := int(partyID)
	goDevoteCount := int(devoteCount)

	// 转换devoteList - NewRefresh需要[2]int类型
	devoteSlice := (*[1 << 30]C.int)(unsafe.Pointer(devoteList))[:goDevoteCount:goDevoteCount]
	var goDevoteList [2]int
	for i := 0; i < goDevoteCount && i < 2; i++ {
		goDevoteList[i] = int(devoteSlice[i])
	}
	// 如果只有一个元素，第二个元素设为0
	if goDevoteCount == 1 {
		goDevoteList[1] = 0
	}

	// 确定曲线类型
	var curveType elliptic.Curve
	if curve == 0 {
		curveType = secp256k1.S256()
	} else {
		curveType = edwards.Edwards()
	}

	// 解析密钥数据 - 必须成功解析真实的keygen数据
	goKeyData := C.GoStringN(keyData, keyLen)
	var saveData tss.KeyStep3Data
	err := json.Unmarshal([]byte(goKeyData), &saveData)
	if err != nil {
		// JSON解析失败，返回错误而不是使用模拟数据
		return -2
	}

	// 检查必要的数据是否存在
	if saveData.ShareI == nil {
		return -3
	}

	if saveData.PublicKey == nil {
		return -4
	}

	// 创建refresh实例 (假设总共3个参与方)
	totalParties := 3
	refresh := reshare.NewRefresh(goPartyID, totalParties, goDevoteList, saveData.ShareI, saveData.PublicKey)
	if refresh == nil {
		return -5
	}

	// 避免编译器警告
	_ = curveType

	sessionID := addSession(unsafe.Pointer(refresh), "refresh")
	*handle = unsafe.Pointer(uintptr(sessionID))

	return 0
}

//export go_refresh_round1
func go_refresh_round1(handle unsafe.Pointer, outData **C.char, outLen *C.int) C.int {
	sessionID := int(uintptr(handle))
	session := getSession(sessionID)
	if session == nil || session.SessionType != "refresh" {
		return -1
	}

	refresh := (*reshare.RefreshInfo)(session.Handle)

	msgs, err := refresh.DKGStep1()
	if err != nil {
		return -3
	}

	data, err := json.Marshal(msgs)
	if err != nil {
		return -3
	}

	cStr := C.CString(string(data))
	*outData = cStr
	*outLen = C.int(len(data))

	return 0
}

//export go_refresh_round2
func go_refresh_round2(handle unsafe.Pointer, inData *C.char, inLen C.int, outData **C.char, outLen *C.int) C.int {
	sessionID := int(uintptr(handle))
	session := getSession(sessionID)
	if session == nil || session.SessionType != "refresh" {
		return -1
	}

	refresh := (*reshare.RefreshInfo)(session.Handle)

	goData := C.GoStringN(inData, inLen)
	var msgs []*tss.Message
	err := json.Unmarshal([]byte(goData), &msgs)
	if err != nil {
		return -3
	}

	outMsgs, err := refresh.DKGStep2(msgs)
	if err != nil {
		return -3
	}

	data, err := json.Marshal(outMsgs)
	if err != nil {
		return -3
	}

	cStr := C.CString(string(data))
	*outData = cStr
	*outLen = C.int(len(data))

	return 0
}

//export go_refresh_round3
func go_refresh_round3(handle unsafe.Pointer, inData *C.char, inLen C.int, keyData **C.char, keyLen *C.int) C.int {
	sessionID := int(uintptr(handle))
	session := getSession(sessionID)
	if session == nil || session.SessionType != "refresh" {
		return -1
	}

	refresh := (*reshare.RefreshInfo)(session.Handle)

	goData := C.GoStringN(inData, inLen)
	var msgs []*tss.Message
	err := json.Unmarshal([]byte(goData), &msgs)
	if err != nil {
		// JSON parsing error
		// 写入调试文件
		if f, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			fmt.Fprintf(f, "DEBUG: JSON parsing error in round3: %v\n", err)
			f.Close()
		}
		return -2
	}

	// 写入调试文件
	if f, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		fmt.Fprintf(f, "DEBUG: Round3 received %d messages\n", len(msgs))
		for i, msg := range msgs {
			fmt.Fprintf(f, "DEBUG: Message %d: From=%d, To=%d, DataLen=%d\n", i, msg.From, msg.To, len(msg.Data))
		}
		f.Close()
	}

	saveData, err := refresh.DKGStep3(msgs)
	if err != nil {
		// DKGStep3 error - this is where the actual protocol error occurs
		// 写入调试文件
		if f, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			fmt.Fprintf(f, "DEBUG: DKGStep3 error: %v\n", err)
			f.Close()
		}
		return -3
	}

	data, err := json.Marshal(saveData)
	if err != nil {
		// Result serialization error
		// 写入调试文件
		if f, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			fmt.Fprintf(f, "DEBUG: Result serialization error: %v\n", err)
			f.Close()
		}
		return -4
	}

	cStr := C.CString(string(data))
	*keyData = cStr
	*keyLen = C.int(len(data))

	return 0
}

//export go_refresh_destroy
func go_refresh_destroy(handle unsafe.Pointer) {
	sessionID := int(uintptr(handle))
	removeSession(sessionID)
}

// ================================
// ECDSA签名相关函数 - 简化版本
// ================================

// ================================
// ECDSA签名相关函数 - 复杂版本 (保持兼容性)
// ================================

//export go_ecdsa_sign_init_p1_complex
func go_ecdsa_sign_init_p1_complex(partyID C.int, peerID C.int, keyData *C.char, keyLen C.int, message *C.char, messageLen C.int, handle *unsafe.Pointer) C.int {
	goPartyID := int(partyID)
	goPeerID := int(peerID)

	// 解析完整的ECDSA签名数据
	goKeyData := C.GoStringN(keyData, keyLen)
	var ecdsaSignData ECDSASignData
	err := json.Unmarshal([]byte(goKeyData), &ecdsaSignData)
	if err != nil {
		return -1 // 密钥数据解析失败
	}

	// 验证P1所需的数据
	if ecdsaSignData.PaiPrivate == nil || ecdsaSignData.E_x1 == nil || ecdsaSignData.P1Ped == nil {
		return -2 // P1缺少必要的keygen数据
	}

	// 解析消息
	goMessage := C.GoStringN(message, messageLen)

	// 创建公钥 - 使用secp256k1曲线
	curve := secp256k1.S256()
	pubKey := &ecdsa.PublicKey{
		Curve: curve,
		X:     ecdsaSignData.KeyStep3Data.PublicKey.X,
		Y:     ecdsaSignData.KeyStep3Data.PublicKey.Y,
	}

	// 使用完整的ECDSA签名数据创建P1签名上下文，使用保存的Pedersen参数
	p1Context := sign.NewP1(pubKey, goMessage, ecdsaSignData.PaiPrivate, ecdsaSignData.E_x1, ecdsaSignData.P1Ped)
	if p1Context == nil {
		return -3
	}

	// 创建签名会话
	signSession := &ECDSASignSession{
		P1Context: p1Context,
		P2Context: nil,
		PartyID:   goPartyID,
		PeerID:    goPeerID,
		IsP1:      true,
	}

	sessionID := addSession(unsafe.Pointer(signSession), "ecdsa_sign")
	*handle = unsafe.Pointer(uintptr(sessionID))

	return 0
}

//export go_ecdsa_sign_init_p2_complex
func go_ecdsa_sign_init_p2_complex(partyID C.int, peerID C.int, keyData *C.char, keyLen C.int, message *C.char, messageLen C.int, handle *unsafe.Pointer) C.int {
	goPartyID := int(partyID)
	goPeerID := int(peerID)

	// 解析完整的ECDSA签名数据
	goKeyData := C.GoStringN(keyData, keyLen)
	var ecdsaSignData ECDSASignData
	err := json.Unmarshal([]byte(goKeyData), &ecdsaSignData)
	if err != nil {
		return -1 // 密钥数据解析失败
	}

	// 验证P2所需的数据
	if ecdsaSignData.P2SaveData == nil {
		return -2 // P2缺少必要的keygen数据
	}

	// 解析消息
	goMessage := C.GoStringN(message, messageLen)

	// 创建公钥 - 使用secp256k1曲线
	curve := secp256k1.S256()
	pubKey := &ecdsa.PublicKey{
		Curve: curve,
		X:     ecdsaSignData.KeyStep3Data.PublicKey.X,
		Y:     ecdsaSignData.KeyStep3Data.PublicKey.Y,
	}

	// 从P2SaveData获取所需参数
	p2SaveData := ecdsaSignData.P2SaveData

	// 创建 P2 签名上下文，使用P2SaveData中的正确参数
	p2Context := sign.NewP2(p2SaveData.X2, p2SaveData.E_x1, pubKey, p2SaveData.PaiPubKey, goMessage, p2SaveData.Ped1)
	if p2Context == nil {
		return -3
	}

	// 创建签名会话
	signSession := &ECDSASignSession{
		P1Context: nil,
		P2Context: p2Context,
		PartyID:   goPartyID,
		PeerID:    goPeerID,
		IsP1:      false,
	}

	sessionID := addSession(unsafe.Pointer(signSession), "ecdsa_sign")
	*handle = unsafe.Pointer(uintptr(sessionID))

	return 0
}

//export go_ecdsa_sign_step1
func go_ecdsa_sign_step1(handle unsafe.Pointer, commitData **C.char, commitLen *C.int) C.int {
	sessionID := int(uintptr(handle))
	session := getSession(sessionID)
	if session == nil || session.SessionType != "ecdsa_sign" {
		return -1
	}

	signSession := (*ECDSASignSession)(session.Handle)

	if signSession.IsP1 {
		// P1执行Step1 - 生成承诺
		commit, err := signSession.P1Context.Step1()
		if err != nil {
			return -2
		}

		// 序列化承诺数据
		commitBytes, err := json.Marshal(commit)
		if err != nil {
			return -3
		}

		*commitData = C.CString(string(commitBytes))
		*commitLen = C.int(len(commitBytes))
	} else {
		return -4 // P2不应该调用这个函数
	}

	return 0
}

//export go_ecdsa_sign_p2_step1
func go_ecdsa_sign_p2_step1(handle unsafe.Pointer, commitData *C.char, commitLen C.int, proofData **C.char, proofLen *C.int, r2Data **C.char, r2Len *C.int) C.int {
	sessionID := int(uintptr(handle))
	session := getSession(sessionID)
	if session == nil || session.SessionType != "ecdsa_sign" {
		return -1
	}

	signSession := (*ECDSASignSession)(session.Handle)

	if !signSession.IsP1 {
		// 解析P1的承诺
		goCommitData := C.GoStringN(commitData, commitLen)
		var commit commitment.Commitment
		err := json.Unmarshal([]byte(goCommitData), &commit)
		if err != nil {
			return -2
		}

		// P2执行Step1 - 生成Schnorr证明和R2点
		bobProof, R2, err := signSession.P2Context.Step1(&commit)
		if err != nil {
			return -3
		}

		// 序列化Schnorr证明
		proofBytes, err := json.Marshal(bobProof)
		if err != nil {
			return -4
		}

		// 序列化R2点
		r2Bytes, err := json.Marshal(R2)
		if err != nil {
			return -5
		}

		*proofData = C.CString(string(proofBytes))
		*proofLen = C.int(len(proofBytes))
		*r2Data = C.CString(string(r2Bytes))
		*r2Len = C.int(len(r2Bytes))
	} else {
		return -6 // P1不应该调用这个函数
	}

	return 0
}

//export go_ecdsa_sign_p1_step2
func go_ecdsa_sign_p1_step2(handle unsafe.Pointer, proofData *C.char, proofLen C.int, r2Data *C.char, r2Len C.int, p1ProofData **C.char, p1ProofLen *C.int, cmtDData **C.char, cmtDLen *C.int) C.int {
	sessionID := int(uintptr(handle))
	session := getSession(sessionID)
	if session == nil || session.SessionType != "ecdsa_sign" {
		return -1
	}

	signSession := (*ECDSASignSession)(session.Handle)

	if signSession.IsP1 {
		// 反序列化P2的Schnorr证明
		goProofData := C.GoStringN(proofData, proofLen)
		var bobProof schnorr.Proof
		err := json.Unmarshal([]byte(goProofData), &bobProof)
		if err != nil {
			return -2
		}

		// 反序列化R2点
		goR2Data := C.GoStringN(r2Data, r2Len)
		var R2 curves.ECPoint
		err = json.Unmarshal([]byte(goR2Data), &R2)
		if err != nil {
			return -3
		}

		// P1执行Step2 - 验证P2的证明并生成自己的证明
		proof, cmtD, err := signSession.P1Context.Step2(&bobProof, &R2)
		if err != nil {
			return -4
		}

		// 序列化P1的证明
		p1ProofBytes, err := json.Marshal(proof)
		if err != nil {
			return -5
		}

		// 序列化承诺见证
		cmtDBytes, err := json.Marshal(cmtD)
		if err != nil {
			return -6
		}

		*p1ProofData = C.CString(string(p1ProofBytes))
		*p1ProofLen = C.int(len(p1ProofBytes))
		*cmtDData = C.CString(string(cmtDBytes))
		*cmtDLen = C.int(len(cmtDBytes))
	} else {
		return -7 // P2不应该调用这个函数
	}

	return 0
}

//export go_ecdsa_sign_p2_step2
func go_ecdsa_sign_p2_step2(handle unsafe.Pointer, cmtDData *C.char, cmtDLen C.int, p1ProofData *C.char, p1ProofLen C.int, ekData **C.char, ekLen *C.int, affineProofData **C.char, affineProofLen *C.int) C.int {
	sessionID := int(uintptr(handle))
	session := getSession(sessionID)
	if session == nil || session.SessionType != "ecdsa_sign" {
		return -1
	}

	signSession := (*ECDSASignSession)(session.Handle)

	if !signSession.IsP1 {
		// 反序列化承诺见证
		goCmtDData := C.GoStringN(cmtDData, cmtDLen)
		var cmtD commitment.Witness
		err := json.Unmarshal([]byte(goCmtDData), &cmtD)
		if err != nil {
			return -2
		}

		// 反序列化P1的证明
		goP1ProofData := C.GoStringN(p1ProofData, p1ProofLen)
		var p1Proof schnorr.Proof
		err = json.Unmarshal([]byte(goP1ProofData), &p1Proof)
		if err != nil {
			return -3
		}

		// P2执行Step2 - 生成加密数据和仿射证明
		E_k2_h_xr, affine_proof, err := signSession.P2Context.Step2(&cmtD, &p1Proof)
		if err != nil {
			return -4
		}

		// 序列化加密数据
		ekBytes, err := json.Marshal(E_k2_h_xr)
		if err != nil {
			return -5
		}

		// 序列化仿射证明
		affineProofBytes, err := json.Marshal(affine_proof)
		if err != nil {
			return -6
		}

		*ekData = C.CString(string(ekBytes))
		*ekLen = C.int(len(ekBytes))
		*affineProofData = C.CString(string(affineProofBytes))
		*affineProofLen = C.int(len(affineProofBytes))
	} else {
		return -7 // P1不应该调用这个函数
	}

	return 0
}

//export go_ecdsa_sign_p1_step3
func go_ecdsa_sign_p1_step3(handle unsafe.Pointer, ekData *C.char, ekLen C.int, affineProofData *C.char, affineProofLen C.int, rData **C.char, rLen *C.int, sData **C.char, sLen *C.int) C.int {
	sessionID := int(uintptr(handle))
	session := getSession(sessionID)
	if session == nil || session.SessionType != "ecdsa_sign" {
		return -1
	}

	signSession := (*ECDSASignSession)(session.Handle)

	if signSession.IsP1 {
		// 反序列化加密数据
		goEkData := C.GoStringN(ekData, ekLen)
		var E_k2_h_xr big.Int
		err := json.Unmarshal([]byte(goEkData), &E_k2_h_xr)
		if err != nil {
			return -2
		}

		// 反序列化仿射证明
		goAffineProofData := C.GoStringN(affineProofData, affineProofLen)
		var affine_proof zkp.AffGProof
		err = json.Unmarshal([]byte(goAffineProofData), &affine_proof)
		if err != nil {
			return -3
		}

		// P1执行Step3 - 生成最终签名
		r, s, err := signSession.P1Context.Step3(&E_k2_h_xr, &affine_proof)
		if err != nil {
			return -4
		}

		// 序列化r和s
		rBytes, err := json.Marshal(r)
		if err != nil {
			return -5
		}

		sBytes, err := json.Marshal(s)
		if err != nil {
			return -6
		}

		*rData = C.CString(string(rBytes))
		*rLen = C.int(len(rBytes))
		*sData = C.CString(string(sBytes))
		*sLen = C.int(len(sBytes))
	} else {
		return -7 // P2不应该调用这个函数
	}

	return 0
}

//export go_ecdsa_sign_destroy
func go_ecdsa_sign_destroy(handle unsafe.Pointer) {
	sessionID := int(uintptr(handle))
	removeSession(sessionID)
}

// ================================
// Ed25519签名相关函数
// ================================

//export go_ed25519_sign_init
func go_ed25519_sign_init(party_id C.int, threshold C.int, part_list *C.int, part_count C.int, key_data *C.char, key_len C.int, message *C.char, message_len C.int, handle *unsafe.Pointer) C.int {
	// Convert C parameters to Go
	partyID := int(party_id)
	thresh := int(threshold)
	partCount := int(part_count)

	// Convert part_list from C array to Go slice
	if part_count <= 0 || part_count > 100 { // 安全检查
		return -1
	}

	partListSlice := (*[100]C.int)(unsafe.Pointer(part_list))[:partCount:partCount]
	partList := make([]int, partCount)
	for i := 0; i < partCount; i++ {
		partList[i] = int(partListSlice[i])
	}

	// Convert key data and message
	keyDataStr := C.GoStringN(key_data, key_len)
	messageStr := C.GoStringN(message, message_len)

	// Parse key data (JSON format from DKG)
	var keyStep3Data tss.KeyStep3Data
	err := json.Unmarshal([]byte(keyDataStr), &keyStep3Data)
	if err != nil {
		return -2 // DKG密钥解析错误
	}

	// Create Ed25519 public key
	publicKey := edwards.NewPublicKey(keyStep3Data.PublicKey.X, keyStep3Data.PublicKey.Y)

	// Create Ed25519 sign instance
	ed25519SignInstance := ed25519Sign.NewEd25519Sign(partyID, thresh, partList, keyStep3Data.ShareI, publicKey, messageStr)
	if ed25519SignInstance == nil {
		return -6
	}

	// Store session and return handle
	sessionID := addSessionWithContext(ed25519SignInstance, "ed25519_sign")
	*handle = unsafe.Pointer(uintptr(sessionID))
	return 0
}

//export go_ed25519_sign_round1
func go_ed25519_sign_round1(handle unsafe.Pointer, out_data **C.char, out_len *C.int) C.int {
	sessionID := int(uintptr(handle))
	session := getSession(sessionID)
	if session == nil || session.SessionType != "ed25519_sign" {
		return -1
	}

	// 获取Ed25519签名实例
	ed25519SignInstance := session.Context.(*ed25519Sign.Ed25519Sign)
	if ed25519SignInstance == nil {
		return -2
	}

	// 执行SignStep1
	messages, err := ed25519SignInstance.SignStep1()
	if err != nil {
		return -3
	}

	// 序列化消息
	data, err := json.Marshal(messages)
	if err != nil {
		return -4
	}

	cStr := C.CString(string(data))
	*out_data = cStr
	*out_len = C.int(len(data))

	return 0
}

//export go_ed25519_sign_round2
func go_ed25519_sign_round2(handle unsafe.Pointer, in_data *C.char, in_len C.int, out_data **C.char, out_len *C.int) C.int {
	sessionID := int(uintptr(handle))
	session := getSession(sessionID)
	if session == nil || session.SessionType != "ed25519_sign" {
		return -1
	}

	// 获取Ed25519签名实例
	ed25519SignInstance := session.Context.(*ed25519Sign.Ed25519Sign)
	if ed25519SignInstance == nil {
		return -2
	}

	// 解析输入消息
	inDataStr := C.GoStringN(in_data, in_len)
	var inMessages []*tss.Message
	err := json.Unmarshal([]byte(inDataStr), &inMessages)
	if err != nil {
		return -3
	}

	// 执行SignStep2
	messages, err := ed25519SignInstance.SignStep2(inMessages)
	if err != nil {
		return -4
	}

	// 序列化输出消息
	data, err := json.Marshal(messages)
	if err != nil {
		return -5
	}

	cStr := C.CString(string(data))
	*out_data = cStr
	*out_len = C.int(len(data))

	return 0
}

//export go_ed25519_sign_round3
func go_ed25519_sign_round3(handle unsafe.Pointer, in_data *C.char, in_len C.int, sig_r **C.char, sig_s **C.char) C.int {
	sessionID := int(uintptr(handle))
	session := getSession(sessionID)
	if session == nil || session.SessionType != "ed25519_sign" {
		return -1
	}

	// 获取Ed25519签名实例
	ed25519SignInstance := session.Context.(*ed25519Sign.Ed25519Sign)
	if ed25519SignInstance == nil {
		return -2
	}

	// 解析输入消息
	inDataStr := C.GoStringN(in_data, in_len)
	var inMessages []*tss.Message
	err := json.Unmarshal([]byte(inDataStr), &inMessages)
	if err != nil {
		return -3
	}

	// 执行SignStep3
	si, r, err := ed25519SignInstance.SignStep3(inMessages)
	if err != nil {
		return -4
	}

	// 转换为字符串
	rStr := r.String()
	sStr := si.String()

	rCStr := C.CString(rStr)
	sCStr := C.CString(sStr)

	*sig_r = rCStr
	*sig_s = sCStr

	return 0
}

//export go_ed25519_sign_destroy
func go_ed25519_sign_destroy(handle unsafe.Pointer) {
	sessionID := int(uintptr(handle))
	removeSession(sessionID)
}

// ================================
// 辅助函数
// ================================

//export mpc_string_alloc
func mpc_string_alloc(src *C.char) *C.char {
	if src == nil {
		return nil
	}
	goStr := C.GoString(src)
	return C.CString(goStr)
}

//export mpc_string_free
func mpc_string_free(str *C.char) {
	if str != nil {
		C.free(unsafe.Pointer(str))
	}
}

//export mpc_message_array_alloc
func mpc_message_array_alloc(count C.int) unsafe.Pointer {
	if count <= 0 {
		return nil
	}

	// 分配消息数组结构体
	intSize := unsafe.Sizeof(C.int(0))
	ptrSize := unsafe.Sizeof(uintptr(0))
	size := intSize + uintptr(count)*ptrSize
	ptr := C.malloc(C.size_t(size))
	if ptr == nil {
		return nil
	}

	// 设置count
	countPtr := (*C.int)(ptr)
	*countPtr = count

	// 分配消息指针数组
	messagesPtr := unsafe.Pointer(uintptr(ptr) + intSize)

	// 初始化消息数组为零
	for i := 0; i < int(count); i++ {
		msgPtr := unsafe.Pointer(uintptr(messagesPtr) + uintptr(i)*ptrSize)
		*(*uintptr)(msgPtr) = 0
	}

	return ptr
}

//export mpc_message_array_free
func mpc_message_array_free(messages unsafe.Pointer) {
	if messages != nil {
		C.free(messages)
	}
}

//export mpc_get_error_string
func mpc_get_error_string(error_code C.int) *C.char {
	switch int(error_code) {
	case 0:
		return C.CString("Success")
	case -1:
		return C.CString("Invalid parameter")
	case -2:
		return C.CString("Memory error")
	case -3:
		return C.CString("Crypto error")
	case -4:
		return C.CString("Network error")
	case -5:
		return C.CString("Timeout error")
	default:
		return C.CString("Unknown error")
	}
}

// ================================
// ECDSA Keygen 相关函数
// ================================

//export go_ecdsa_keygen_generate_p2_params
func go_ecdsa_keygen_generate_p2_params(out_data **C.char, out_len *C.int) C.int {
	// P2生成自己的预参数和证明
	p2PreParamsAndProof := keygen.GeneratePreParamsWithDlnProof()

	// 序列化P2预参数
	p2ParamsData, err := json.Marshal(p2PreParamsAndProof)
	if err != nil {
		return -1 // 序列化失败
	}

	// 返回数据
	*out_data = C.CString(string(p2ParamsData))
	*out_len = C.int(len(p2ParamsData))

	return 0 // 成功
}

//export go_ecdsa_keygen_p1
func go_ecdsa_keygen_p1(key_data *C.char, key_len C.int, peer_id C.int, p2_params *C.char, p2_params_len C.int, out_data **C.char, out_len *C.int, message_data **C.char, message_len *C.int) C.int {
	keyDataStr := C.GoStringN(key_data, key_len)
	p2ParamsStr := C.GoStringN(p2_params, p2_params_len)

	// 解析DKG密钥数据
	var keyStep3Data tss.KeyStep3Data
	err := json.Unmarshal([]byte(keyDataStr), &keyStep3Data)
	if err != nil {
		return -1 // DKG密钥解析错误
	}

	// 解析P2预参数
	var p2PreParamsAndProof keygen.PreParamsWithDlnProof
	err = json.Unmarshal([]byte(p2ParamsStr), &p2PreParamsAndProof)
	if err != nil {
		return -2 // P2预参数解析错误
	}

	// 从DKG数据中提取必要信息
	share1 := keyStep3Data.ShareI

	// 生成Paillier密钥对
	paiPrivateKey, _, err := paillier.NewKeyPair(8)
	if err != nil {
		return -3 // Paillier密钥生成失败
	}

	// P1生成自己的预参数和证明
	p1PreParamsAndProof := keygen.GeneratePreParamsWithDlnProof()

	// 执行P1 keygen，使用P2的预参数
	message, e_x1, err := keygen.P1(share1, paiPrivateKey, keyStep3Data.Id, int(peer_id), p1PreParamsAndProof, p2PreParamsAndProof.PedersonParameters(), p2PreParamsAndProof.Proof)
	if err != nil {
		return -4 // P1 keygen执行失败
	}

	// 创建完整的ECDSA签名数据
	ecdsaSignData := ECDSASignData{
		KeyStep3Data: keyStep3Data,
		PaiPrivate:   paiPrivateKey,
		E_x1:         e_x1,
		P1Ped:        p1PreParamsAndProof.PedersonParameters(),
		P2SaveData:   nil, // P1不需要P2SaveData
	}

	// 序列化签名数据
	signData, err := json.Marshal(ecdsaSignData)
	if err != nil {
		return -5 // 签名数据序列化失败
	}

	// 序列化消息
	messageDataBytes, err := json.Marshal(message)
	if err != nil {
		return -6 // 消息序列化失败
	}

	// 返回数据
	*out_data = C.CString(string(signData))
	*out_len = C.int(len(signData))
	*message_data = C.CString(string(messageDataBytes))
	*message_len = C.int(len(messageDataBytes))

	return 0 // 成功
}

//export go_ecdsa_keygen_p2
func go_ecdsa_keygen_p2(key_data *C.char, key_len C.int, p1_id C.int, p1_message *C.char, p1_msg_len C.int, p2_params *C.char, p2_params_len C.int, out_data **C.char, out_len *C.int) C.int {
	keyDataStr := C.GoStringN(key_data, key_len)
	p1MessageStr := C.GoStringN(p1_message, p1_msg_len)
	p2ParamsStr := C.GoStringN(p2_params, p2_params_len)

	// 解析DKG密钥数据
	var keyStep3Data tss.KeyStep3Data
	err := json.Unmarshal([]byte(keyDataStr), &keyStep3Data)
	if err != nil {
		return -1 // DKG密钥解析错误
	}

	// 解析P1消息
	var message tss.Message
	err = json.Unmarshal([]byte(p1MessageStr), &message)
	if err != nil {
		return -2 // P1消息解析错误
	}

	// 解析P2预参数
	var p2PreParamsAndProof keygen.PreParamsWithDlnProof
	err = json.Unmarshal([]byte(p2ParamsStr), &p2PreParamsAndProof)
	if err != nil {
		return -3 // P2预参数解析错误
	}

	// 从DKG数据中提取必要信息
	share2 := keyStep3Data.ShareI
	publicKey := keyStep3Data.PublicKey

	// 执行P2 keygen，使用传入的P2预参数
	p2SaveData, err := keygen.P2(share2, publicKey, &message, int(p1_id), keyStep3Data.Id, p2PreParamsAndProof.PedersonParameters())
	if err != nil {
		log.Println("err is ", err)
		return -4 // P2 keygen执行失败
	}

	// 创建完整的ECDSA签名数据
	ecdsaSignData := ECDSASignData{
		KeyStep3Data: keyStep3Data,
		PaiPrivate:   nil, // P2不需要PaiPrivate
		E_x1:         nil, // P2不需要E_x1
		P2SaveData:   p2SaveData,
	}

	// 序列化签名数据
	signData, err := json.Marshal(ecdsaSignData)
	if err != nil {
		return -5 // 签名数据序列化失败
	}

	// 返回数据
	*out_data = C.CString(string(signData))
	*out_len = C.int(len(signData))

	return 0 // 成功
}

func main() {
	// CGO库不需要main函数，但Go要求有
}
