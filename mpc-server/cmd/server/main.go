package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"mpc-server/internal/config"
	"mpc-server/internal/handlers"
	"mpc-server/internal/mpc"
	"mpc-server/internal/peer"
	"mpc-server/internal/websocket"
)

func main() {
	// 解析命令行参数
	var serverID string
	flag.StringVar(&serverID, "server", "", "Server ID (third-party, enterprise, mobile-app)")
	flag.Parse()

	if serverID == "" {
		fmt.Println("Usage: go run main.go -server <server-id>")
		fmt.Println("Available servers: third-party, enterprise, mobile-app")
		os.Exit(1)
	}

	// 获取服务器配置
	serverConfig, err := config.GetServerConfig(serverID)
	if err != nil {
		log.Fatalf("Failed to get server config for %s: %v", serverID, err)
	}

	log.Printf("Starting MPC Server: %s (%s)", serverConfig.Name, serverID)
	log.Printf("Connection Mode: %s", serverConfig.ConnectionMode)
	log.Printf("Capabilities: %v", serverConfig.Capabilities)

	// 根据连接模式决定启动方式
	if serverConfig.ConnectionMode == "client" {
		// 客户端模式：不启动HTTP服务器，只作为客户端连接到服务器
		startClientMode(serverID, serverConfig)
	} else {
		// 服务器模式：启动HTTP服务器等待连接
		startServerMode(serverID, serverConfig)
	}
}

// startClientMode 启动客户端模式
func startClientMode(serverID string, serverConfig *config.ServerConfig) {
	log.Printf("Starting in client mode, will connect to: %s", serverConfig.ServerURL)

	// 创建客户端管理器
	clientManager := peer.NewClientManager(serverID, serverConfig.ServerURL, serverConfig.AutoDisconnect)

	// 这里可以添加客户端特定的逻辑
	// 例如：等待用户输入或API调用来触发连接和操作

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Printf("Shutting down client %s...", serverID)
	clientManager.Disconnect()
}

// setupServerComponents 设置服务器组件
func setupServerComponents(serverID string) (*websocket.Hub, *handlers.Handler, *mpc.MPCManager, error) {
	// 创建组件
	wsHub := websocket.NewHub()

	// 先创建handler来获取peerClient
	tempHandler, err := handlers.NewHandler(serverID, nil, wsHub)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create temp handler: %v", err)
	}

	// 使用peerClient和wsHub创建MPCManager
	mpcManager := mpc.NewMPCManager(serverID, tempHandler.GetPeerClient(), wsHub)

	// 设置mpcManager到handler中
	handler := tempHandler
	handler.SetMPCManager(mpcManager)

	return wsHub, handler, mpcManager, nil
}

// setupRouter 设置Gin路由
func setupRouter(serverID string, serverConfig *config.ServerConfig, handler *handlers.Handler, wsHub *websocket.Hub) *gin.Engine {
	// 设置Gin路由
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// 添加CORS中间件
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// 只有启用HTTP API的服务器才设置API路由
	if serverConfig.EnableHTTPAPI {
		// API路由
		api := router.Group("/api/v1")
		{
			// 服务器信息
			api.GET("/info", handler.GetServerInfo)

			// 会话管理
			api.GET("/sessions", handler.ListSessions)
			api.GET("/sessions/:sessionId", handler.GetSessionStatus)

			// MPC操作
			api.POST("/keygen", handler.InitKeygen)
			api.POST("/reshare", handler.InitReshare)
			api.POST("/sign", handler.InitSign)
		}
		log.Printf("HTTP API enabled for server %s", serverID)
	} else {
		log.Printf("HTTP API disabled for server %s (协作节点模式)", serverID)
	}

	// WebSocket路由
	router.GET("/ws", func(c *gin.Context) {
		clientID := c.Query("client_id")
		if clientID == "" {
			clientID = serverID // 默认使用服务器ID作为客户端ID
		}

		wsHub.HandleWebSocket(c.Writer, c.Request, clientID)
	})

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"server_id": serverID,
			"name":      serverConfig.Name,
		})
	})

	return router
}

// startServerMode 启动服务器模式
func startServerMode(serverID string, serverConfig *config.ServerConfig) {
	log.Printf("Starting in server mode on port %d", serverConfig.Port)

	// 设置服务器组件
	wsHub, handler, _, err := setupServerComponents(serverID)
	if err != nil {
		log.Fatalf("Failed to setup server components: %v", err)
	}

	// 启动WebSocket Hub
	go wsHub.Run()

	// 设置消息处理器
	wsHub.SetMessageHandler(handler)

	// 设置路由
	router := setupRouter(serverID, serverConfig, handler, wsHub)

	// 启动服务器
	addr := fmt.Sprintf(":%d", serverConfig.Port)
	log.Printf("Server %s listening on %s", serverID, addr)

	// 优雅关闭
	go func() {
		if err := router.Run(addr); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 连接到peer服务器
	go func() {
		log.Printf("Connecting to peer servers...")
		if err := handler.ConnectToPeers(); err != nil {
			log.Printf("Warning: Failed to connect to some peers: %v", err)
		} else {
			log.Printf("Successfully connected to peer servers")
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Printf("Shutting down server %s...", serverID)
}
