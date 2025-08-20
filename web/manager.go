package web

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	
	"znb/iot-uplink-gen/web/api"
	"znb/iot-uplink-gen/web/middleware"
	wsManager "znb/iot-uplink-gen/web/websocket"
)

// WebManager Web管理器
type WebManager struct {
	router     *gin.Engine
	server     *http.Server
	wsManager  *wsManager.WSManager
	config     *Config
}

// Config Web配置
type Config struct {
	Port      int    `json:"port"`
	Host      string `json:"host"`
	Debug     bool   `json:"debug"`
	StaticDir string `json:"static_dir"`
}

// DeviceManager 设备管理器接口
type DeviceManager interface {
	GetDevices() ([]DeviceInfo, error)
	GetDevice(id string) (*DeviceInfo, error)
	StartDevice(id string) error
	StopDevice(id string) error
	UpdateDevice(id string, config interface{}) error
}

// DeviceInfo 设备信息
type DeviceInfo struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	Status       string                 `json:"status"`
	LastReport   time.Time              `json:"last_report"`
	Properties   map[string]interface{} `json:"properties"`
	Config       map[string]interface{} `json:"config"`
}

// NewWebManager 创建Web管理器
func NewWebManager(config *Config) *WebManager {
	if config == nil {
		config = &Config{
			Port:      8080,
			Host:      "0.0.0.0",
			Debug:     false,
			StaticDir: "web/static",
		}
	}

	// 设置Gin模式
	if !config.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	
	// 添加中间件
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.CORS())

	// 创建WebSocket管理器
	wsMgr := wsManager.NewWSManager()

	wm := &WebManager{
		router:    router,
		wsManager: wsMgr,
		config:    config,
	}

	// 设置路由
	wm.setupRoutes()

	return wm
}

// setupRoutes 设置路由
func (wm *WebManager) setupRoutes() {
	// 静态文件服务
	wm.router.Static("/static", wm.config.StaticDir)
	
	// 模板文件
	templatesPath := "web/templates/*.html"
	if _, err := filepath.Glob(templatesPath); err == nil {
		wm.router.LoadHTMLGlob(templatesPath)
	}

	// 主页
	wm.router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "IoT设备管理器",
		})
	})

	// API路由组
	apiGroup := wm.router.Group("/api/v1")
	api.SetupRoutes(apiGroup, wm.wsManager)

	// WebSocket路由
	wm.router.GET("/ws", wm.handleWebSocket)
	wm.router.GET("/ws/devices", wm.handleDeviceWebSocket)
	wm.router.GET("/ws/logs", wm.handleLogWebSocket)

	// 健康检查
	wm.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"timestamp": time.Now(),
		})
	})
}

// handleWebSocket 处理WebSocket连接
func (wm *WebManager) handleWebSocket(c *gin.Context) {
	conn, err := wsManager.DefaultUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := wsManager.NewClient("general", conn)
	wm.wsManager.RegisterClient(client)
	
	log.Printf("WebSocket client connected: %s", client.ID)
}

// handleDeviceWebSocket 处理设备WebSocket连接
func (wm *WebManager) handleDeviceWebSocket(c *gin.Context) {
	conn, err := wsManager.DefaultUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Device WebSocket upgrade failed: %v", err)
		return
	}

	client := wsManager.NewClient("devices", conn)
	wm.wsManager.RegisterClient(client)
	
	log.Printf("Device WebSocket client connected: %s", client.ID)
}

// handleLogWebSocket 处理日志WebSocket连接
func (wm *WebManager) handleLogWebSocket(c *gin.Context) {
	conn, err := wsManager.DefaultUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Log WebSocket upgrade failed: %v", err)
		return
	}

	client := wsManager.NewClient("logs", conn)
	wm.wsManager.RegisterClient(client)
	
	log.Printf("Log WebSocket client connected: %s", client.ID)
}

// Start 启动Web服务器
func (wm *WebManager) Start() error {
	addr := fmt.Sprintf("%s:%d", wm.config.Host, wm.config.Port)
	
	wm.server = &http.Server{
		Addr:    addr,
		Handler: wm.router,
	}

	// 启动WebSocket管理器
	go wm.wsManager.Start()

	log.Printf("Starting Web server on %s", addr)
	
	// 启动服务器
	go func() {
		if err := wm.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Web server start failed: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	
	log.Println("Shutting down Web server...")

	// 优雅关闭
	return wm.Stop()
}

// Stop 停止Web服务器
func (wm *WebManager) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 停止WebSocket管理器
	wm.wsManager.Stop()

	// 关闭HTTP服务器
	if wm.server != nil {
		if err := wm.server.Shutdown(ctx); err != nil {
			log.Printf("Web server shutdown error: %v", err)
			return err
		}
	}

	log.Println("Web server stopped")
	return nil
}

// BroadcastDeviceUpdate 广播设备更新
func (wm *WebManager) BroadcastDeviceUpdate(deviceID string, data interface{}) {
	message := map[string]interface{}{
		"type":      "device_update",
		"device_id": deviceID,
		"data":      data,
		"timestamp": time.Now(),
	}
	wm.wsManager.BroadcastToChannel("devices", message)
}

// BroadcastLogMessage 广播日志消息
func (wm *WebManager) BroadcastLogMessage(level, message string) {
	logMessage := map[string]interface{}{
		"type":      "log",
		"level":     level,
		"message":   message,
		"timestamp": time.Now(),
	}
	wm.wsManager.BroadcastToChannel("logs", logMessage)
}