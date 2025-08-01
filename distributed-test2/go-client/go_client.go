package main

import (
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/decred/dcrd/dcrec/edwards/v2"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/okx/threshold-lib/tss"
	"github.com/okx/threshold-lib/tss/key/dkg"
	"github.com/okx/threshold-lib/tss/key/reshare"
)

// DistributedGoClient Go客户端结构
type DistributedGoClient struct {
	ClientID      string
	PartyID       int
	Threshold     int
	TotalParties  int
	ServerURL     string
	conn          *websocket.Conn
	sessionID     string
	keygenDone    chan bool
	sessionJoined bool // 添加标志，防止重复处理session_created

	// DKG相关字段
	dkgSetup       *dkg.SetupInfo
	curve          elliptic.Curve
	round1Messages map[int]*tss.Message // 存储第一轮消息
	round2Messages map[int]*tss.Message // 存储第二轮消息
	finalKeyData   *tss.KeyStep3Data    // 最终生成的密钥数据

	// Refresh相关字段
	refreshDone           chan bool
	refreshSetup          interface{}          // refresh设置信息
	refreshRound1Messages map[int]*tss.Message // 存储refresh第一轮消息
	refreshRound2Messages map[int]*tss.Message // 存储refresh第二轮消息
	refreshedKeyData      *tss.KeyStep3Data    // refresh后的密钥数据
}

// Message WebSocket消息结构
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

// NewDistributedGoClient 创建新的Go客户端
func NewDistributedGoClient(clientID, serverURL string, partyID, threshold, totalParties int) *DistributedGoClient {
	// 使用secp256k1曲线
	curve := edwards.Edwards()

	// 创建DKG设置
	dkgSetup := dkg.NewSetUp(partyID, totalParties, curve)

	return &DistributedGoClient{
		ClientID:       clientID,
		PartyID:        partyID,
		Threshold:      threshold,
		TotalParties:   totalParties,
		ServerURL:      serverURL,
		keygenDone:     make(chan bool, 1),
		dkgSetup:       dkgSetup,
		curve:          curve,
		round1Messages: make(map[int]*tss.Message),
		round2Messages: make(map[int]*tss.Message),
		// 初始化refresh相关字段
		refreshDone:           make(chan bool, 1),
		refreshRound1Messages: make(map[int]*tss.Message),
		refreshRound2Messages: make(map[int]*tss.Message),
	}
}

// Connect 连接到协调服务器
func (c *DistributedGoClient) Connect() error {
	// 解析服务器URL
	serverURL, err := url.Parse(c.ServerURL)
	if err != nil {
		return fmt.Errorf("解析服务器URL失败: %v", err)
	}

	// 构建WebSocket URL
	u := url.URL{
		Scheme: "ws",
		Host:   serverURL.Host,
		Path:   "/ws",
	}
	q := u.Query()
	q.Set("client_id", c.ClientID)
	u.RawQuery = q.Encode()

	log.Printf("🔗 Go客户端 %s 正在连接到 %s", c.ClientID, u.String())

	c.conn, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("连接失败: %v", err)
	}

	log.Printf("✅ Go客户端 %s 已连接", c.ClientID)
	return nil
}

// listenMessages 监听来自服务器的消息
func (c *DistributedGoClient) listenMessages() {
	defer func() {
		if c.conn != nil {
			c.conn.Close()
		}
	}()

	for {
		var msg Message
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("❌ Go客户端 %s 读取消息失败: %v", c.ClientID, err)
			break
		}

		log.Printf("📥 Go客户端 %s 收到消息: %s", c.ClientID, msg.Type)
		c.handleMessage(&msg)
	}
}

// handleMessage 处理收到的消息
func (c *DistributedGoClient) handleMessage(msg *Message) {
	switch msg.Type {
	case "session_created":
		c.handleSessionCreated(msg)
	case "start_keygen":
		c.handleStartKeygen(msg)
	case "keygen_round1":
		c.handleKeygenRound1Data(msg)
	case "keygen_round2":
		c.handleKeygenRound2Data(msg)
	case "keygen_complete":
		c.handleKeygenComplete(msg)
	case "start_refresh":
		c.handleStartRefresh(msg)
	case "refresh_round1_data":
		c.handleRefreshRound1Data(msg)
	case "refresh_round2_data":
		c.handleRefreshRound2Data(msg)
	case "refresh_complete":
		c.handleRefreshComplete(msg)
	case "error":
		log.Printf("❌ Go客户端 %s 收到错误: %s", c.ClientID, msg.Error)
	default:
		log.Printf("⚠️ Go客户端 %s 收到未知消息类型: %s", c.ClientID, msg.Type)
	}
}

