package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/iot-go-sdk/pkg/config"
	"github.com/iot-go-sdk/pkg/framework/core"
	"github.com/iot-go-sdk/pkg/framework/plugins/mqtt"
	"github.com/iot-go-sdk/pkg/framework/plugins/ota"
	appConfig "znb/iot-uplink-gen/config"
	"znb/iot-uplink-gen/device"
	"znb/iot-uplink-gen/manager"
	"znb/iot-uplink-gen/process"
	"znb/iot-uplink-gen/simulator"
	"znb/iot-uplink-gen/web"
)

func main() {
	// 命令行参数
	mode := flag.String("mode", "sensor", "运行模式: sensor(传感器), simulator(TSL模拟器), multi(多设备管理器), process(多进程管理器), simple(简化多设备)")
	productType := flag.String("product", "", "产品类型（TSL模拟器模式必需）")
	tslFile := flag.String("tsl", "", "TSL文件路径（可选）")
	ruleFile := flag.String("rule", "", "规则文件路径（可选）")
	configFile := flag.String("config", "config.json", "配置文件路径")
	multiConfigFile := flag.String("multi-config", "configs/devices.json", "多设备配置文件路径")
	templatePath := flag.String("template-path", "configs/device_templates", "设备模板路径")
	devicePath := flag.String("device-path", "configs", "设备配置目录路径（简化模式）")
	webEnabled := flag.Bool("web", true, "是否启用Web管理界面")
	flag.Parse()

	// 加载应用配置
	appCfg, err := appConfig.LoadConfigFromFile(*configFile)
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// 使用框架配置创建框架
	framework := core.New(appCfg)

	// 初始化框架
	if err := framework.Initialize(appCfg); err != nil {
		log.Fatal("Failed to initialize framework:", err)
	}

	// 创建插件配置
	pluginCfg := &config.Config{
		Device: config.DeviceConfig{
			ProductKey:   appCfg.Device.ProductKey,
			DeviceName:   appCfg.Device.DeviceName,
			DeviceSecret: appCfg.Device.DeviceSecret,
		},
		MQTT: config.MQTTConfig{
			Host:         appCfg.MQTT.Host,
			Port:         appCfg.MQTT.Port,
			UseTLS:       appCfg.MQTT.UseTLS,
			KeepAlive:    time.Duration(appCfg.MQTT.KeepAlive) * time.Second,
			CleanSession: appCfg.MQTT.CleanSession,
		},
	}

	// 加载插件
	if err := framework.LoadPlugin(mqtt.NewMQTTPlugin(pluginCfg)); err != nil {
		log.Printf("Failed to load MQTT plugin: %v", err)
	}
	
	if err := framework.LoadPlugin(ota.NewOTAPlugin()); err != nil {
		log.Printf("Failed to load OTA plugin: %v", err)
	}

	// 根据模式创建设备
	switch *mode {
	case "sensor":
		// 简单传感器模式
		if err := runSensorMode(framework, appCfg); err != nil {
			log.Fatal("Failed to run sensor mode:", err)
		}

		// 启动框架
		if err := framework.Start(); err != nil {
			log.Fatal("Failed to start framework:", err)
		}

		log.Printf("IoT Uplink Generator started successfully in %s mode", *mode)
		
		// 等待关闭信号
		framework.WaitForShutdown()

	case "simulator":
		// TSL模拟器模式
		// 如果指定了不同的配置文件，重新加载配置并重新创建framework
		if *configFile != "config.json" {
			newAppCfg, err := appConfig.LoadConfigFromFile(*configFile)
			if err != nil {
				log.Fatal("Failed to load device specific config:", err)
			}
			appCfg = newAppCfg
			log.Printf("使用设备配置文件: %s (设备: %s.%s)", *configFile, appCfg.Device.ProductKey, appCfg.Device.DeviceName)
			
			// 重新创建framework使用新的配置
			framework = core.New(appCfg)
			if err := framework.Initialize(appCfg); err != nil {
				log.Fatal("Failed to initialize framework with device config:", err)
			}
			
			// 重新加载插件
			pluginCfg := &config.Config{
				Device: config.DeviceConfig{
					ProductKey:   appCfg.Device.ProductKey,
					DeviceName:   appCfg.Device.DeviceName,
					DeviceSecret: appCfg.Device.DeviceSecret,
				},
				MQTT: config.MQTTConfig{
					Host:         appCfg.MQTT.Host,
					Port:         appCfg.MQTT.Port,
					UseTLS:       appCfg.MQTT.UseTLS,
					KeepAlive:    time.Duration(appCfg.MQTT.KeepAlive) * time.Second,
					CleanSession: appCfg.MQTT.CleanSession,
				},
			}
			
			if err := framework.LoadPlugin(mqtt.NewMQTTPlugin(pluginCfg)); err != nil {
				log.Printf("Failed to load MQTT plugin: %v", err)
			}
			
			if err := framework.LoadPlugin(ota.NewOTAPlugin()); err != nil {
				log.Printf("Failed to load OTA plugin: %v", err)
			}
		}
		
		if err := runSimulatorMode(framework, appCfg, *productType, *tslFile, *ruleFile); err != nil {
			log.Fatal("Failed to run simulator mode:", err)
		}

		// 启动框架
		if err := framework.Start(); err != nil {
			log.Fatal("Failed to start framework:", err)
		}

		log.Printf("IoT Uplink Generator started successfully in %s mode", *mode)
		
		// 等待关闭信号
		framework.WaitForShutdown()

	case "multi":
		// 多设备管理器模式
		if err := runMultiDeviceMode(*multiConfigFile, *templatePath, *webEnabled); err != nil {
			log.Fatal("Failed to run multi-device mode:", err)
		}

	case "process":
		// 多进程管理器模式
		if err := runProcessMode(*multiConfigFile, *templatePath, *webEnabled); err != nil {
			log.Fatal("Failed to run process mode:", err)
		}

	case "simple":
		// 简化多设备模式
		if err := runSimpleMode(*devicePath, *webEnabled); err != nil {
			log.Fatal("Failed to run simple mode:", err)
		}

	default:
		log.Fatal("Unknown mode:", *mode)
	}
}

