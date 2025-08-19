package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/iot-go-sdk/pkg/config"
	"github.com/iot-go-sdk/pkg/framework/core"
	"github.com/iot-go-sdk/pkg/framework/plugins/mqtt"
	"github.com/iot-go-sdk/pkg/framework/plugins/ota"
	appConfig "znb/iot-uplink-gen/config"
	"znb/iot-uplink-gen/device"
	"znb/iot-uplink-gen/simulator"
)

func main() {
	// 命令行参数
	mode := flag.String("mode", "sensor", "运行模式: sensor(传感器) 或 simulator(TSL模拟器)")
	productType := flag.String("product", "", "产品类型（TSL模拟器模式必需）")
	tslFile := flag.String("tsl", "", "TSL文件路径（可选）")
	ruleFile := flag.String("rule", "", "规则文件路径（可选）")
	configFile := flag.String("config", "config.json", "配置文件路径")
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

	case "simulator":
		// TSL模拟器模式
		if err := runSimulatorMode(framework, appCfg, *productType, *tslFile, *ruleFile); err != nil {
			log.Fatal("Failed to run simulator mode:", err)
		}

	default:
		log.Fatal("Unknown mode:", *mode)
	}

	// 启动框架
	if err := framework.Start(); err != nil {
		log.Fatal("Failed to start framework:", err)
	}

	log.Printf("IoT Uplink Generator started successfully in %s mode", *mode)
	
	// 等待关闭信号
	framework.WaitForShutdown()
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