// sendMessage 发送消息到服务器
func (c *DistributedGoClient) sendMessage(msg *Message) error {
	if c.conn == nil {
		return fmt.Errorf("连接未建立")
	}

	return c.conn.WriteJSON(msg)
}

// Run 运行客户端
func (c *DistributedGoClient) Run() error {
	// 连接到服务器
	err := c.Connect()
	if err != nil {
		return err
	}

	// 启动消息监听
	go c.listenMessages()

	// 启动密钥生成
	return c.StartKeygen()
}

// StartKeygen 启动密钥生成
func (c *DistributedGoClient) StartKeygen() error {
	log.Printf("🔑 Go客户端 %s 开始密钥生成...", c.ClientID)

	// 等待一小段时间确保连接稳定
	time.Sleep(500 * time.Millisecond)

	// 构建会话数据
	sessionData := map[string]interface{}{
		"session_type":  "keygen",
		"party_id":      c.PartyID,
		"threshold":     c.Threshold,
		"total_parties": c.TotalParties,
	}

	// 将会话数据转换为JSON字符串
	dataBytes, err := json.Marshal(sessionData)
	if err != nil {
		return fmt.Errorf("序列化会话数据失败: %v", err)
	}

	// 请求创建会话
	msg := &Message{
		Type:         "create_session",
		ClientID:     c.ClientID,
		SessionType:  "keygen",
		Threshold:    c.Threshold,
		TotalParties: c.TotalParties,
		Data:         string(dataBytes),
	}

	log.Printf("📤 Go客户端 %s 正在发送create_session请求: Type=%s, SessionType=%s, Threshold=%d, TotalParties=%d",
		c.ClientID, msg.Type, msg.SessionType, msg.Threshold, msg.TotalParties)

	err = c.sendMessage(msg)
	if err != nil {
		return fmt.Errorf("发送创建会话消息失败: %v", err)
	}

	log.Printf("✅ Go客户端 %s 已成功发送create_session请求", c.ClientID)

	// 等待密钥生成完成
	select {
	case success := <-c.keygenDone:
		if success {
			log.Printf("✅ Go客户端 %s 密钥生成成功", c.ClientID)
			return nil
		} else {
			return fmt.Errorf("密钥生成失败")
		}
	case <-time.After(60 * time.Second):
		return fmt.Errorf("密钥生成超时")
	}
}

// handleSessionCreated 处理会话创建
func (c *DistributedGoClient) handleSessionCreated(msg *Message) {
	// 检查是否已经处理过session_created消息
	if c.sessionJoined {
		log.Printf("⚠️ Go客户端 %s 已经加入会话，忽略重复的session_created消息", c.ClientID)
		return
	}

	c.sessionID = msg.SessionID
	c.sessionJoined = true // 设置标志，防止重复处理
	log.Printf("✅ Go客户端 %s 会话已创建: %s，等待协调服务器开始信号", c.ClientID, c.sessionID)

	// 不重新初始化DKG设置，保持原有状态
	// 只清空之前的消息
	c.round1Messages = make(map[int]*tss.Message)
	c.round2Messages = make(map[int]*tss.Message)

	// 不立即开始密钥生成，等待协调服务器的开始信号
	log.Printf("⏳ Go客户端 %s 等待协调服务器发送开始密钥生成信号", c.ClientID)
}

// handleStartKeygen 处理开始密钥生成信号
func (c *DistributedGoClient) handleStartKeygen(msg *Message) {
	log.Printf("🚀 Go客户端 %s 收到开始密钥生成信号，开始第一轮", c.ClientID)
	c.performKeygenRound1()
}

