package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	wsManager "znb/iot-uplink-gen/web/websocket"
)

// SetupRoutes 设置API路由
func SetupRoutes(router *gin.RouterGroup, wsMgr *wsManager.WSManager) {
	// 设备管理路由
	deviceRoutes := router.Group("/devices")
	setupDeviceRoutes(deviceRoutes, wsMgr)

	// 系统管理路由
	systemRoutes := router.Group("/system")
	setupSystemRoutes(systemRoutes, wsMgr)

	// WebSocket统计路由
	wsRoutes := router.Group("/websocket")
	setupWebSocketRoutes(wsRoutes, wsMgr)
}

// setupDeviceRoutes 设置设备管理路由
func setupDeviceRoutes(router *gin.RouterGroup, wsMgr *wsManager.WSManager) {
	// 获取所有设备
	router.GET("", func(c *gin.Context) {
		// TODO: 实现获取设备列表
		devices := []map[string]interface{}{
			{
				"id":          "device1",
				"name":        "智能调速电机",
				"type":        "motor",
				"status":      "online",
				"last_report": time.Now(),
				"properties": map[string]interface{}{
					"speed":       1200,
					"temperature": 45.5,
					"voltage":     220,
				},
			},
			{
				"id":          "device2", 
				"name":        "智能空调",
				"type":        "air_conditioner",
				"status":      "online",
				"last_report": time.Now(),
				"properties": map[string]interface{}{
					"current_temperature": 25.5,
					"target_temperature":  26,
					"humidity":           65,
				},
			},
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    devices,
		})
	})

	// 获取单个设备
	router.GET("/:id", func(c *gin.Context) {
		deviceID := c.Param("id")
		
		// TODO: 从设备管理器获取设备信息
		device := map[string]interface{}{
			"id":          deviceID,
			"name":        "设备名称",
			"type":        "设备类型",
			"status":      "online",
			"last_report": time.Now(),
			"properties": map[string]interface{}{
				"property1": "value1",
			},
			"config": map[string]interface{}{
				"ProductKey":    "FuWtDWoy",
				"DeviceName":    "AzEYXBjJY5",
				"DeviceSecret":  "***",
			},
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    device,
		})
	})

	// 启动设备
	router.POST("/:id/start", func(c *gin.Context) {
		deviceID := c.Param("id")
		
		// TODO: 调用设备管理器启动设备
		// err := deviceManager.StartDevice(deviceID)
		
		// 广播设备状态更新
		wsMgr.BroadcastToChannel("devices", map[string]interface{}{
			"type":      "device_status",
			"device_id": deviceID,
			"status":    "starting",
			"timestamp": time.Now(),
		})

		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "设备启动成功",
		})
	})

	// 停止设备
	router.POST("/:id/stop", func(c *gin.Context) {
		deviceID := c.Param("id")
		
		// TODO: 调用设备管理器停止设备
		// err := deviceManager.StopDevice(deviceID)
		
		// 广播设备状态更新
		wsMgr.BroadcastToChannel("devices", map[string]interface{}{
			"type":      "device_status",
			"device_id": deviceID,
			"status":    "stopping",
			"timestamp": time.Now(),
		})

		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "设备停止成功",
		})
	})

	// 调用设备服务
	router.POST("/:id/services/:service", func(c *gin.Context) {
		_ = c.Param("id")
		serviceName := c.Param("service")
		
		var params map[string]interface{}
		c.ShouldBindJSON(&params)
		
		// TODO: 调用设备服务
		// result, err := deviceManager.InvokeService(deviceID, serviceName, params)
		
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "服务调用成功",
			"data": map[string]interface{}{
				"service": serviceName,
				"params":  params,
				"result":  "调用成功",
			},
		})
	})

	// 设置设备属性
	router.PUT("/:id/properties", func(c *gin.Context) {
		_ = c.Param("id")
		
		var properties map[string]interface{}
		c.ShouldBindJSON(&properties)
		
		// TODO: 设置设备属性
		// err := deviceManager.SetProperties(deviceID, properties)
		
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "属性设置成功",
			"data":    properties,
		})
	})

	// 获取设备数据
	router.GET("/:id/data", func(c *gin.Context) {
		deviceID := c.Param("id")
		startTime := c.Query("start_time")
		endTime := c.Query("end_time")
		property := c.Query("property")
		
		// TODO: 查询设备历史数据
		data := []map[string]interface{}{
			{
				"timestamp": time.Now().Add(-10 * time.Minute),
				"values": map[string]interface{}{
					"temperature": 45.2,
					"speed":       1180,
				},
			},
			{
				"timestamp": time.Now().Add(-5 * time.Minute),
				"values": map[string]interface{}{
					"temperature": 46.1,
					"speed":       1205,
				},
			},
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": map[string]interface{}{
				"device_id":  deviceID,
				"start_time": startTime,
				"end_time":   endTime,
				"property":   property,
				"data":       data,
			},
		})
	})
}

// setupSystemRoutes 设置系统管理路由
func setupSystemRoutes(router *gin.RouterGroup, wsMgr *wsManager.WSManager) {
	// 系统状态
	router.GET("/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": map[string]interface{}{
				"uptime":         "1h 30m",
				"device_count":   2,
				"online_devices": 2,
				"cpu_usage":      "15%",
				"memory_usage":   "128MB",
				"timestamp":      time.Now(),
			},
		})
	})

	// 系统日志
	router.GET("/logs", func(c *gin.Context) {
		level := c.Query("level")
		limit := c.DefaultQuery("limit", "100")
		
		// TODO: 获取系统日志
		logs := []map[string]interface{}{
			{
				"timestamp": time.Now(),
				"level":     "info",
				"message":   "[device1] 设备已连接到IoT平台",
				"source":    "device1",
			},
			{
				"timestamp": time.Now().Add(-1 * time.Minute),
				"level":     "info", 
				"message":   "[device2] 属性上报成功: 8个属性",
				"source":    "device2",
			},
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": map[string]interface{}{
				"level": level,
				"limit": limit,
				"logs":  logs,
			},
		})
	})

	// 系统配置
	router.GET("/config", func(c *gin.Context) {
		// TODO: 获取系统配置
		config := map[string]interface{}{
			"web": map[string]interface{}{
				"port":  8080,
				"debug": false,
			},
			"device": map[string]interface{}{
				"scan_path": "configs",
				"report_interval": 30,
			},
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    config,
		})
	})

	// 更新系统配置
	router.PUT("/config", func(c *gin.Context) {
		var config map[string]interface{}
		c.ShouldBindJSON(&config)
		
		// TODO: 更新系统配置
		
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "配置更新成功",
			"data":    config,
		})
	})
}

// setupWebSocketRoutes 设置WebSocket路由
func setupWebSocketRoutes(router *gin.RouterGroup, wsMgr *wsManager.WSManager) {
	// WebSocket统计信息
	router.GET("/stats", func(c *gin.Context) {
		stats := wsMgr.GetStats()
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    stats,
		})
	})

	// 发送测试消息
	router.POST("/broadcast", func(c *gin.Context) {
		var request struct {
			Channel string      `json:"channel"`
			Data    interface{} `json:"data"`
		}
		
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    1,
				"message": "参数错误",
				"error":   err.Error(),
			})
			return
		}

		wsMgr.BroadcastToChannel(request.Channel, request.Data)

		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "消息发送成功",
		})
	})
}