// runSensorMode 运行简单传感器模式
func runSensorMode(framework core.Framework, appCfg core.Config) error {
	// 创建并注册简单传感器设备
	sensorDevice := device.NewSensorDevice(
		appCfg.Device.ProductKey,
		appCfg.Device.DeviceName,
		appCfg.Device.DeviceSecret,
	)
	
	// 设置框架引用
	sensorDevice.SetFramework(framework)
	
	// 注册设备
	if err := framework.RegisterDevice(sensorDevice); err != nil {
		return err
	}

	log.Println("Sensor device registered successfully")
	return nil
}

// runSimulatorMode 运行TSL模拟器模式
func runSimulatorMode(framework core.Framework, appCfg core.Config, productType, tslFile, ruleFile string) error {
	// 获取当前工作目录
	workDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// 创建设备工厂
	factory := simulator.NewDeviceFactory(workDir)

	var simulatedDevice *simulator.SimulatedDevice

	if tslFile != "" && ruleFile != "" {
		// 从指定文件创建设备
		simulatedDevice, err = factory.CreateDeviceFromFiles(
			appCfg.Device.ProductKey,
			appCfg.Device.DeviceName,
			appCfg.Device.DeviceSecret,
			tslFile,
			ruleFile,
		)
	} else if productType != "" {
		// 从产品类型创建设备
		simulatedDevice, err = factory.CreateDevice(
			appCfg.Device.ProductKey,
			appCfg.Device.DeviceName,
			appCfg.Device.DeviceSecret,
			productType,
		)
	} else {
		// 列出可用的产品类型
		products, listErr := factory.ListAvailableProducts()
		if listErr != nil {
			return listErr
		}

		if len(products) == 0 {
			log.Println("没有找到可用的产品配置")
			log.Println("请确保在configs目录下有对应的tsl_*.json和rule_*.json文件")
			return nil
		}

		log.Println("可用的产品类型:")
		for _, product := range products {
			log.Printf("  - %s", product)
		}
		log.Println("请使用 -product 参数指定产品类型")
		return nil
	}

	if err != nil {
		return err
	}

	// 设置框架引用
	simulatedDevice.SetFramework(framework)

	// 设置上报间隔（可以从配置中读取）
	simulatedDevice.SetUploadInterval(30 * time.Second)

	// 注册设备
	if err := framework.RegisterDevice(simulatedDevice); err != nil {
		return err
	}

	log.Printf("Simulated device [%s] registered successfully", simulatedDevice.GetProductName())
	return nil
}