// performKeygenRound1 执行密钥生成第一轮
func (c *DistributedGoClient) performKeygenRound1() {
	log.Printf("🔄 Go客户端 %s (PartyID: %d) 开始DKG第一轮", c.ClientID, c.PartyID)
	log.Printf("🔍 Go客户端 %s DKG设置状态: RoundNumber=%d, DeviceNumber=%d, Total=%d",
		c.ClientID, c.dkgSetup.RoundNumber, c.dkgSetup.DeviceNumber, c.dkgSetup.Total)

	// 执行DKG第一轮
	round1Msgs, err := c.dkgSetup.DKGStep1()
	if err != nil {
		log.Printf("❌ Go客户端 %s DKG第一轮失败: %v", c.ClientID, err)
		log.Printf("🔍 Go客户端 %s 失败时DKG设置状态: RoundNumber=%d", c.ClientID, c.dkgSetup.RoundNumber)
		c.keygenDone <- false
		return
	}

	log.Printf("📊 Go客户端 %s 生成了 %d 条第一轮消息", c.ClientID, len(round1Msgs))

	// 将所有消息打包成一个JSON对象
	allMsgs := make(map[string]interface{})
	for toParty, msg := range round1Msgs {
		if toParty == c.PartyID {
			continue // 不发送给自己
		}
		allMsgs[fmt.Sprintf("%d", toParty)] = map[string]interface{}{
			"From": msg.From,
			"To":   msg.To,
			"Data": msg.Data,
		}
		log.Printf("📤 Go客户端 %s 已准备第一轮消息给参与方 %d", c.ClientID, toParty)
	}

	// 序列化所有消息
	allMsgsBytes, err := json.Marshal(allMsgs)
	if err != nil {
		log.Printf("❌ Go客户端 %s 序列化第一轮消息失败: %v", c.ClientID, err)
		c.keygenDone <- false
		return
	}

	// 编码为base64
	encodedData := base64.StdEncoding.EncodeToString(allMsgsBytes)

	// 发送单个WebSocket消息包含所有目标消息
	response := &Message{
		Type:      "keygen_round1",
		SessionID: c.sessionID,
		FromParty: c.PartyID,
		Data:      encodedData,
	}

	err = c.sendMessage(response)
	if err != nil {
		log.Printf("❌ Go客户端 %s 发送第一轮消息失败: %v", c.ClientID, err)
		c.keygenDone <- false
	} else {
		log.Printf("📤 Go客户端 %s 已发送第一轮消息包", c.ClientID)
	}
}

// handleKeygenRound1Data 处理第一轮密钥生成数据
func (c *DistributedGoClient) handleKeygenRound1Data(msg *Message) {
	log.Printf("📥 Go客户端 %s 收到来自参与方 %d 的第一轮数据", c.ClientID, msg.FromParty)

	// 解码base64数据
	msgData, err := base64.StdEncoding.DecodeString(msg.Data)
	if err != nil {
		log.Printf("❌ Go客户端 %s 解码第一轮数据失败: %v", c.ClientID, err)
		return
	}

	// 解析聚合的消息数据
	var allMsgs map[string]interface{}
	err = json.Unmarshal(msgData, &allMsgs)
	if err != nil {
		log.Printf("❌ Go客户端 %s 解析第一轮聚合数据失败: %v", c.ClientID, err)
		return
	}

	log.Printf("🔍 Go客户端 %s 解码后的聚合数据: %s", c.ClientID, string(msgData))

	// 查找发送给当前参与方的消息
	myPartyKey := fmt.Sprintf("%d", c.PartyID)
	if msgInfo, exists := allMsgs[myPartyKey]; exists {
		// 提取消息信息
		msgMap := msgInfo.(map[string]interface{})
		tssData := msgMap["Data"].(string)

		log.Printf("🔍 Go客户端 %s 提取到发送给自己的数据: %s", c.ClientID, tssData)

		// 创建TSS消息
		tssMsg := &tss.Message{
			From: msg.FromParty,
			To:   c.PartyID,
			Data: tssData,
		}

		// 存储第一轮消息
		c.round1Messages[msg.FromParty] = tssMsg
		log.Printf("📝 Go客户端 %s 已存储来自参与方 %d 的第一轮消息", c.ClientID, msg.FromParty)

		// 检查是否收集到所有第一轮数据
		if c.hasAllRound1Data() {
			log.Printf("✅ Go客户端 %s 已收集到所有第一轮数据，开始第二轮", c.ClientID)
			c.performKeygenRound2()
		}
	} else {
		log.Printf("⚠️ Go客户端 %s 在聚合数据中未找到发送给自己的消息", c.ClientID)
	}
}

