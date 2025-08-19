package manager

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/iot-go-sdk/pkg/config"
	"github.com/iot-go-sdk/pkg/framework/core"
	"github.com/iot-go-sdk/pkg/framework/plugins/mqtt"
	"github.com/iot-go-sdk/pkg/framework/plugins/ota"
	"znb/iot-uplink-gen/simulator"
)

// DeviceStatus 设备状态
type DeviceStatus string

const (
	StatusStopped    DeviceStatus = "stopped"
	StatusStarting   DeviceStatus = "starting"
	StatusRunning    DeviceStatus = "running"
	StatusStopping   DeviceStatus = "stopping"
	StatusError      DeviceStatus = "error"
	StatusRestarting DeviceStatus = "restarting"
)

// ManagedDevice 管理设备，封装framework实例
type ManagedDevice struct {
	// 设备基本信息
	deviceInfo   *DeviceInfo
	template     *DeviceTemplate
	globalConfig *GlobalConfig

	// Framework实例和组件
	framework       core.Framework
	simulatedDevice *simulator.SimulatedDevice
	factory         *simulator.DeviceFactory

	// 状态管理
	status          DeviceStatus
	lastError       error
	startTime       time.Time
	lastHeartbeat   time.Time
	restartCount    int
	maxRestartCount int

	// 控制和同步
	ctx        context.Context
	cancel     context.CancelFunc
	mutex      sync.RWMutex
	stopCh     chan struct{}
	statusCh   chan DeviceStatus

	// 监控和日志
	stats       *DeviceStats
	logCallback func(deviceID, level, message string)
}

// DeviceStats 设备统计信息
type DeviceStats struct {
	StartTime         time.Time                `json:"start_time"`
	LastHeartbeat     time.Time                `json:"last_heartbeat"`
	RestartCount      int                      `json:"restart_count"`
	ConnectionStatus  string                   `json:"connection_status"`
	SimulatorStats    simulator.SimulatorStats `json:"simulator_stats"`
	ErrorCount        int64                    `json:"error_count"`
	LastError         string                   `json:"last_error"`
	TotalUptime       time.Duration            `json:"total_uptime"`
}

// NewManagedDevice 创建管理设备
func NewManagedDevice(deviceInfo *DeviceInfo, template *DeviceTemplate, globalConfig *GlobalConfig) *ManagedDevice {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &ManagedDevice{
		deviceInfo:      deviceInfo,
		template:        template,
		globalConfig:    globalConfig,
		status:          StatusStopped,
		ctx:             ctx,
		cancel:          cancel,
		stopCh:          make(chan struct{}),
		statusCh:        make(chan DeviceStatus, 10),
		maxRestartCount: 5,
		stats: &DeviceStats{
			ConnectionStatus: "disconnected",
		},
	}
}

// Start 启动设备
func (md *ManagedDevice) Start() error {
	md.mutex.Lock()
	defer md.mutex.Unlock()

	if md.status == StatusRunning || md.status == StatusStarting {
		return fmt.Errorf("设备[%s]已经运行或正在启动", md.deviceInfo.DeviceID)
	}

	md.log("info", "开始启动设备...")
	md.setStatus(StatusStarting)

	// 重置状态
	md.lastError = nil
	md.startTime = time.Now()
	md.stats.StartTime = md.startTime

	// 启动设备
	go md.runDevice()

	return nil
}

// Stop 停止设备
func (md *ManagedDevice) Stop() error {
	md.mutex.Lock()
	defer md.mutex.Unlock()

	if md.status == StatusStopped || md.status == StatusStopping {
		return nil
	}

	md.log("info", "开始停止设备...")
	md.setStatus(StatusStopping)

	// 取消context
	md.cancel()

	// 等待停止完成（最多30秒）
	go func() {
		timer := time.NewTimer(30 * time.Second)
		defer timer.Stop()

		select {
		case <-md.stopCh:
			md.log("info", "设备已正常停止")
		case <-timer.C:
			md.log("warn", "设备停止超时，强制停止")
		}

		md.mutex.Lock()
		md.setStatus(StatusStopped)
		md.mutex.Unlock()
	}()

	return nil
}

