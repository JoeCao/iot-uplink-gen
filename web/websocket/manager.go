package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// DefaultUpgrader WebSocket升级器
var DefaultUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源
	},
}

// Client WebSocket客户端
type Client struct {
	ID      string
	Channel string
	Conn    *websocket.Conn
	Send    chan []byte
}

// WSManager WebSocket管理器
type WSManager struct {
	clients    map[string]*Client
	channels   map[string]map[string]*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan BroadcastMessage
	mutex      sync.RWMutex
	running    bool
}

// BroadcastMessage 广播消息
type BroadcastMessage struct {
	Channel string
	Data    interface{}
}

// NewClient 创建新客户端
func NewClient(channel string, conn *websocket.Conn) *Client {
	return &Client{
		ID:      uuid.New().String(),
		Channel: channel,
		Conn:    conn,
		Send:    make(chan []byte, 256),
	}
}

// NewWSManager 创建WebSocket管理器
func NewWSManager() *WSManager {
	return &WSManager{
		clients:    make(map[string]*Client),
		channels:   make(map[string]map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan BroadcastMessage),
		running:    false,
	}
}

// Start 启动WebSocket管理器
func (wsm *WSManager) Start() {
	wsm.running = true
	log.Println("WebSocket manager started")

	for wsm.running {
		select {
		case client := <-wsm.register:
			wsm.registerClient(client)

		case client := <-wsm.unregister:
			wsm.unregisterClient(client)

		case message := <-wsm.broadcast:
			wsm.broadcastMessage(message)
		}
	}
}

// Stop 停止WebSocket管理器
func (wsm *WSManager) Stop() {
	wsm.running = false
	log.Println("WebSocket manager stopped")
	
	// 关闭所有客户端连接
	wsm.mutex.Lock()
	for _, client := range wsm.clients {
		close(client.Send)
		client.Conn.Close()
	}
	wsm.mutex.Unlock()
}

// RegisterClient 注册客户端
func (wsm *WSManager) RegisterClient(client *Client) {
	wsm.register <- client
	
	// 启动客户端读写协程
	go wsm.clientWritePump(client)
	go wsm.clientReadPump(client)
}

// registerClient 内部注册客户端方法
func (wsm *WSManager) registerClient(client *Client) {
	wsm.mutex.Lock()
	defer wsm.mutex.Unlock()

	// 添加到客户端映射
	wsm.clients[client.ID] = client

	// 添加到频道映射
	if wsm.channels[client.Channel] == nil {
		wsm.channels[client.Channel] = make(map[string]*Client)
	}
	wsm.channels[client.Channel][client.ID] = client

	log.Printf("Client registered: %s in channel %s, total clients: %d", 
		client.ID, client.Channel, len(wsm.clients))
}

// unregisterClient 注销客户端
func (wsm *WSManager) unregisterClient(client *Client) {
	wsm.mutex.Lock()
	defer wsm.mutex.Unlock()

	// 从客户端映射中删除
	if _, ok := wsm.clients[client.ID]; ok {
		delete(wsm.clients, client.ID)
		close(client.Send)
	}

	// 从频道映射中删除
	if channelClients, ok := wsm.channels[client.Channel]; ok {
		delete(channelClients, client.ID)
		
		// 如果频道中没有客户端，删除频道
		if len(channelClients) == 0 {
			delete(wsm.channels, client.Channel)
		}
	}

	client.Conn.Close()
	log.Printf("Client unregistered: %s from channel %s, total clients: %d", 
		client.ID, client.Channel, len(wsm.clients))
}

// BroadcastToChannel 向指定频道广播消息
func (wsm *WSManager) BroadcastToChannel(channel string, data interface{}) {
	wsm.broadcast <- BroadcastMessage{
		Channel: channel,
		Data:    data,
	}
}

// broadcastMessage 内部广播消息方法
func (wsm *WSManager) broadcastMessage(msg BroadcastMessage) {
	wsm.mutex.RLock()
	channelClients, ok := wsm.channels[msg.Channel]
	wsm.mutex.RUnlock()

	if !ok {
		return
	}

	// 序列化消息
	messageBytes, err := json.Marshal(msg.Data)
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return
	}

	// 向频道中的所有客户端发送消息
	for _, client := range channelClients {
		select {
		case client.Send <- messageBytes:
		default:
			// 发送失败，注销客户端
			wsm.unregister <- client
		}
	}
}

// clientReadPump 客户端读消息泵
func (wsm *WSManager) clientReadPump(client *Client) {
	defer func() {
		wsm.unregister <- client
	}()

	// 设置读取参数
	client.Conn.SetReadLimit(512)
	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}

// clientWritePump 客户端写消息泵
func (wsm *WSManager) clientWritePump(client *Client) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// 发送消息
			if err := client.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}

		case <-ticker.C:
			// 发送心跳
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// GetStats 获取WebSocket统计信息
func (wsm *WSManager) GetStats() map[string]interface{} {
	wsm.mutex.RLock()
	defer wsm.mutex.RUnlock()

	channelStats := make(map[string]int)
	for channel, clients := range wsm.channels {
		channelStats[channel] = len(clients)
	}

	return map[string]interface{}{
		"total_clients":   len(wsm.clients),
		"channel_stats":   channelStats,
		"running":        wsm.running,
		"timestamp":      time.Now(),
	}
}