// hasAllRound1Data 检查是否收集到所有第一轮数据
func (c *DistributedGoClient) hasAllRound1Data() bool {
	expectedCount := c.TotalParties - 1 // 除了自己
	actualCount := len(c.round1Messages)

	log.Printf("📊 Go客户端 %s 第一轮消息收集进度: %d/%d", c.ClientID, actualCount, expectedCount)
	return actualCount >= expectedCount
}

// performKeygenRound2 执行密钥生成第二轮
func (c *DistributedGoClient) performKeygenRound2() {
	log.Printf("🔄 Go客户端 %s (PartyID: %d) 开始DKG第二轮", c.ClientID, c.PartyID)

	// 准备第一轮消息数组 - 直接遍历存储的消息
	var round1Msgs []*tss.Message
	for fromParty, msg := range c.round1Messages {
		log.Printf("🔍 Go客户端 %s 找到来自参与方 %d 的第一轮消息", c.ClientID, fromParty)
		round1Msgs = append(round1Msgs, msg)
	}

	if len(round1Msgs) != c.TotalParties-1 {
		log.Printf("❌ Go客户端 %s 第一轮消息数量不正确: %d, 期望: %d", c.ClientID, len(round1Msgs), c.TotalParties-1)
		c.keygenDone <- false
		return
	}

	// 执行DKG第二轮
	round2Msgs, err := c.dkgSetup.DKGStep2(round1Msgs)
	if err != nil {
		log.Printf("❌ Go客户端 %s DKG第二轮失败: %v", c.ClientID, err)
		c.keygenDone <- false
		return
	}

	log.Printf("📊 Go客户端 %s 生成了 %d 条第二轮消息", c.ClientID, len(round2Msgs))

	// 将所有消息打包成一个JSON对象
	allMsgs := make(map[string]interface{})
	for toParty, msg := range round2Msgs {
		if toParty == c.PartyID {
			continue // 不发送给自己
		}
		allMsgs[fmt.Sprintf("%d", toParty)] = map[string]interface{}{
			"From": msg.From,
			"To":   msg.To,
			"Data": msg.Data,
		}
		log.Printf("📤 Go客户端 %s 已准备第二轮消息给参与方 %d", c.ClientID, toParty)
	}

	// 序列化所有消息
	allMsgsBytes, err := json.Marshal(allMsgs)
	if err != nil {
		log.Printf("❌ Go客户端 %s 序列化第二轮消息失败: %v", c.ClientID, err)
		c.keygenDone <- false
		return
	}

	// 编码为base64
	encodedData := base64.StdEncoding.EncodeToString(allMsgsBytes)

	// 发送单个WebSocket消息包含所有目标消息
	response := &Message{
		Type:      "keygen_round2",
		SessionID: c.sessionID,
		FromParty: c.PartyID,
		Data:      encodedData,
	}

	err = c.sendMessage(response)
	if err != nil {
		log.Printf("❌ Go客户端 %s 发送第二轮消息失败: %v", c.ClientID, err)
		c.keygenDone <- false
	} else {
		log.Printf("📤 Go客户端 %s 已发送第二轮消息包", c.ClientID)
	}
}

// handleKeygenRound2Data 处理第二轮密钥生成数据
func (c *DistributedGoClient) handleKeygenRound2Data(msg *Message) {
	log.Printf("📥 Go客户端 %s 收到来自参与方 %d 的第二轮数据", c.ClientID, msg.FromParty)

	// 解码base64数据
	msgData, err := base64.StdEncoding.DecodeString(msg.Data)
	if err != nil {
		log.Printf("❌ Go客户端 %s 解码第二轮数据失败: %v", c.ClientID, err)
		return
	}

	// 解析聚合的消息数据
	var allMsgs map[string]interface{}
	err = json.Unmarshal(msgData, &allMsgs)
	if err != nil {
		log.Printf("❌ Go客户端 %s 解析第二轮聚合数据失败: %v", c.ClientID, err)
		return
	}

	log.Printf("🔍 Go客户端 %s 解码后的第二轮聚合数据: %s", c.ClientID, string(msgData))

	// 查找发送给当前参与方的消息
	myPartyKey := fmt.Sprintf("%d", c.PartyID)
	if msgInfo, exists := allMsgs[myPartyKey]; exists {
		// 提取消息信息
		msgMap := msgInfo.(map[string]interface{})
		tssData := msgMap["Data"].(string)

		log.Printf("🔍 Go客户端 %s 提取到发送给自己的第二轮数据: %s", c.ClientID, tssData)

		// 创建TSS消息
		tssMsg := &tss.Message{
			From: msg.FromParty,
			To:   c.PartyID,
			Data: tssData,
		}

		// 存储第二轮消息
		c.round2Messages[msg.FromParty] = tssMsg
		log.Printf("📝 Go客户端 %s 已存储来自参与方 %d 的第二轮消息", c.ClientID, msg.FromParty)

		// 检查是否收集到所有第二轮数据
		if c.hasAllRound2Data() {
			log.Printf("✅ Go客户端 %s 已收集到所有第二轮数据，完成密钥生成", c.ClientID)
			c.performKeygenRound3()
		}
	} else {
		log.Printf("⚠️ Go客户端 %s 在第二轮聚合数据中未找到发送给自己的消息", c.ClientID)
	}
}

