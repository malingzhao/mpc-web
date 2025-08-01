package main

import (
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/decred/dcrd/dcrec/edwards/v2"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/okx/threshold-lib/tss"
	"github.com/okx/threshold-lib/tss/key/reshare"
)

// RefreshClient 专门用于密钥刷新的客户端
type RefreshClient struct {
	ClientID     string
	PartyID      int
	Threshold    int
	TotalParties int
	ServerURL    string
	conn         *websocket.Conn
	sessionID    string
	refreshDone  chan bool

	// 原始密钥数据（从keygen获得）
	originalKeyData *tss.KeyStep3Data

	// Refresh相关字段
	curve                 elliptic.Curve
	refreshSetup          *reshare.RefreshInfo
	refreshRound1Messages map[int]interface{}
	refreshRound2Messages map[int]*tss.Message
	refreshedKeyData      *tss.KeyStep3Data
}

// Message 消息结构
type Message struct {
	Type      string `json:"type"`
	ClientID  string `json:"client_id"`
	SessionID string `json:"session_id,omitempty"`
	Data      string `json:"data,omitempty"`
	Error     string `json:"error,omitempty"`
}

// NewRefreshClient 创建新的刷新客户端
func NewRefreshClient(clientID, serverURL string, partyID, threshold, totalParties int, originalKeyData *tss.KeyStep3Data) *RefreshClient {
	// 使用edwards曲线
	curve := edwards.Edwards()

	return &RefreshClient{
		ClientID:              clientID,
		PartyID:               partyID,
		Threshold:             threshold,
		TotalParties:          totalParties,
		ServerURL:             serverURL,
		refreshDone:           make(chan bool, 1),
		originalKeyData:       originalKeyData,
		curve:                 curve,
		refreshRound1Messages: make(map[int]interface{}),
		refreshRound2Messages: make(map[int]*tss.Message),
	}
}

// Connect 连接到协调服务器
func (c *RefreshClient) Connect() error {
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

	log.Printf("🔗 刷新客户端 %s 正在连接到: %s", c.ClientID, u.String())

	// 建立WebSocket连接
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("连接失败: %v", err)
	}

	c.conn = conn
	log.Printf("✅ 刷新客户端 %s 连接成功", c.ClientID)
	return nil
}

// listenMessages 监听服务器消息
func (c *RefreshClient) listenMessages() {
	for {
		var msg Message
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("❌ 刷新客户端 %s 读取消息失败: %v", c.ClientID, err)
			return
		}

		c.handleMessage(&msg)
	}
}

// handleMessage 处理收到的消息
func (c *RefreshClient) handleMessage(msg *Message) {
	log.Printf("📨 刷新客户端 %s 收到消息: %s", c.ClientID, msg.Type)

	switch msg.Type {
	case "session_created":
		c.handleSessionCreated(msg)
	case "start_refresh":
		c.handleStartRefresh(msg)
	case "refresh_round1_data":
		c.handleRefreshRound1Data(msg)
	case "refresh_round2_data":
		c.handleRefreshRound2Data(msg)
	case "refresh_complete":
		c.handleRefreshComplete(msg)
	case "error":
		log.Printf("❌ 刷新客户端 %s 收到错误: %s", c.ClientID, msg.Error)
	default:
		log.Printf("⚠️ 刷新客户端 %s 收到未知消息类型: %s", c.ClientID, msg.Type)
	}
}

// sendMessage 发送消息到服务器
func (c *RefreshClient) sendMessage(msg *Message) error {
	if c.conn == nil {
		return fmt.Errorf("连接未建立")
	}

	return c.conn.WriteJSON(msg)
}

// StartRefresh 启动密钥刷新
func (c *RefreshClient) StartRefresh() error {
	log.Printf("🔄 刷新客户端 %s 开始密钥刷新...", c.ClientID)

	// 等待一小段时间确保连接稳定
	time.Sleep(500 * time.Millisecond)

	// 请求创建刷新会话
	createDataMap := map[string]interface{}{
		"session_type":  "refresh",
		"party_id":      c.PartyID,
		"threshold":     c.Threshold,
		"total_parties": c.TotalParties,
	}
	createDataBytes, _ := json.Marshal(createDataMap)
	msg := &Message{
		Type:      "create_session",
		ClientID:  c.ClientID,
		SessionID: c.sessionID,
		Data:      string(createDataBytes),
	}

	return c.sendMessage(msg)
}