// runMultiDeviceMode 运行多设备管理器模式
func runMultiDeviceMode(configFile, templatePath string, webEnabled bool) error {
	log.Println("启动多设备管理器模式...")

	// 创建设备管理器
	deviceManager := manager.NewDeviceManager(configFile, templatePath)

	// 启动设备管理器
	if err := deviceManager.Start(); err != nil {
		return fmt.Errorf("启动设备管理器失败: %v", err)
	}

	log.Println("多设备管理器启动成功")

	// 启动Web管理界面（如果启用）
	if webEnabled {
		go func() {
			if err := startWebServer(deviceManager); err != nil {
				log.Printf("Web服务器启动失败: %v", err)
			}
		}()
	}

	// 等待关闭信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 监听事件（可选）
	go func() {
		eventCh := deviceManager.GetEventChannel()
		for event := range eventCh {
			log.Printf("设备事件: [%s] %s - %s", event.DeviceID, event.Type, event.Message)
		}
	}()

	// 等待停止信号
	<-sigCh
	log.Println("接收到停止信号，正在关闭...")

	// 停止设备管理器
	if err := deviceManager.Stop(); err != nil {
		log.Printf("停止设备管理器失败: %v", err)
	}

	log.Println("多设备管理器已停止")
	return nil
}

// runProcessMode 运行多进程管理器模式
func runProcessMode(configFile, templatePath string, webEnabled bool) error {
	log.Println("启动多进程管理器模式...")

	// 获取当前可执行文件路径
	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %v", err)
	}

	// 获取工作目录
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取工作目录失败: %v", err)
	}

	// 创建进程管理器
	processManager := process.NewProcessManager(executablePath, workDir)

	// 加载配置
	if err := processManager.LoadConfig(configFile, templatePath); err != nil {
		return fmt.Errorf("加载配置失败: %v", err)
	}

	// 启动进程管理器
	if err := processManager.Start(); err != nil {
		return fmt.Errorf("启动进程管理器失败: %v", err)
	}

	log.Println("多进程管理器启动成功")

	// 启动Web管理界面（如果启用）
	if webEnabled {
		go func() {
			if err := startWebServerForProcess(processManager); err != nil {
				log.Printf("Web服务器启动失败: %v", err)
			}
		}()
	}

	// 等待关闭信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 监听事件
	go func() {
		eventCh := processManager.GetEventChannel()
		for event := range eventCh {
			log.Printf("进程事件: [%s] %s - %s (PID: %d)", 
				event.DeviceID, event.Type, event.Message, event.ProcessID)
		}
	}()

	// 等待停止信号
	<-sigCh
	log.Println("接收到停止信号，正在关闭...")

	// 停止进程管理器
	if err := processManager.Stop(); err != nil {
		log.Printf("停止进程管理器失败: %v", err)
	}

	log.Println("多进程管理器已停止")
	return nil
}

// startWebServer 启动Web服务器（占位函数）
func startWebServer(_ *manager.DeviceManager) error {
	// TODO: 实现Web管理界面
	log.Println("Web管理界面将在后续版本中实现")
	return nil
}

// startWebServerForProcess 启动进程管理器Web服务器（占位函数）
func startWebServerForProcess(_ *process.ProcessManager) error {
	// TODO: 实现进程管理器Web管理界面
	log.Println("进程管理器Web管理界面将在后续版本中实现")
	return nil
}