// hasAllRound2Data 检查是否收集到所有第二轮数据
func (c *DistributedGoClient) hasAllRound2Data() bool {
	expectedCount := c.TotalParties - 1 // 除了自己
	actualCount := len(c.round2Messages)

	log.Printf("📊 Go客户端 %s 第二轮消息收集进度: %d/%d", c.ClientID, actualCount, expectedCount)
	return actualCount >= expectedCount
}

// performKeygenRound3 执行密钥生成第三轮
func (c *DistributedGoClient) performKeygenRound3() {
	log.Printf("🔄 Go客户端 %s (PartyID: %d) 开始DKG第三轮", c.ClientID, c.PartyID)

	// 准备第二轮消息数组 - 直接遍历存储的消息
	var round2Msgs []*tss.Message
	for fromParty, msg := range c.round2Messages {
		log.Printf("🔍 Go客户端 %s 找到来自参与方 %d 的第二轮消息", c.ClientID, fromParty)
		round2Msgs = append(round2Msgs, msg)
	}

	if len(round2Msgs) != c.TotalParties-1 {
		log.Printf("❌ Go客户端 %s 第二轮消息数量不正确: %d, 期望: %d", c.ClientID, len(round2Msgs), c.TotalParties-1)
		c.keygenDone <- false
		return
	}

	// 执行DKG第三轮，这将直接返回最终密钥数据
	keyData, err := c.dkgSetup.DKGStep3(round2Msgs)
	if err != nil {
		log.Printf("❌ Go客户端 %s DKG第三轮失败: %v", c.ClientID, err)
		c.keygenDone <- false
		return
	}

	// 保存最终密钥数据
	c.finalKeyData = keyData

	// 将密钥数据保存到文件
	keyFileName := fmt.Sprintf("test_key_party%d.key", c.PartyID)
	keyDataBytes, err := json.Marshal(keyData)
	if err != nil {
		log.Printf("❌ Go客户端 %s 序列化密钥数据失败: %v", c.ClientID, err)
	} else {
		err = os.WriteFile(keyFileName, keyDataBytes, 0644)
		if err != nil {
			log.Printf("❌ Go客户端 %s 保存密钥文件失败: %v", c.ClientID, err)
		} else {
			log.Printf("💾 Go客户端 %s 密钥已保存到文件: %s", c.ClientID, keyFileName)
		}
	}

	log.Printf("✅ Go客户端 %s 密钥生成成功完成！", c.ClientID)

	// 发送完成消息
	completeMsg := &Message{
		Type:      "keygen_complete",
		SessionID: c.sessionID,
		FromParty: c.PartyID,
		Data:      "success",
	}

	err = c.sendMessage(completeMsg)
	if err != nil {
		log.Printf("❌ Go客户端 %s 发送完成消息失败: %v", c.ClientID, err)
	} else {
		log.Printf("📤 Go客户端 %s 已发送密钥生成完成消息", c.ClientID)
	}

	// 通知密钥生成完成
	c.keygenDone <- true
}

// handleKeygenComplete 处理密钥生成完成
func (c *DistributedGoClient) handleKeygenComplete(msg *Message) {
	log.Printf("🎉 Go客户端 %s 收到密钥生成完成通知", c.ClientID)
	c.keygenDone <- true
}

// GetDkgKey 获取DKG密钥数据
func (c *DistributedGoClient) GetDkgKey() *tss.KeyStep3Data {
	return c.finalKeyData
}