// Restart 重启设备
func (md *ManagedDevice) Restart() error {
	md.mutex.Lock()
	defer md.mutex.Unlock()

	if md.status == StatusRestarting {
		return fmt.Errorf("设备[%s]正在重启中", md.deviceInfo.DeviceID)
	}

	md.log("info", "开始重启设备...")
	md.setStatus(StatusRestarting)
	md.restartCount++

	// 异步重启
	go func() {
		// 停止当前实例
		if md.framework != nil {
			md.framework.Stop()
		}

		// 等待一段时间
		time.Sleep(2 * time.Second)

		// 重新启动
		if err := md.runDeviceInternal(); err != nil {
			md.mutex.Lock()
			md.lastError = err
			md.setStatus(StatusError)
			md.mutex.Unlock()
			md.log("error", fmt.Sprintf("重启失败: %v", err))
		}
	}()

	return nil
}

// runDevice 运行设备主循环
func (md *ManagedDevice) runDevice() {
	defer func() {
		if r := recover(); r != nil {
			md.mutex.Lock()
			md.lastError = fmt.Errorf("设备运行异常: %v", r)
			md.setStatus(StatusError)
			md.mutex.Unlock()
			md.log("error", fmt.Sprintf("设备运行异常: %v", r))
		}
	}()

	if err := md.runDeviceInternal(); err != nil {
		md.mutex.Lock()
		md.lastError = err
		md.setStatus(StatusError)
		md.mutex.Unlock()
		md.log("error", fmt.Sprintf("设备启动失败: %v", err))
		return
	}

	// 启动心跳监控
	go md.heartbeatLoop()

	// 设置为运行状态
	md.mutex.Lock()
	md.setStatus(StatusRunning)
	md.mutex.Unlock()
	md.log("info", "设备启动成功")

	// 等待停止信号
	<-md.ctx.Done()

	// 清理资源
	md.cleanup()
	
	select {
	case md.stopCh <- struct{}{}:
	default:
	}
}

// runDeviceInternal 内部设备运行逻辑
func (md *ManagedDevice) runDeviceInternal() error {
	md.log("info", "开始内部设备运行流程...")
	
	// 1. 生成框架配置
	md.log("info", "生成框架配置...")
	coreCfg, err := md.deviceInfo.GenerateDeviceConfig(md.template, md.globalConfig)
	if err != nil {
		return fmt.Errorf("生成设备配置失败: %v", err)
	}
	md.log("info", "框架配置生成完成")

	// 2. 创建framework实例
	md.log("info", "创建framework实例...")
	md.framework = core.New(*coreCfg)
	md.log("info", "framework实例创建完成")

	// 3. 初始化framework
	md.log("info", "初始化framework...")
	if err := md.framework.Initialize(*coreCfg); err != nil {
		return fmt.Errorf("初始化framework失败: %v", err)
	}
	md.log("info", "framework初始化完成")

	// 4. 创建插件配置
	pluginCfg := &config.Config{
		Device: config.DeviceConfig{
			ProductKey:   md.deviceInfo.ProductKey,
			DeviceName:   md.deviceInfo.DeviceName,
			DeviceSecret: md.deviceInfo.DeviceSecret,
		},
		MQTT: config.MQTTConfig{
			Host:         coreCfg.MQTT.Host,
			Port:         coreCfg.MQTT.Port,
			UseTLS:       coreCfg.MQTT.UseTLS,
			KeepAlive:    time.Duration(coreCfg.MQTT.KeepAlive) * time.Second,
			CleanSession: coreCfg.MQTT.CleanSession,
		},
	}

	// 5. 加载插件
	if err := md.framework.LoadPlugin(mqtt.NewMQTTPlugin(pluginCfg)); err != nil {
		return fmt.Errorf("加载MQTT插件失败: %v", err)
	}

	if err := md.framework.LoadPlugin(ota.NewOTAPlugin()); err != nil {
		md.log("warn", fmt.Sprintf("加载OTA插件失败: %v", err))
	}

	// 6. 创建设备工厂和模拟设备
	md.factory = simulator.NewDeviceFactory(".")

	// 从模板文件创建设备
	md.simulatedDevice, err = md.factory.CreateDeviceFromFiles(
		md.deviceInfo.ProductKey,
		md.deviceInfo.DeviceName,
		md.deviceInfo.DeviceSecret,
		md.template.TSLFile,
		md.template.RuleFile,
	)
	if err != nil {
		return fmt.Errorf("创建模拟设备失败: %v", err)
	}

	// 7. 设置设备配置
	md.simulatedDevice.SetFramework(md.framework)
	interval := time.Duration(md.deviceInfo.GetUploadInterval(md.globalConfig.DefaultInterval)) * time.Second
	md.simulatedDevice.SetUploadInterval(interval)

	// 设置日志回调
	md.simulatedDevice.SetLogCallback(func(msg string) {
		md.log("info", msg)
	})

	// 8. 注册设备
	if err := md.framework.RegisterDevice(md.simulatedDevice); err != nil {
		return fmt.Errorf("注册设备失败: %v", err)
	}

	// 9. 启动framework
	md.log("info", "启动framework...")
	if err := md.framework.Start(); err != nil {
		return fmt.Errorf("启动framework失败: %v", err)
	}
	md.log("info", "framework启动完成")

	md.log("info", fmt.Sprintf("设备[%s]启动成功，上报间隔: %v", md.deviceInfo.DeviceID, interval))
	return nil
}

