package process

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"znb/iot-uplink-gen/manager"
)

// ProcessStatus 进程状态
type ProcessStatus string

const (
	ProcessStatusStopped  ProcessStatus = "stopped"
	ProcessStatusStarting ProcessStatus = "starting"
	ProcessStatusRunning  ProcessStatus = "running"
	ProcessStatusStopping ProcessStatus = "stopping"
	ProcessStatusError    ProcessStatus = "error"
	ProcessStatusCrashed  ProcessStatus = "crashed"
)

// DeviceProcess 设备进程信息
type DeviceProcess struct {
	DeviceID     string                 `json:"device_id"`
	ProcessID    int                    `json:"process_id"`
	Status       ProcessStatus          `json:"status"`
	StartTime    time.Time              `json:"start_time"`
	Command      []string               `json:"command"`
	ConfigFile   string                 `json:"config_file"`
	LogFile      string                 `json:"log_file"`
	RestartCount int                    `json:"restart_count"`
	LastError    string                 `json:"last_error"`
	
	// 进程控制
	cmd        *exec.Cmd
	ctx        context.Context
	cancel     context.CancelFunc
	stdout     io.ReadCloser
	stderr     io.ReadCloser
	
	// 监控
	mutex      sync.RWMutex
	lastOutput time.Time
	outputCh   chan string
}

// ProcessManager 进程管理器
type ProcessManager struct {
	processes     map[string]*DeviceProcess // deviceID -> DeviceProcess
	config        *manager.MultiDeviceConfig
	templates     map[string]*manager.DeviceTemplate
	
	// 配置
	executablePath string
	workDir        string
	logDir         string
	configDir      string
	
	// 控制
	ctx           context.Context
	cancel        context.CancelFunc
	mutex         sync.RWMutex
	running       bool
	
	// 监控
	eventCh       chan ProcessEvent
	maxRestarts   int
	restartDelay  time.Duration
}

// ProcessEvent 进程事件
type ProcessEvent struct {
	DeviceID    string        `json:"device_id"`
	Type        string        `json:"type"`        // start, stop, crash, restart, output
	Status      ProcessStatus `json:"status"`
	Message     string        `json:"message"`
	Timestamp   time.Time     `json:"timestamp"`
	ProcessID   int           `json:"process_id,omitempty"`
	Error       error         `json:"error,omitempty"`
}

// NewProcessManager 创建进程管理器
func NewProcessManager(executablePath, workDir string) *ProcessManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &ProcessManager{
		processes:      make(map[string]*DeviceProcess),
		templates:      make(map[string]*manager.DeviceTemplate),
		executablePath: executablePath,
		workDir:        workDir,
		logDir:         filepath.Join(workDir, "logs"),
		configDir:      filepath.Join(workDir, "configs", "processes"),
		ctx:            ctx,
		cancel:         cancel,
		eventCh:        make(chan ProcessEvent, 100),
		maxRestarts:    5,
		restartDelay:   10 * time.Second,
	}
}

// LoadConfig 加载配置
func (pm *ProcessManager) LoadConfig(configPath, templatePath string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// 加载多设备配置
	config, err := manager.LoadMultiDeviceConfig(configPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %v", err)
	}
	pm.config = config

	// 加载设备模板
	if err := pm.loadTemplates(templatePath); err != nil {
		return fmt.Errorf("加载模板失败: %v", err)
	}

	// 创建必要的目录
	if err := os.MkdirAll(pm.logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %v", err)
	}
	
	if err := os.MkdirAll(pm.configDir, 0755); err != nil {
		return fmt.Errorf("创建进程配置目录失败: %v", err)
	}

	log.Printf("进程管理器配置加载完成，已加载 %d 个模板", len(pm.templates))
	return nil
}

