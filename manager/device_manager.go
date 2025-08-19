package manager

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DeviceManager 设备管理器
type DeviceManager struct {
	// 配置
	config        *MultiDeviceConfig
	configPath    string
	templatePath  string

	// 设备管理
	devices       map[string]*ManagedDevice // deviceID -> ManagedDevice
	templates     map[string]*DeviceTemplate // templateName -> DeviceTemplate
	
	// 状态管理
	ctx           context.Context
	cancel        context.CancelFunc
	mutex         sync.RWMutex
	running       bool
	
	// 监控和日志
	logBuffer     []LogEntry
	logMutex      sync.RWMutex
	maxLogEntries int
	
	// 事件通知
	eventCh       chan DeviceEvent
	logCh         chan LogEntry
}

// DeviceEvent 设备事件
type DeviceEvent struct {
	DeviceID    string       `json:"device_id"`
	Type        string       `json:"type"`        // start, stop, error, status_change
	Status      DeviceStatus `json:"status"`
	Message     string       `json:"message"`
	Timestamp   time.Time    `json:"timestamp"`
	Error       error        `json:"error,omitempty"`
}

// LogEntry 日志条目
type LogEntry struct {
	DeviceID  string    `json:"device_id"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// ManagerStats 管理器统计信息
type ManagerStats struct {
	TotalDevices    int                        `json:"total_devices"`
	RunningDevices  int                        `json:"running_devices"`
	ErrorDevices    int                        `json:"error_devices"`
	DeviceStats     map[string]*DeviceStats    `json:"device_stats"`
	StartTime       time.Time                  `json:"start_time"`
	LastUpdate      time.Time                  `json:"last_update"`
}

// NewDeviceManager 创建设备管理器
func NewDeviceManager(configPath, templatePath string) *DeviceManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &DeviceManager{
		configPath:    configPath,
		templatePath:  templatePath,
		devices:       make(map[string]*ManagedDevice),
		templates:     make(map[string]*DeviceTemplate),
		ctx:           ctx,
		cancel:        cancel,
		eventCh:       make(chan DeviceEvent, 100),
		logCh:         make(chan LogEntry, 1000),
		maxLogEntries: 10000,
	}
}

// LoadConfig 加载配置
func (dm *DeviceManager) LoadConfig() error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	// 加载多设备配置
	dm.log("info", "manager", fmt.Sprintf("从 %s 加载配置文件...", dm.configPath))
	config, err := LoadMultiDeviceConfig(dm.configPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %v", err)
	}
	dm.config = config
	dm.log("info", "manager", "多设备配置加载成功")

	// 加载设备模板
	if err := dm.loadTemplates(); err != nil {
		return fmt.Errorf("加载模板失败: %v", err)
	}

	dm.log("info", "manager", fmt.Sprintf("配置加载完成，已加载 %d 个模板", len(dm.templates)))
	return nil
}

// loadTemplates 加载设备模板
func (dm *DeviceManager) loadTemplates() error {
	if dm.templatePath == "" {
		dm.templatePath = "configs/device_templates"
	}

	// 创建模板目录（如果不存在）
	if err := os.MkdirAll(dm.templatePath, 0755); err != nil {
		return fmt.Errorf("创建模板目录失败: %v", err)
	}

	// 扫描模板目录
	entries, err := os.ReadDir(dm.templatePath)
	if err != nil {
		return fmt.Errorf("读取模板目录失败: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		templatePath := filepath.Join(dm.templatePath, entry.Name())
		template, err := LoadDeviceTemplate(templatePath)
		if err != nil {
			dm.log("warn", "manager", fmt.Sprintf("加载模板[%s]失败: %v", entry.Name(), err))
			continue
		}

		dm.templates[entry.Name()] = template
		dm.log("info", "manager", fmt.Sprintf("加载模板: %s (%s)", template.Name, template.ProductType))
	}

	return nil
}

// Start 启动管理器
func (dm *DeviceManager) Start() error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	if dm.running {
		return fmt.Errorf("设备管理器已经运行")
	}

	dm.log("info", "manager", "启动设备管理器...")

	// 加载配置
	dm.log("info", "manager", "开始加载配置...")
	if err := dm.LoadConfig(); err != nil {
		return err
	}
	dm.log("info", "manager", "配置加载完成")

	// 启动日志处理器
	go dm.logProcessor()

	// 启动健康检查
	go dm.healthChecker()

	// 启动所有启用的设备
	enabledDevices := dm.config.GetEnabledDevices()
	dm.log("info", "manager", fmt.Sprintf("发现 %d 个启用的设备", len(enabledDevices)))

	var startErrors []string
	for i, deviceInfo := range enabledDevices {
		dm.log("info", "manager", fmt.Sprintf("正在启动第 %d/%d 个设备: %s", i+1, len(enabledDevices), deviceInfo.DeviceID))
		if err := dm.startDevice(&deviceInfo); err != nil {
			errorMsg := fmt.Sprintf("启动设备[%s]失败: %v", deviceInfo.DeviceID, err)
			startErrors = append(startErrors, errorMsg)
			dm.log("error", "manager", errorMsg)
		}
	}

	dm.running = true
	
	if len(startErrors) > 0 {
		dm.log("warn", "manager", fmt.Sprintf("部分设备启动失败: %d/%d", len(startErrors), len(enabledDevices)))
	} else {
		dm.log("info", "manager", fmt.Sprintf("所有设备启动成功: %d 个设备", len(enabledDevices)))
	}

	return nil
}

// Stop 停止管理器
func (dm *DeviceManager) Stop() error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	if !dm.running {
		return nil
	}

	dm.log("info", "manager", "停止设备管理器...")

	// 停止所有设备
	var wg sync.WaitGroup
	for deviceID := range dm.devices {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if err := dm.stopDeviceInternal(id); err != nil {
				dm.log("error", "manager", fmt.Sprintf("停止设备[%s]失败: %v", id, err))
			}
		}(deviceID)
	}

	// 等待所有设备停止（最多30秒）
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		dm.log("info", "manager", "所有设备已停止")
	case <-time.After(30 * time.Second):
		dm.log("warn", "manager", "设备停止超时")
	}

	// 取消context
	dm.cancel()
	dm.running = false

	dm.log("info", "manager", "设备管理器已停止")
	return nil
}

// startDevice 启动设备
func (dm *DeviceManager) startDevice(deviceInfo *DeviceInfo) error {
	// 查找设备组获取模板信息
	_, group, err := dm.config.GetDeviceByID(deviceInfo.DeviceID)
	if err != nil {
		return err
	}

	// 获取模板
	template, exists := dm.templates[group.Template]
	if !exists {
		return fmt.Errorf("模板[%s]不存在", group.Template)
	}

	// 创建管理设备
	managedDevice := NewManagedDevice(deviceInfo, template, &dm.config.GlobalConfig)
	
	// 设置日志回调
	managedDevice.SetLogCallback(func(deviceID, level, message string) {
		dm.logCh <- LogEntry{
			DeviceID:  deviceID,
			Level:     level,
			Message:   message,
			Timestamp: time.Now(),
		}
	})

	// 监听状态变化
	go dm.monitorDeviceStatus(managedDevice)

	// 启动设备
	dm.log("info", "manager", fmt.Sprintf("准备启动设备[%s]", deviceInfo.DeviceID))
	if err := managedDevice.Start(); err != nil {
		return err
	}
	dm.log("info", "manager", fmt.Sprintf("设备[%s]启动成功", deviceInfo.DeviceID))

	// 添加到设备列表
	dm.devices[deviceInfo.DeviceID] = managedDevice
	
	// 发送事件
	dm.sendEvent(DeviceEvent{
		DeviceID:  deviceInfo.DeviceID,
		Type:      "start",
		Status:    StatusStarting,
		Message:   "设备启动中",
		Timestamp: time.Now(),
	})

	return nil
}

// stopDeviceInternal 内部停止设备方法
func (dm *DeviceManager) stopDeviceInternal(deviceID string) error {
	device, exists := dm.devices[deviceID]
	if !exists {
		return fmt.Errorf("设备[%s]不存在", deviceID)
	}

	// 停止设备
	if err := device.Stop(); err != nil {
		return err
	}

	// 从设备列表移除
	delete(dm.devices, deviceID)

	// 发送事件
	dm.sendEvent(DeviceEvent{
		DeviceID:  deviceID,
		Type:      "stop",
		Status:    StatusStopped,
		Message:   "设备已停止",
		Timestamp: time.Now(),
	})

	return nil
}

// AddDevice 添加设备
func (dm *DeviceManager) AddDevice(deviceInfo *DeviceInfo, groupName string) error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	// 检查设备是否已存在
	if _, exists := dm.devices[deviceInfo.DeviceID]; exists {
		return fmt.Errorf("设备[%s]已存在", deviceInfo.DeviceID)
	}

	// 添加到配置
	var targetGroup *DeviceGroup
	for i := range dm.config.DeviceGroups {
		if dm.config.DeviceGroups[i].GroupName == groupName {
			targetGroup = &dm.config.DeviceGroups[i]
			break
		}
	}

	if targetGroup == nil {
		return fmt.Errorf("设备组[%s]不存在", groupName)
	}

	targetGroup.Devices = append(targetGroup.Devices, *deviceInfo)

	// 保存配置
	if err := SaveMultiDeviceConfig(dm.config, dm.configPath); err != nil {
		return fmt.Errorf("保存配置失败: %v", err)
	}

	// 如果设备启用且管理器运行中，立即启动设备
	if dm.running && deviceInfo.Enabled {
		if err := dm.startDevice(deviceInfo); err != nil {
			return fmt.Errorf("启动设备失败: %v", err)
		}
	}

	dm.log("info", "manager", fmt.Sprintf("添加设备[%s]到组[%s]", deviceInfo.DeviceID, groupName))
	return nil
}

// RemoveDevice 移除设备
func (dm *DeviceManager) RemoveDevice(deviceID string) error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	// 停止设备
	if err := dm.stopDeviceInternal(deviceID); err != nil {
		dm.log("warn", "manager", fmt.Sprintf("停止设备[%s]失败: %v", deviceID, err))
	}

	// 从配置中移除
	found := false
	for i := range dm.config.DeviceGroups {
		for j := range dm.config.DeviceGroups[i].Devices {
			if dm.config.DeviceGroups[i].Devices[j].DeviceID == deviceID {
				// 移除设备
				dm.config.DeviceGroups[i].Devices = append(
					dm.config.DeviceGroups[i].Devices[:j],
					dm.config.DeviceGroups[i].Devices[j+1:]...,
				)
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		return fmt.Errorf("设备[%s]在配置中不存在", deviceID)
	}

	// 保存配置
	if err := SaveMultiDeviceConfig(dm.config, dm.configPath); err != nil {
		return fmt.Errorf("保存配置失败: %v", err)
	}

	dm.log("info", "manager", fmt.Sprintf("移除设备[%s]", deviceID))
	return nil
}

// StartDevice 启动指定设备
func (dm *DeviceManager) StartDevice(deviceID string) error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	// 检查设备是否已运行
	if device, exists := dm.devices[deviceID]; exists {
		if device.GetStatus() == StatusRunning {
			return fmt.Errorf("设备[%s]已经运行", deviceID)
		}
		return device.Start()
	}

	// 从配置中查找设备
	deviceInfo, _, err := dm.config.GetDeviceByID(deviceID)
	if err != nil {
		return err
	}

	return dm.startDevice(deviceInfo)
}

// StopDevice 停止指定设备
func (dm *DeviceManager) StopDevice(deviceID string) error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	return dm.stopDeviceInternal(deviceID)
}

// RestartDevice 重启指定设备
func (dm *DeviceManager) RestartDevice(deviceID string) error {
	dm.mutex.RLock()
	device, exists := dm.devices[deviceID]
	dm.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("设备[%s]不存在", deviceID)
	}

	return device.Restart()
}

// GetDeviceStatus 获取设备状态
func (dm *DeviceManager) GetDeviceStatus(deviceID string) (DeviceStatus, error) {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	device, exists := dm.devices[deviceID]
	if !exists {
		return StatusStopped, fmt.Errorf("设备[%s]不存在", deviceID)
	}

	return device.GetStatus(), nil
}

// GetAllDeviceStats 获取所有设备统计信息
func (dm *DeviceManager) GetAllDeviceStats() *ManagerStats {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	stats := &ManagerStats{
		DeviceStats: make(map[string]*DeviceStats),
		LastUpdate:  time.Now(),
	}

	totalDevices := 0
	runningDevices := 0
	errorDevices := 0

	// 统计配置中的所有设备
	for _, group := range dm.config.DeviceGroups {
		totalDevices += len(group.Devices)
	}

	// 统计运行中的设备
	for deviceID, device := range dm.devices {
		deviceStats := device.GetStats()
		stats.DeviceStats[deviceID] = deviceStats

		switch device.GetStatus() {
		case StatusRunning:
			runningDevices++
		case StatusError:
			errorDevices++
		}
	}

	stats.TotalDevices = totalDevices
	stats.RunningDevices = runningDevices
	stats.ErrorDevices = errorDevices

	return stats
}

// monitorDeviceStatus 监控设备状态变化
func (dm *DeviceManager) monitorDeviceStatus(device *ManagedDevice) {
	statusCh := device.GetStatusChannel()
	deviceID := device.GetDeviceInfo().DeviceID

	for {
		select {
		case <-dm.ctx.Done():
			return
		case status := <-statusCh:
			dm.sendEvent(DeviceEvent{
				DeviceID:  deviceID,
				Type:      "status_change",
				Status:    status,
				Message:   fmt.Sprintf("设备状态变更为: %s", status),
				Timestamp: time.Now(),
			})
		}
	}
}

// healthChecker 健康检查器
func (dm *DeviceManager) healthChecker() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-dm.ctx.Done():
			return
		case <-ticker.C:
			dm.checkDeviceHealth()
		}
	}
}

// checkDeviceHealth 检查设备健康状态
func (dm *DeviceManager) checkDeviceHealth() {
	dm.mutex.RLock()
	devices := make([]*ManagedDevice, 0, len(dm.devices))
	for _, device := range dm.devices {
		devices = append(devices, device)
	}
	dm.mutex.RUnlock()

	for _, device := range devices {
		if !device.IsHealthy() && device.ShouldRestart() {
			dm.log("warn", "manager", fmt.Sprintf("设备[%s]不健康，尝试重启", device.GetDeviceInfo().DeviceID))
			
			if err := device.Restart(); err != nil {
				dm.log("error", "manager", fmt.Sprintf("重启设备[%s]失败: %v", device.GetDeviceInfo().DeviceID, err))
			}
		}
	}
}

// logProcessor 日志处理器
func (dm *DeviceManager) logProcessor() {
	for {
		select {
		case <-dm.ctx.Done():
			return
		case entry := <-dm.logCh:
			dm.addLogEntry(entry)
		}
	}
}

// addLogEntry 添加日志条目
func (dm *DeviceManager) addLogEntry(entry LogEntry) {
	dm.logMutex.Lock()
	defer dm.logMutex.Unlock()

	dm.logBuffer = append(dm.logBuffer, entry)
	
	// 限制日志缓冲区大小
	if len(dm.logBuffer) > dm.maxLogEntries {
		dm.logBuffer = dm.logBuffer[len(dm.logBuffer)-dm.maxLogEntries:]
	}
}

// GetLogs 获取日志
func (dm *DeviceManager) GetLogs(deviceID string, limit int) []LogEntry {
	dm.logMutex.RLock()
	defer dm.logMutex.RUnlock()

	var logs []LogEntry
	for i := len(dm.logBuffer) - 1; i >= 0; i-- {
		entry := dm.logBuffer[i]
		if deviceID == "" || entry.DeviceID == deviceID {
			logs = append(logs, entry)
			if limit > 0 && len(logs) >= limit {
				break
			}
		}
	}

	return logs
}

// sendEvent 发送事件
func (dm *DeviceManager) sendEvent(event DeviceEvent) {
	select {
	case dm.eventCh <- event:
	default:
		// 事件通道满，丢弃事件
	}
}

// GetEventChannel 获取事件通道
func (dm *DeviceManager) GetEventChannel() <-chan DeviceEvent {
	return dm.eventCh
}

// log 记录日志
func (dm *DeviceManager) log(level, deviceID, message string) {
	entry := LogEntry{
		DeviceID:  deviceID,
		Level:     level,
		Message:   message,
		Timestamp: time.Now(),
	}

	// 立即输出到控制台
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")
	log.Printf("[%s] [%s] [%s] %s", timestamp, level, deviceID, message)

	// 添加到日志缓冲区
	select {
	case dm.logCh <- entry:
	default:
		// 日志通道满，丢弃日志
	}
}

// IsRunning 检查管理器是否运行
func (dm *DeviceManager) IsRunning() bool {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()
	return dm.running
}

// GetConfig 获取配置
func (dm *DeviceManager) GetConfig() *MultiDeviceConfig {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()
	return dm.config
}

// GetTemplates 获取模板列表
func (dm *DeviceManager) GetTemplates() map[string]*DeviceTemplate {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()
	
	templates := make(map[string]*DeviceTemplate)
	for k, v := range dm.templates {
		templates[k] = v
	}
	return templates
}