// StartRefresh 启动密钥刷新
func (c *DistributedGoClient) StartRefresh() error {
	log.Printf("🔄 Go客户端 %s 开始密钥刷新...", c.ClientID)

	// 等待一小段时间确保连接稳定
	time.Sleep(500 * time.Millisecond)

	// 请求创建刷新会话
	sessionData := map[string]interface{}{
		"session_type":  "refresh",
		"party_id":      c.PartyID,
		"threshold":     c.Threshold,
		"total_parties": c.TotalParties,
	}
	dataBytes, _ := json.Marshal(sessionData)
	msg := &Message{
		Type:     "create_session",
		ClientID: c.ClientID,
		Data:     string(dataBytes),
	}

	return c.sendMessage(msg)
}

// handleStartRefresh 处理开始刷新
func (c *DistributedGoClient) handleStartRefresh(msg *Message) {
	log.Printf("🔄 Go客户端 %s 开始执行密钥刷新第一轮...", c.ClientID)

	// 使用现有的密钥数据进行刷新
	if c.finalKeyData == nil {
		log.Printf("❌ Go客户端 %s 没有可用的密钥数据进行刷新", c.ClientID)
		return
	}

	// 创建刷新设置
	devoteList := [2]int{c.PartyID, c.TotalParties}
	refreshInfo := reshare.NewRefresh(c.PartyID, c.TotalParties, devoteList, c.finalKeyData.ShareI, c.finalKeyData.PublicKey)
	c.refreshSetup = refreshInfo

	// 执行第一轮
	round1Data, err := refreshInfo.DKGStep1()
	if err != nil {
		log.Printf("❌ Go客户端 %s 刷新第一轮失败: %v", c.ClientID, err)
		return
	}

	// 发送第一轮数据
	round1DataMap := map[string]interface{}{
		"party_id":    c.PartyID,
		"round1_data": round1Data,
	}
	round1DataBytes, _ := json.Marshal(round1DataMap)
	refreshMsg := &Message{
		Type:     "refresh_round1",
		ClientID: c.ClientID,
		Data:     string(round1DataBytes),
	}

	err = c.sendMessage(refreshMsg)
	if err != nil {
		log.Printf("❌ Go客户端 %s 发送刷新第一轮数据失败: %v", c.ClientID, err)
	}
}

// handleRefreshRound1Data 处理刷新第一轮数据
func (c *DistributedGoClient) handleRefreshRound1Data(msg *Message) {
	log.Printf("📨 Go客户端 %s 收到刷新第一轮数据", c.ClientID)

	// 解析数据
	var refreshData struct {
		PartyID    int          `json:"party_id"`
		Round1Data *tss.Message `json:"round1_data"`
	}
	err := json.Unmarshal([]byte(msg.Data), &refreshData)
	if err != nil {
		log.Printf("❌ Go客户端 %s 解析刷新第一轮数据失败: %v", c.ClientID, err)
		return
	}
	partyID := refreshData.PartyID
	round1Data := refreshData.Round1Data

	// 存储第一轮消息
	c.refreshRound1Messages[partyID] = round1Data

	// 检查是否收集到所有第一轮消息
	if len(c.refreshRound1Messages) == c.TotalParties {
		log.Printf("✅ Go客户端 %s 收集到所有刷新第一轮消息，开始第二轮", c.ClientID)
		c.executeRefreshRound2()
	}
}

// executeRefreshRound2 执行刷新第二轮
func (c *DistributedGoClient) executeRefreshRound2() {
	refreshInfo := c.refreshSetup.(*reshare.RefreshInfo)

	// 转换消息格式
	var round1Messages []*tss.Message
	for _, msg := range c.refreshRound1Messages {
		round1Messages = append(round1Messages, msg)
	}

	// 执行第二轮
	round2Data, err := refreshInfo.DKGStep2(round1Messages)
	if err != nil {
		log.Printf("❌ Go客户端 %s 刷新第二轮失败: %v", c.ClientID, err)
		return
	}

	// 发送第二轮数据
	round2DataMap := map[string]interface{}{
		"party_id":    c.PartyID,
		"round2_data": round2Data,
	}
	round2DataBytes, _ := json.Marshal(round2DataMap)
	refreshMsg := &Message{
		Type:     "refresh_round2",
		ClientID: c.ClientID,
		Data:     string(round2DataBytes),
	}

	err = c.sendMessage(refreshMsg)
	if err != nil {
		log.Printf("❌ Go客户端 %s 发送刷新第二轮数据失败: %v", c.ClientID, err)
	}
}