// loadTemplates 加载设备模板
func (pm *ProcessManager) loadTemplates(templatePath string) error {
	entries, err := os.ReadDir(templatePath)
	if err != nil {
		return fmt.Errorf("读取模板目录失败: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(templatePath, entry.Name())
		template, err := manager.LoadDeviceTemplate(fullPath)
		if err != nil {
			log.Printf("加载模板[%s]失败: %v", entry.Name(), err)
			continue
		}

		pm.templates[entry.Name()] = template
		log.Printf("加载模板: %s (%s)", template.Name, template.ProductType)
	}

	return nil
}

// Start 启动进程管理器
func (pm *ProcessManager) Start() error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if pm.running {
		return fmt.Errorf("进程管理器已经运行")
	}

	log.Println("启动进程管理器...")

	// 启动进程监控
	go pm.processMonitor()

	// 启动所有启用的设备进程
	enabledDevices := pm.config.GetEnabledDevices()
	log.Printf("发现 %d 个启用的设备", len(enabledDevices))

	var startErrors []string
	for i, deviceInfo := range enabledDevices {
		log.Printf("正在启动第 %d/%d 个设备进程: %s", i+1, len(enabledDevices), deviceInfo.DeviceID)
		if err := pm.startDeviceProcess(&deviceInfo); err != nil {
			errorMsg := fmt.Sprintf("启动设备进程[%s]失败: %v", deviceInfo.DeviceID, err)
			startErrors = append(startErrors, errorMsg)
			log.Printf("错误: %s", errorMsg)
		}
	}

	pm.running = true
	
	if len(startErrors) > 0 {
		log.Printf("部分设备进程启动失败: %d/%d", len(startErrors), len(enabledDevices))
	} else {
		log.Printf("所有设备进程启动成功: %d 个进程", len(enabledDevices))
	}

	return nil
}

// Stop 停止进程管理器
func (pm *ProcessManager) Stop() error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if !pm.running {
		return nil
	}

	log.Println("停止进程管理器...")

	// 停止所有设备进程
	var wg sync.WaitGroup
	for deviceID := range pm.processes {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if err := pm.stopDeviceProcess(id); err != nil {
				log.Printf("停止设备进程[%s]失败: %v", id, err)
			}
		}(deviceID)
	}

	// 等待所有进程停止（最多30秒）
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("所有设备进程已停止")
	case <-time.After(30 * time.Second):
		log.Println("设备进程停止超时，强制终止...")
		pm.forceStopAllProcesses()
	}

	// 取消context
	pm.cancel()
	pm.running = false

	log.Println("进程管理器已停止")
	return nil
}

// startDeviceProcess 启动设备进程
func (pm *ProcessManager) startDeviceProcess(deviceInfo *manager.DeviceInfo) error {
	// 查找设备组获取模板信息
	_, group, err := pm.config.GetDeviceByID(deviceInfo.DeviceID)
	if err != nil {
		return err
	}

	// 获取模板
	template, exists := pm.templates[group.Template]
	if !exists {
		return fmt.Errorf("模板[%s]不存在", group.Template)
	}

	// 生成进程专用配置文件
	processConfigFile, err := pm.generateProcessConfig(deviceInfo, template)
	if err != nil {
		return fmt.Errorf("生成进程配置失败: %v", err)
	}

	// 创建进程上下文
	ctx, cancel := context.WithCancel(pm.ctx)

	// 构建命令行参数
	logFile := filepath.Join(pm.logDir, fmt.Sprintf("%s.log", deviceInfo.DeviceID))
	cmd := exec.CommandContext(ctx, pm.executablePath,
		"-mode", "simulator",
		"-product", template.ProductType,
		"-config", processConfigFile,
	)
	
	// 设置工作目录
	cmd.Dir = pm.workDir
	
	// 设置环境变量
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("DEVICE_ID=%s", deviceInfo.DeviceID),
		fmt.Sprintf("LOG_FILE=%s", logFile),
	)

	// 获取标准输出和错误输出管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("创建stdout管道失败: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		stdout.Close()
		return fmt.Errorf("创建stderr管道失败: %v", err)
	}

	// 创建设备进程对象
	deviceProcess := &DeviceProcess{
		DeviceID:     deviceInfo.DeviceID,
		Status:       ProcessStatusStarting,
		StartTime:    time.Now(),
		Command:      cmd.Args,
		ConfigFile:   processConfigFile,
		LogFile:      logFile,
		RestartCount: 0,
		cmd:          cmd,
		ctx:          ctx,
		cancel:       cancel,
		stdout:       stdout,
		stderr:       stderr,
		outputCh:     make(chan string, 100),
	}

	// 启动进程
	if err := cmd.Start(); err != nil {
		cancel()
		stdout.Close()
		stderr.Close()
		return fmt.Errorf("启动进程失败: %v", err)
	}

	deviceProcess.ProcessID = cmd.Process.Pid
	deviceProcess.Status = ProcessStatusRunning

	// 添加到进程列表
	pm.processes[deviceInfo.DeviceID] = deviceProcess

	// 启动输出监控
	go pm.monitorProcessOutput(deviceProcess)

	// 启动进程等待
	go pm.waitForProcess(deviceProcess)

	// 发送启动事件
	pm.sendEvent(ProcessEvent{
		DeviceID:  deviceInfo.DeviceID,
		Type:      "start",
		Status:    ProcessStatusRunning,
		Message:   "设备进程启动成功",
		Timestamp: time.Now(),
		ProcessID: deviceProcess.ProcessID,
	})

	log.Printf("设备进程[%s]启动成功，PID: %d", deviceInfo.DeviceID, deviceProcess.ProcessID)
	return nil
}