// runSimpleMode 运行简化多设备模式
func runSimpleMode(devicePath string, webEnabled bool) error {
	log.Println("启动简化多设备模式...")

	// 获取当前可执行文件路径
	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %v", err)
	}

	// 获取工作目录
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取工作目录失败: %v", err)
	}

	// 扫描设备配置目录
	deviceDirs, err := scanDeviceDirectories(devicePath)
	if err != nil {
		return fmt.Errorf("扫描设备目录失败: %v", err)
	}

	if len(deviceDirs) == 0 {
		log.Printf("在 %s 目录下未找到任何以 'device' 开头的配置目录", devicePath)
		return nil
	}

	log.Printf("发现 %d 个设备配置目录: %v", len(deviceDirs), deviceDirs)

	// 启动设备进程
	processes := make(map[string]*exec.Cmd)
	
	for i, deviceDir := range deviceDirs {
		log.Printf("正在启动第 %d/%d 个设备进程: %s", i+1, len(deviceDirs), deviceDir)
		
		// 构建绝对路径
		var deviceDirPath string
		if filepath.IsAbs(devicePath) {
			deviceDirPath = filepath.Join(devicePath, deviceDir)
		} else {
			deviceDirPath = filepath.Join(workDir, devicePath, deviceDir)
		}
		
		configFile := filepath.Join(deviceDirPath, "config.json")
		tslFile := filepath.Join(deviceDirPath, "tsl.json")
		ruleFile := filepath.Join(deviceDirPath, "rule.json")
		
		log.Printf("设备[%s] - 配置路径: %s", deviceDir, configFile)
		log.Printf("设备[%s] - TSL路径: %s", deviceDir, tslFile)
		log.Printf("设备[%s] - 规则路径: %s", deviceDir, ruleFile)
		
		// 创建进程
		cmd := exec.Command(executablePath,
			"-mode", "simulator",
			"-product", "auto-detect",
			"-config", configFile,
			"-tsl", tslFile,
			"-rule", ruleFile,
		)
		
		// 设置工作目录
		cmd.Dir = workDir
		
		// 设置输出重定向，添加设备前缀
		cmd.Stdout = &PrefixWriter{prefix: fmt.Sprintf("[%s] ", deviceDir), writer: os.Stdout}
		cmd.Stderr = &PrefixWriter{prefix: fmt.Sprintf("[%s][ERROR] ", deviceDir), writer: os.Stderr}
		
		// 启动进程
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("启动设备进程 %s 失败: %v", deviceDir, err)
		}
		
		processes[deviceDir] = cmd
		log.Printf("设备进程[%s]启动成功，PID: %d", deviceDir, cmd.Process.Pid)
		
		// 启动goroutine监控进程状态
		go func(deviceDir string, cmd *exec.Cmd) {
			if err := cmd.Wait(); err != nil {
				log.Printf("设备进程[%s]异常退出: %v", deviceDir, err)
			} else {
				log.Printf("设备进程[%s]正常退出", deviceDir)
			}
		}(deviceDir, cmd)
	}

	log.Printf("所有设备进程启动成功: %d 个进程", len(processes))

	// 启动Web管理界面（如果启用）
	var webManager *web.WebManager
	if webEnabled {
		log.Println("启动Web管理界面...")
		webConfig := &web.Config{
			Port:      8080,
			Host:      "0.0.0.0",
			Debug:     false,
			StaticDir: "web/static",
		}
		
		webManager = web.NewWebManager(webConfig)
		
		// 在单独的goroutine中启动Web服务器
		go func() {
			if err := webManager.Start(); err != nil {
				log.Printf("Web服务器启动失败: %v", err)
			}
		}()
		
		log.Println("Web管理界面已启动，访问地址: http://0.0.0.0:8080")
	}

	// 等待关闭信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 等待停止信号
	<-sigCh
	log.Println("接收到停止信号，正在关闭所有设备进程...")

	// 停止Web服务器
	if webManager != nil {
		log.Println("停止Web管理界面...")
		if err := webManager.Stop(); err != nil {
			log.Printf("停止Web服务器失败: %v", err)
		}
	}

	// 停止所有进程
	for deviceDir, cmd := range processes {
		if cmd.Process != nil {
			log.Printf("停止设备进程[%s]，PID: %d", deviceDir, cmd.Process.Pid)
			if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
				log.Printf("发送SIGTERM信号失败: %v", err)
			}
		}
	}

	log.Println("简化多设备模式已停止")
	return nil
}

// scanDeviceDirectories 扫描以device开头的配置目录
func scanDeviceDirectories(devicePath string) ([]string, error) {
	entries, err := os.ReadDir(devicePath)
	if err != nil {
		return nil, err
	}

	var deviceDirs []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "device") {
			// 检查必需的文件是否存在
			configFile := fmt.Sprintf("%s/%s/config.json", devicePath, entry.Name())
			tslFile := fmt.Sprintf("%s/%s/tsl.json", devicePath, entry.Name())
			ruleFile := fmt.Sprintf("%s/%s/rule.json", devicePath, entry.Name())
			
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				log.Printf("跳过设备目录 %s: 缺少 config.json", entry.Name())
				continue
			}
			if _, err := os.Stat(tslFile); os.IsNotExist(err) {
				log.Printf("跳过设备目录 %s: 缺少 tsl.json", entry.Name())
				continue
			}
			if _, err := os.Stat(ruleFile); os.IsNotExist(err) {
				log.Printf("跳过设备目录 %s: 缺少 rule.json", entry.Name())
				continue
			}
			
			deviceDirs = append(deviceDirs, entry.Name())
		}
	}
	
	return deviceDirs, nil
}

// PrefixWriter 为输出添加前缀的Writer
type PrefixWriter struct {
	prefix string
	writer io.Writer
}

func (pw *PrefixWriter) Write(p []byte) (n int, err error) {
	lines := strings.Split(string(p), "\n")
	for i, line := range lines {
		if line != "" || i < len(lines)-1 {
			if line != "" {
				_, err = pw.writer.Write([]byte(pw.prefix + line + "\n"))
			} else if i < len(lines)-1 {
				_, err = pw.writer.Write([]byte("\n"))
			}
			if err != nil {
				return 0, err
			}
		}
	}
	return len(p), nil
}