// handleSessionCreated 处理会话创建
func (c *RefreshClient) handleSessionCreated(msg *Message) {
	var sessionData struct {
		SessionID string `json:"session_id"`
	}
	err := json.Unmarshal([]byte(msg.Data), &sessionData)
	if err != nil {
		log.Printf("❌ 刷新客户端 %s 解析会话数据失败: %v", c.ClientID, err)
		return
	}
	c.sessionID = sessionData.SessionID
	log.Printf("✅ 刷新客户端 %s 加入会话: %s", c.ClientID, c.sessionID)
}

// handleStartRefresh 处理开始刷新
func (c *RefreshClient) handleStartRefresh(msg *Message) {
	log.Printf("🔄 刷新客户端 %s 开始执行密钥刷新第一轮...", c.ClientID)

	// 使用原始密钥数据进行刷新
	if c.originalKeyData == nil {
		log.Printf("❌ 刷新客户端 %s 没有可用的密钥数据进行刷新", c.ClientID)
		return
	}

	// 创建刷新设置
	devoteList := [2]int{c.Threshold, c.TotalParties}
	refreshInfo := reshare.NewRefresh(c.PartyID, c.TotalParties, devoteList, c.originalKeyData.ShareI, c.originalKeyData.PublicKey)
	c.refreshSetup = refreshInfo

	// 执行第一轮
	round1Data, err := refreshInfo.DKGStep1()
	if err != nil {
		log.Printf("❌ 刷新客户端 %s 刷新第一轮失败: %v", c.ClientID, err)
		return
	}

	// 发送第一轮数据
	round1DataMap := map[string]interface{}{
		"party_id":    c.PartyID,
		"round1_data": round1Data,
	}
	round1DataBytes, _ := json.Marshal(round1DataMap)
	refreshMsg := &Message{
		Type:      "refresh_round1",
		ClientID:  c.ClientID,
		SessionID: c.sessionID,
		Data:      string(round1DataBytes),
	}

	err = c.sendMessage(refreshMsg)
	if err != nil {
		log.Printf("❌ 刷新客户端 %s 发送刷新第一轮数据失败: %v", c.ClientID, err)
	}
}

// handleRefreshRound1Data 处理刷新第一轮数据
func (c *RefreshClient) handleRefreshRound1Data(msg *Message) {
	log.Printf("📨 刷新客户端 %s 收到刷新第一轮数据", c.ClientID)

	// 解析数据 - 协调服务器发送的是map[clientID]data格式
	var roundDataMap map[string]string
	err := json.Unmarshal([]byte(msg.Data), &roundDataMap)
	if err != nil {
		log.Printf("❌ 刷新客户端 %s 解析刷新第一轮数据失败: %v", c.ClientID, err)
		return
	}

	// 解析每个客户端的数据
	for clientID, dataStr := range roundDataMap {
		var refreshData struct {
			PartyID    int                    `json:"party_id"`
			Round1Data map[string]interface{} `json:"round1_data"`
		}
		err := json.Unmarshal([]byte(dataStr), &refreshData)
		if err != nil {
			log.Printf("❌ 刷新客户端 %s 解析客户端 %s 的第一轮数据失败: %v", c.ClientID, clientID, err)
			continue
		}
		c.refreshRound1Messages[refreshData.PartyID] = refreshData.Round1Data
	}

	// 检查是否收集到所有第一轮消息
	if len(c.refreshRound1Messages) == c.TotalParties {
		log.Printf("✅ 刷新客户端 %s 收集到所有刷新第一轮消息，开始第二轮", c.ClientID)
		c.executeRefreshRound2()
	}
}

// executeRefreshRound2 执行刷新第二轮
func (c *RefreshClient) executeRefreshRound2() {
	// 从每个参与方的round1_data中提取发送给当前客户端的消息
	messages := make([]*tss.Message, 0, len(c.refreshRound1Messages)-1)
	for partyID, round1DataMap := range c.refreshRound1Messages {
		if partyID != c.PartyID {
			// round1DataMap 是 map[int]*tss.Message 类型
			if msgMap, ok := round1DataMap.(map[string]interface{}); ok {
				// 查找发送给当前客户端的消息
				for toStr, msgData := range msgMap {
					to := 0
					fmt.Sscanf(toStr, "%d", &to)
					if to == c.PartyID {
						// 将消息数据转换为 tss.Message
						if msgBytes, err := json.Marshal(msgData); err == nil {
							var tssMsg tss.Message
							if err := json.Unmarshal(msgBytes, &tssMsg); err == nil {
								messages = append(messages, &tssMsg)
							}
						}
						break
					}
				}
			}
		}
	}

	// 执行第二轮
	round2Data, err := c.refreshSetup.DKGStep2(messages)
	if err != nil {
		log.Printf("❌ 刷新客户端 %s 刷新第二轮失败: %v", c.ClientID, err)
		return
	}

	// 发送第二轮数据
	round2DataMap := map[string]interface{}{
		"party_id":    c.PartyID,
		"round2_data": round2Data,
	}
	round2DataBytes, _ := json.Marshal(round2DataMap)
	refreshMsg := &Message{
		Type:      "refresh_round2",
		ClientID:  c.ClientID,
		SessionID: c.sessionID,
		Data:      string(round2DataBytes),
	}

	err = c.sendMessage(refreshMsg)
	if err != nil {
		log.Printf("❌ 刷新客户端 %s 发送刷新第二轮数据失败: %v", c.ClientID, err)
	}
}