// generateProcessConfig 生成进程专用配置文件
func (pm *ProcessManager) generateProcessConfig(deviceInfo *manager.DeviceInfo, template *manager.DeviceTemplate) (string, error) {
	// 生成单设备配置
	config, err := deviceInfo.GenerateDeviceConfig(template, &pm.config.GlobalConfig)
	if err != nil {
		return "", err
	}

	// 保存到进程配置目录
	configFile := filepath.Join(pm.configDir, fmt.Sprintf("%s.json", deviceInfo.DeviceID))
	
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化配置失败: %v", err)
	}

	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return "", fmt.Errorf("保存配置文件失败: %v", err)
	}

	return configFile, nil
}

// stopDeviceProcess 停止设备进程
func (pm *ProcessManager) stopDeviceProcess(deviceID string) error {
	process, exists := pm.processes[deviceID]
	if !exists {
		return fmt.Errorf("设备进程[%s]不存在", deviceID)
	}

	process.mutex.Lock()
	defer process.mutex.Unlock()

	if process.Status == ProcessStatusStopped || process.Status == ProcessStatusStopping {
		return nil
	}

	log.Printf("停止设备进程[%s]，PID: %d", deviceID, process.ProcessID)
	process.Status = ProcessStatusStopping

	// 优雅停止
	if process.cmd != nil && process.cmd.Process != nil {
		if err := process.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			log.Printf("发送SIGTERM信号失败: %v", err)
		}

		// 等待5秒
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()

		select {
		case <-process.ctx.Done():
			log.Printf("设备进程[%s]已优雅停止", deviceID)
		case <-timer.C:
			log.Printf("设备进程[%s]停止超时，强制杀死", deviceID)
			if err := process.cmd.Process.Kill(); err != nil {
				log.Printf("强制杀死进程失败: %v", err)
			}
		}
	}

	// 取消context
	process.cancel()
	process.Status = ProcessStatusStopped

	// 关闭管道
	if process.stdout != nil {
		process.stdout.Close()
	}
	if process.stderr != nil {
		process.stderr.Close()
	}

	// 从进程列表移除
	delete(pm.processes, deviceID)

	// 发送停止事件
	pm.sendEvent(ProcessEvent{
		DeviceID:  deviceID,
		Type:      "stop",
		Status:    ProcessStatusStopped,
		Message:   "设备进程已停止",
		Timestamp: time.Now(),
		ProcessID: process.ProcessID,
	})

	return nil
}

// monitorProcessOutput 监控进程输出
func (pm *ProcessManager) monitorProcessOutput(process *DeviceProcess) {
	// 监控stdout
	go func() {
		scanner := bufio.NewScanner(process.stdout)
		for scanner.Scan() {
			line := scanner.Text()
			select {
			case process.outputCh <- line:
			default:
				// 输出缓冲区满，丢弃
			}
			
			process.mutex.Lock()
			process.lastOutput = time.Now()
			process.mutex.Unlock()
		}
	}()

	// 监控stderr
	go func() {
		scanner := bufio.NewScanner(process.stderr)
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("[%s][ERROR] %s", process.DeviceID, line)
		}
	}()

	// 处理输出
	for {
		select {
		case <-process.ctx.Done():
			return
		case line := <-process.outputCh:
			log.Printf("[%s] %s", process.DeviceID, line)
		}
	}
}