// cleanup 清理资源
func (md *ManagedDevice) cleanup() {
	md.log("info", "清理设备资源...")

	if md.framework != nil {
		if err := md.framework.Stop(); err != nil {
			md.log("warn", fmt.Sprintf("停止framework失败: %v", err))
		}
		md.framework = nil
	}

	md.simulatedDevice = nil
	md.factory = nil
	
	md.stats.ConnectionStatus = "disconnected"
	md.log("info", "设备资源清理完成")
}

// heartbeatLoop 心跳监控循环
func (md *ManagedDevice) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-md.ctx.Done():
			return
		case <-ticker.C:
			md.updateHeartbeat()
		}
	}
}

// updateHeartbeat 更新心跳
func (md *ManagedDevice) updateHeartbeat() {
	md.mutex.Lock()
	defer md.mutex.Unlock()

	md.lastHeartbeat = time.Now()
	md.stats.LastHeartbeat = md.lastHeartbeat

	// 更新统计信息
	if md.simulatedDevice != nil {
		md.stats.SimulatorStats = md.simulatedDevice.GetStats()
		md.stats.ConnectionStatus = "connected"
	}

	if md.status == StatusRunning {
		md.stats.TotalUptime = time.Since(md.startTime)
	}
}

// GetStatus 获取设备状态
func (md *ManagedDevice) GetStatus() DeviceStatus {
	md.mutex.RLock()
	defer md.mutex.RUnlock()
	return md.status
}

// GetStats 获取设备统计信息
func (md *ManagedDevice) GetStats() *DeviceStats {
	md.mutex.RLock()
	defer md.mutex.RUnlock()

	// 创建副本避免并发问题
	stats := *md.stats
	if md.lastError != nil {
		stats.LastError = md.lastError.Error()
	}
	return &stats
}

// GetDeviceInfo 获取设备信息
func (md *ManagedDevice) GetDeviceInfo() *DeviceInfo {
	return md.deviceInfo
}

// GetTemplate 获取设备模板
func (md *ManagedDevice) GetTemplate() *DeviceTemplate {
	return md.template
}

// setStatus 设置状态
func (md *ManagedDevice) setStatus(status DeviceStatus) {
	md.status = status
	
	// 非阻塞发送状态变更通知
	select {
	case md.statusCh <- status:
	default:
	}
}

// SetLogCallback 设置日志回调
func (md *ManagedDevice) SetLogCallback(callback func(deviceID, level, message string)) {
	md.logCallback = callback
}

// log 记录日志
func (md *ManagedDevice) log(level, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logMsg := fmt.Sprintf("[%s] [%s] %s", timestamp, md.deviceInfo.DeviceID, message)
	
	// 控制台输出
	log.Println(logMsg)
	
	// 回调输出
	if md.logCallback != nil {
		md.logCallback(md.deviceInfo.DeviceID, level, message)
	}
}

// IsHealthy 检查设备是否健康
func (md *ManagedDevice) IsHealthy() bool {
	md.mutex.RLock()
	defer md.mutex.RUnlock()

	if md.status != StatusRunning {
		return false
	}

	// 检查心跳是否超时（5分钟）
	if time.Since(md.lastHeartbeat) > 5*time.Minute {
		return false
	}

	return true
}

// ShouldRestart 检查是否应该重启
func (md *ManagedDevice) ShouldRestart() bool {
	md.mutex.RLock()
	defer md.mutex.RUnlock()

	return md.status == StatusError && md.restartCount < md.maxRestartCount
}

// GetStatusChannel 获取状态变更通道
func (md *ManagedDevice) GetStatusChannel() <-chan DeviceStatus {
	return md.statusCh
}