// handleRefreshRound2Data 处理刷新第二轮数据
func (c *DistributedGoClient) handleRefreshRound2Data(msg *Message) {
	log.Printf("📨 Go客户端 %s 收到刷新第二轮数据", c.ClientID)

	// 解析数据
	var refreshData struct {
		PartyID    int          `json:"party_id"`
		Round2Data *tss.Message `json:"round2_data"`
	}
	err := json.Unmarshal([]byte(msg.Data), &refreshData)
	if err != nil {
		log.Printf("❌ Go客户端 %s 解析刷新第二轮数据失败: %v", c.ClientID, err)
		return
	}
	partyID := refreshData.PartyID
	round2Data := refreshData.Round2Data

	// 存储第二轮消息
	c.refreshRound2Messages[partyID] = round2Data

	// 检查是否收集到所有第二轮消息
	if len(c.refreshRound2Messages) == c.TotalParties {
		log.Printf("✅ Go客户端 %s 收集到所有刷新第二轮消息，开始第三轮", c.ClientID)
		c.executeRefreshRound3()
	}
}

// executeRefreshRound3 执行刷新第三轮
func (c *DistributedGoClient) executeRefreshRound3() {
	refreshInfo := c.refreshSetup.(*reshare.RefreshInfo)

	// 将 refreshRound2Messages 转换为 []*tss.Message
	var round2Messages []*tss.Message
	for _, msg := range c.refreshRound2Messages {
		round2Messages = append(round2Messages, msg)
	}

	// 执行第三轮
	refreshedKeyData, err := refreshInfo.DKGStep3(round2Messages)
	if err != nil {
		log.Printf("❌ Go客户端 %s 刷新第三轮失败: %v", c.ClientID, err)
		return
	}

	// 保存刷新后的密钥数据
	c.refreshedKeyData = refreshedKeyData

	log.Printf("🎉 Go客户端 %s 密钥刷新完成！", c.ClientID)

	// 发送完成消息
	completeDataMap := map[string]interface{}{
		"party_id": c.PartyID,
		"success":  true,
	}
	completeDataBytes, _ := json.Marshal(completeDataMap)
	refreshMsg := &Message{
		Type:     "refresh_complete",
		ClientID: c.ClientID,
		Data:     string(completeDataBytes),
	}

	err = c.sendMessage(refreshMsg)
	if err != nil {
		log.Printf("❌ Go客户端 %s 发送刷新完成消息失败: %v", c.ClientID, err)
	}

	// 通知刷新完成
	c.refreshDone <- true
}

// handleRefreshComplete 处理刷新完成
func (c *DistributedGoClient) handleRefreshComplete(msg *Message) {
	log.Printf("🎉 Go客户端 %s 收到密钥刷新完成通知", c.ClientID)
	c.refreshDone <- true
}

// GetRefreshedKey 获取刷新后的密钥数据
func (c *DistributedGoClient) GetRefreshedKey() *tss.KeyStep3Data {
	return c.refreshedKeyData
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("用法: go run go_client.go <client_id> <server_url> <party_id>")
		os.Exit(1)
	}

	clientID := os.Args[1]
	serverURL := os.Args[2]
	partyID, err := strconv.Atoi(os.Args[3])
	if err != nil {
		log.Fatalf("❌ 无效的参与方ID: %v", err)
	}

	// 创建客户端
	client := NewDistributedGoClient(clientID, serverURL, partyID, 2, 3)

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Printf("🛑 收到退出信号，正在关闭Go客户端 %s", clientID)
		if client.conn != nil {
			client.conn.Close()
		}
		os.Exit(0)
	}()

	// 运行客户端
	err = client.Run()
	if err != nil {
		log.Fatalf("❌ 运行失败: %v", err)
	}

	log.Printf("✅ Go客户端 %s 密钥生成完成，DKG密钥份额数量: %d", clientID, len(client.GetDkgKey().SharePubKeyMap))

	// 等待一段时间，确保其他客户端能够收集完所有数据
	log.Printf("⏳ Go客户端 %s 等待其他客户端完成数据收集...", clientID)
	time.Sleep(10 * time.Second)

	log.Printf("✅ Go客户端 %s 运行完成", clientID)
}