// waitForProcess 等待进程结束
func (pm *ProcessManager) waitForProcess(process *DeviceProcess) {
	if err := process.cmd.Wait(); err != nil {
		process.mutex.Lock()
		process.Status = ProcessStatusCrashed
		process.LastError = err.Error()
		process.mutex.Unlock()

		pm.sendEvent(ProcessEvent{
			DeviceID:  process.DeviceID,
			Type:      "crash",
			Status:    ProcessStatusCrashed,
			Message:   fmt.Sprintf("设备进程崩溃: %v", err),
			Timestamp: time.Now(),
			ProcessID: process.ProcessID,
			Error:     err,
		})

		log.Printf("设备进程[%s]崩溃: %v", process.DeviceID, err)

		// 尝试重启
		if process.RestartCount < pm.maxRestarts {
			time.Sleep(pm.restartDelay)
			pm.restartDeviceProcess(process.DeviceID)
		}
	} else {
		process.mutex.Lock()
		process.Status = ProcessStatusStopped
		process.mutex.Unlock()

		log.Printf("设备进程[%s]正常退出", process.DeviceID)
	}
}

// restartDeviceProcess 重启设备进程
func (pm *ProcessManager) restartDeviceProcess(deviceID string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	process, exists := pm.processes[deviceID]
	if !exists {
		return fmt.Errorf("设备进程[%s]不存在", deviceID)
	}

	// 增加重启次数
	process.RestartCount++

	log.Printf("重启设备进程[%s]，第 %d 次重启", deviceID, process.RestartCount)

	// 获取设备信息
	deviceInfo, _, err := pm.config.GetDeviceByID(deviceID)
	if err != nil {
		return err
	}

	// 停止当前进程
	pm.stopDeviceProcess(deviceID)

	// 启动新进程
	return pm.startDeviceProcess(deviceInfo)
}

// processMonitor 进程监控器
func (pm *ProcessManager) processMonitor() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			pm.checkProcessHealth()
		}
	}
}

// checkProcessHealth 检查进程健康状态
func (pm *ProcessManager) checkProcessHealth() {
	pm.mutex.RLock()
	processes := make([]*DeviceProcess, 0, len(pm.processes))
	for _, process := range pm.processes {
		processes = append(processes, process)
	}
	pm.mutex.RUnlock()

	for _, process := range processes {
		process.mutex.RLock()
		isHealthy := process.Status == ProcessStatusRunning &&
			time.Since(process.lastOutput) < 2*time.Minute
		shouldRestart := process.Status == ProcessStatusCrashed &&
			process.RestartCount < pm.maxRestarts
		process.mutex.RUnlock()

		if !isHealthy && shouldRestart {
			log.Printf("设备进程[%s]不健康，尝试重启", process.DeviceID)
			pm.restartDeviceProcess(process.DeviceID)
		}
	}
}

// forceStopAllProcesses 强制停止所有进程
func (pm *ProcessManager) forceStopAllProcesses() {
	for _, process := range pm.processes {
		if process.cmd != nil && process.cmd.Process != nil {
			process.cmd.Process.Kill()
		}
	}
}

// sendEvent 发送事件
func (pm *ProcessManager) sendEvent(event ProcessEvent) {
	select {
	case pm.eventCh <- event:
	default:
		// 事件通道满，丢弃事件
	}
}

// GetEventChannel 获取事件通道
func (pm *ProcessManager) GetEventChannel() <-chan ProcessEvent {
	return pm.eventCh
}

// GetProcessStats 获取进程统计信息
func (pm *ProcessManager) GetProcessStats() map[string]*DeviceProcess {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	stats := make(map[string]*DeviceProcess)
	for id, process := range pm.processes {
		// 创建副本以避免并发问题
		process.mutex.RLock()
		stats[id] = &DeviceProcess{
			DeviceID:     process.DeviceID,
			ProcessID:    process.ProcessID,
			Status:       process.Status,
			StartTime:    process.StartTime,
			Command:      process.Command,
			ConfigFile:   process.ConfigFile,
			LogFile:      process.LogFile,
			RestartCount: process.RestartCount,
			LastError:    process.LastError,
		}
		process.mutex.RUnlock()
	}

	return stats
}

// IsRunning 检查管理器是否运行
func (pm *ProcessManager) IsRunning() bool {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return pm.running
}