package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"mpc-server/internal/protocol"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许跨域
	},
}

// MessageHandler 消息处理器接口
type MessageHandler interface {
	HandleWebSocketMessage(client *Client, message []byte)
}

// Client WebSocket客户端连接
type Client struct {
	ID   string
	Conn *websocket.Conn
	Send chan []byte
	Hub  *Hub
	mu   sync.Mutex
}

// Hub WebSocket连接管理中心
type Hub struct {
	clients        map[string]*Client
	register       chan *Client
	unregister     chan *Client
	broadcast      chan []byte
	messageHandler MessageHandler
	mu             sync.RWMutex
}

// NewHub 创建新的Hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 256),
	}
}

// SetMessageHandler 设置消息处理器
func (h *Hub) SetMessageHandler(handler MessageHandler) {
	h.messageHandler = handler
}

// Run 运行Hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.mu.Unlock()
			log.Printf("Client %s connected", client.ID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				close(client.Send)
			}
			h.mu.Unlock()
			log.Printf("Client %s disconnected", client.ID)

		case message := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.clients, client.ID)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// SendToClient 发送消息给指定客户端
func (h *Hub) SendToClient(clientID string, message []byte) error {
	h.mu.RLock()
	client, exists := h.clients[clientID]
	h.mu.RUnlock()

	if !exists {
		return fmt.Errorf("client %s not found", clientID)
	}

	select {
	case client.Send <- message:
		return nil
	default:
		return fmt.Errorf("client %s send channel is full", clientID)
	}
}

// BroadcastMessage 广播消息
func (h *Hub) BroadcastMessage(message []byte) {
	h.broadcast <- message
}

// BroadcastToAll 广播消息给所有连接的客户端
func (h *Hub) BroadcastToAll(message []byte) {
	h.broadcast <- message
}

// GetConnectedClients 获取已连接的客户端列表
func (h *Hub) GetConnectedClients() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := make([]string, 0, len(h.clients))
	for id := range h.clients {
		clients = append(clients, id)
	}
	return clients
}

// HandleWebSocket 处理WebSocket连接
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request, clientID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		ID:   clientID,
		Conn: conn,
		Send: make(chan []byte, 256),
		Hub:  h,
	}

	h.register <- client

	// 启动goroutines
	go client.writePump()
	go client.readPump()
}

// readPump 读取消息
func (c *Client) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(10 * 1024 * 1024)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, messageBytes, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// 使用自定义消息处理器处理消息
		if c.Hub.messageHandler != nil {
			c.Hub.messageHandler.HandleWebSocketMessage(c, messageBytes)
		} else {
			// 默认处理逻辑
			c.handleMessage(messageBytes)
		}
	}
}

// writePump 写入消息
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

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// 批量发送队列中的消息
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
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

// handleMessage 处理接收到的消息（默认处理逻辑）
func (c *Client) handleMessage(messageBytes []byte) {
	// 解析消息
	msg, err := protocol.FromJSON(messageBytes)
	if err != nil {
		log.Printf("Failed to parse message: %v", err)
		return
	}

	log.Printf("Received message from %s: type=%s, session=%s, msg=%v", c.ID, msg.Type, msg.SessionID, msg)

	// 如果消息有指定接收者，转发给对应客户端
	if msg.To != "" && msg.To != c.ID {
		err = c.Hub.SendToClient(msg.To, messageBytes)
		if err != nil {
			log.Printf("Failed to send message to %s: %v", msg.To, err)
		}
		return
	}

	// 如果是广播消息，转发给所有其他客户端
	if msg.To == "" {
		c.Hub.mu.RLock()
		for id, client := range c.Hub.clients {
			if id != c.ID { // 不发送给自己
				select {
				case client.Send <- messageBytes:
				default:
					log.Printf("Failed to send broadcast message to %s", id)
				}
			}
		}
		c.Hub.mu.RUnlock()
	}
}

// SendMessage 发送消息
func (c *Client) SendMessage(msg *protocol.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	messageBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case c.Send <- messageBytes:
		return nil
	default:
		return fmt.Errorf("send channel is full")
	}
}