// handleRefreshRound2Data 处理刷新第二轮数据
func (c *RefreshClient) handleRefreshRound2Data(msg *Message) {
	log.Printf("📨 刷新客户端 %s 收到刷新第二轮数据", c.ClientID)

	// 解析数据 - 协调服务器发送的是map[clientID]data格式
	var roundDataMap map[string]string
	err := json.Unmarshal([]byte(msg.Data), &roundDataMap)
	if err != nil {
		log.Printf("❌ 刷新客户端 %s 解析刷新第二轮数据失败: %v", c.ClientID, err)
		return
	}

	// 解析每个客户端的数据
	for clientID, dataStr := range roundDataMap {
		var refreshData struct {
			PartyID    int                    `json:"party_id"`
			Round2Data map[string]interface{} `json:"round2_data"`
		}
		err := json.Unmarshal([]byte(dataStr), &refreshData)
		if err != nil {
			log.Printf("❌ 刷新客户端 %s 解析客户端 %s 的第二轮数据失败: %v", c.ClientID, clientID, err)
			continue
		}

		// 从 round2_data map 中提取发送给当前客户端的消息
		if refreshData.Round2Data != nil {
			for toStr, msgData := range refreshData.Round2Data {
				to := 0
				fmt.Sscanf(toStr, "%d", &to)
				if to == c.PartyID {
					// 将消息数据转换为 tss.Message
					if msgBytes, err := json.Marshal(msgData); err == nil {
						var tssMsg tss.Message
						if err := json.Unmarshal(msgBytes, &tssMsg); err == nil {
							// 使用发送方的PartyID作为key
							c.refreshRound2Messages[refreshData.PartyID] = &tssMsg
							log.Printf("📥 刷新客户端 %s 收到来自参与方 %d 的第二轮消息", c.ClientID, refreshData.PartyID)
						}
					}
					break
				}
			}
		}
	}

	// 检查是否收集到所有第二轮消息（除了自己）
	if len(c.refreshRound2Messages) == c.TotalParties-1 {
		log.Printf("✅ 刷新客户端 %s 收集到所有刷新第二轮消息，开始第三轮", c.ClientID)
		c.executeRefreshRound3()
	}
}

// executeRefreshRound3 执行刷新第三轮
func (c *RefreshClient) executeRefreshRound3() {
	// 从第二轮消息中提取发送给当前客户端的消息
	messages := make([]*tss.Message, 0)
	for partyID, msg := range c.refreshRound2Messages {
		if partyID != c.PartyID && msg != nil {
			// 检查消息的 To 字段是否为当前客户端
			if msg.To == c.PartyID {
				messages = append(messages, msg)
			}
		}
	}

	// 执行第三轮
	refreshedKeyData, err := c.refreshSetup.DKGStep3(messages)
	if err != nil {
		log.Printf("❌ 刷新客户端 %s 刷新第三轮失败: %v", c.ClientID, err)
		return
	}

	// 保存刷新后的密钥数据
	c.refreshedKeyData = refreshedKeyData

	log.Printf("🎉 刷新客户端 %s 密钥刷新完成！", c.ClientID)

	// 发送完成消息
	completeDataMap := map[string]interface{}{
		"party_id": c.PartyID,
		"success":  true,
	}
	completeDataBytes, _ := json.Marshal(completeDataMap)
	refreshMsg := &Message{
		Type:      "refresh_complete",
		ClientID:  c.ClientID,
		SessionID: c.sessionID,
		Data:      string(completeDataBytes),
	}

	err = c.sendMessage(refreshMsg)
	if err != nil {
		log.Printf("❌ 刷新客户端 %s 发送刷新完成消息失败: %v", c.ClientID, err)
	}

	// 通知刷新完成
	c.refreshDone <- true
}

