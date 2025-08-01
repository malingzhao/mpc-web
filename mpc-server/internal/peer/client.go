package peer

import (
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"mpc-server/internal/config"
	"mpc-server/internal/protocol"
)

// PeerClient 管理与其他服务器的WebSocket连接
type PeerClient struct {
	serverID       string
	config         *config.ServerConfig
	connections    map[string]*websocket.Conn
	writeMutexes   map[string]*sync.Mutex // 为每个连接添加写入锁
	mu             sync.RWMutex
	messageHandler MessageHandler
}

// MessageHandler 处理来自peer的消息
type MessageHandler interface {
	HandlePeerMessage(peerID string, message []byte)
}

// NewPeerClient 创建新的PeerClient
func NewPeerClient(serverID string, config *config.ServerConfig) *PeerClient {
	return &PeerClient{
		serverID:     serverID,
		config:       config,
		connections:  make(map[string]*websocket.Conn),
		writeMutexes: make(map[string]*sync.Mutex),
	}
}

// SetMessageHandler 设置消息处理器
func (pc *PeerClient) SetMessageHandler(handler MessageHandler) {
	pc.messageHandler = handler
}

// ConnectToPeers 连接到所有配置的peer服务器
func (pc *PeerClient) ConnectToPeers(peers []config.Peer) error {
	for _, peer := range pc.config.Peers {
		success := false
		for {
			if !success {
				if err := pc.connectToPeer(peer); err != nil {
					log.Printf("Failed to connect to peer %s: %v", peer.ID, err) // 继续尝试连接其他peer，不因为一个失败而停止
				} else {
					success = true
				}
				time.Sleep(time.Second * 1)

			}

			if success {
				break
			}
		}
	}
	return nil
}

// connectToPeer 连接到指定的peer
func (pc *PeerClient) connectToPeer(peer config.Peer) error {
	u, err := url.Parse(peer.URL)
	if err != nil {
		return fmt.Errorf("invalid peer URL %s: %v", peer.URL, err)
	}

	// 添加client_id参数
	q := u.Query()
	q.Set("client_id", pc.serverID)
	u.RawQuery = q.Encode()

	log.Printf("Connecting to peer %s at %s", peer.ID, u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to dial %s: %v", u.String(), err)
	}

	pc.mu.Lock()
	pc.connections[peer.ID] = conn
	pc.writeMutexes[peer.ID] = &sync.Mutex{} // 为新连接创建写入锁
	pc.mu.Unlock()

	log.Printf("Successfully connected to peer %s", peer.ID)

	// 启动消息读取goroutine
	go pc.readMessages(peer.ID, conn)

	return nil
}

// readMessages 读取来自peer的消息
func (pc *PeerClient) readMessages(peerID string, conn *websocket.Conn) {
	defer func() {
		pc.mu.Lock()
		delete(pc.connections, peerID)
		delete(pc.writeMutexes, peerID) // 清理写入锁
		pc.mu.Unlock()
		conn.Close()
		log.Printf("Disconnected from peer %s", peerID)
	}()

	for {
		_, messageBytes, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error from peer %s: %v", peerID, err)
			}
			break
		}

		if pc.messageHandler != nil {
			pc.messageHandler.HandlePeerMessage(peerID, messageBytes)
		}
	}
}

// SendToPeer 发送消息给指定的peer
func (pc *PeerClient) SendToPeer(peerID string, message []byte) error {
	pc.mu.RLock()
	conn, exists := pc.connections[peerID]
	writeMutex, mutexExists := pc.writeMutexes[peerID]
	pc.mu.RUnlock()

	if !exists || !mutexExists {
		return fmt.Errorf("no connection to peer %s", peerID)
	}

	// 使用写入锁保护WebSocket写入操作
	writeMutex.Lock()
	defer writeMutex.Unlock()

	return conn.WriteMessage(websocket.TextMessage, message)
}

// BroadcastToPeers 广播消息给所有连接的peer
func (pc *PeerClient) BroadcastToPeers(message []byte) {
	pc.mu.RLock()
	// 复制连接和锁的映射，避免长时间持有读锁
	connections := make(map[string]*websocket.Conn)
	writeMutexes := make(map[string]*sync.Mutex)
	for peerID, conn := range pc.connections {
		connections[peerID] = conn
		writeMutexes[peerID] = pc.writeMutexes[peerID]
	}
	pc.mu.RUnlock()

	for peerID, conn := range connections {
		writeMutex := writeMutexes[peerID]
		if writeMutex != nil {
			// 使用写入锁保护WebSocket写入操作
			writeMutex.Lock()
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("Failed to send message to peer %s: %v", peerID, err)
			}
			writeMutex.Unlock()
		}
	}
}

// GetConnectedPeers 获取已连接的peer列表
func (pc *PeerClient) GetConnectedPeers() []string {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	peers := make([]string, 0, len(pc.connections))
	for peerID := range pc.connections {
		peers = append(peers, peerID)
	}
	return peers
}

// Close 关闭所有连接
func (pc *PeerClient) Close() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	for peerID, conn := range pc.connections {
		conn.Close()
		log.Printf("Closed connection to peer %s", peerID)
	}
	pc.connections = make(map[string]*websocket.Conn)
	pc.writeMutexes = make(map[string]*sync.Mutex) // 清理所有写入锁
}

// SendSessionSync 发送会话同步消息
func (pc *PeerClient) SendSessionSync(sessionID string, sessionData interface{}) {
	msg := protocol.NewMessage(
		protocol.MsgTypeSessionSync,
		sessionID,
		pc.serverID,
		"", // 广播给所有peer
		sessionData,
	)

	msgBytes, err := msg.ToJSON()
	if err != nil {
		log.Printf("Failed to marshal session sync message: %v", err)
		return
	}

	pc.BroadcastToPeers(msgBytes)
	log.Printf("Sent session sync for session %s", sessionID)
}