// handleRefreshComplete 处理刷新完成
func (c *RefreshClient) handleRefreshComplete(msg *Message) {
	log.Printf("🎉 刷新客户端 %s 收到密钥刷新完成通知", c.ClientID)
	c.refreshDone <- true
}

// Run 运行刷新客户端
func (c *RefreshClient) Run() error {
	// 连接到服务器
	err := c.Connect()
	if err != nil {
		return err
	}

	// 启动消息监听
	go c.listenMessages()

	// 启动密钥刷新
	return c.StartRefresh()
}

// GetRefreshedKey 获取刷新后的密钥数据
func (c *RefreshClient) GetRefreshedKey() *tss.KeyStep3Data {
	return c.refreshedKeyData
}

// 模拟从文件加载原始密钥数据的函数
func loadOriginalKeyData(partyID int) *tss.KeyStep3Data {
	// 从密钥文件中加载数据
	keyFileName := fmt.Sprintf("test_key_party%d.key", partyID)
	log.Printf("🔑 正在加载密钥文件: %s", keyFileName)

	// 读取密钥文件
	keyDataBytes, err := os.ReadFile(keyFileName)
	if err != nil {
		log.Printf("❌ 读取密钥文件失败: %v", err)
		return nil
	}

	keyDataStr := strings.TrimSpace(string(keyDataBytes))

	// 尝试解析JSON格式的密钥数据（Go客户端生成）
	var keyData tss.KeyStep3Data
	err = json.Unmarshal([]byte(keyDataStr), &keyData)
	if err == nil {
		log.Printf("✅ 成功加载参与方 %d 的JSON格式密钥数据", partyID)
		return &keyData
	}

	// 如果JSON解析失败，尝试Base64解码（Java客户端生成）
	log.Printf("🔄 JSON解析失败，尝试Base64解码: %v", err)
	decodedData, err := base64.StdEncoding.DecodeString(keyDataStr)
	if err != nil {
		log.Printf("❌ Base64解码失败: %v", err)
		return nil
	}

	// 尝试解析解码后的数据为JSON
	err = json.Unmarshal(decodedData, &keyData)
	if err != nil {
		log.Printf("❌ 解析Base64解码后的密钥数据失败: %v", err)
		return nil
	}

	log.Printf("✅ 成功加载参与方 %d 的Base64格式密钥数据", partyID)
	return &keyData
}

func main() {
	var clientID, serverURL, keyFile string
	var partyID, threshold, totalParties int

	// 解析命令行参数
	flag.StringVar(&clientID, "client-id", "", "客户端ID")
	flag.IntVar(&partyID, "party-id", 0, "参与方ID")
	flag.IntVar(&threshold, "threshold", 0, "阈值")
	flag.IntVar(&totalParties, "total-parties", 0, "总参与方数")
	flag.StringVar(&keyFile, "key-file", "", "密钥文件路径")
	flag.StringVar(&serverURL, "server-url", "ws://localhost:8080/ws", "服务器URL")
	flag.Parse()

	if clientID == "" || partyID == 0 || threshold == 0 || totalParties == 0 || keyFile == "" {
		fmt.Println("用法: refresh_client -client-id=<id> -party-id=<id> -threshold=<n> -total-parties=<n> -key-file=<file> [-server-url=<url>]")
		os.Exit(1)
	}

	// 加载原始密钥数据（从之前的keygen获得）
	originalKeyData := loadOriginalKeyData(partyID)
	if originalKeyData == nil {
		log.Printf("⚠️ 警告：没有加载到原始密钥数据，刷新可能会失败")
	}

	// 创建刷新客户端
	client := NewRefreshClient(clientID, serverURL, partyID, threshold, totalParties, originalKeyData)

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Printf("🛑 收到退出信号，正在关闭刷新客户端 %s", clientID)
		if client.conn != nil {
			client.conn.Close()
		}
		os.Exit(0)
	}()

	// 运行刷新客户端
	err := client.Run()
	if err != nil {
		log.Fatalf("❌ 运行失败: %v", err)
	}

	// 等待刷新完成
	select {
	case <-client.refreshDone:
		log.Printf("✅ 刷新客户端 %s 密钥刷新完成！", clientID)
		if client.GetRefreshedKey() != nil {
			log.Printf("🔑 刷新后的密钥份额数量: %d", len(client.GetRefreshedKey().SharePubKeyMap))
		}
	case <-time.After(60 * time.Second):
		log.Printf("⏰ 刷新客户端 %s 超时", clientID)
	}

	log.Printf("✅ 刷新客户端 %s 运行完成", clientID)
}
