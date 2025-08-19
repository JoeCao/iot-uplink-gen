package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"time"

	"github.com/iot-go-sdk/pkg/framework/core"
)

// LoadConfig loads the framework configuration
func LoadConfig() (core.Config, error) {
	return LoadConfigFromFile("config.json")
}

// LoadConfigFromFile loads configuration from specified file
func LoadConfigFromFile(filename string) (core.Config, error) {
	// 默认配置
	config := core.Config{
		Device: core.DeviceConfig{
			ProductKey:   "MySensorProduct",
			DeviceName:   "SensorDevice001",
			DeviceSecret: "device_secret_here",
			Region:       "cn-shanghai",
		},
		MQTT: core.MQTTConfig{
			Host:          "localhost",
			Port:          1883,
			UseTLS:        false,
			KeepAlive:     60,
			CleanSession:  true,
			AutoReconnect: true,
			ReconnectMax:  10,
			Timeout:       30 * time.Second,
		},
		Features: core.FeatureConfig{
			EnableOTA:    true,
			EnableShadow: false,
			EnableRules:  false,
			EnableMetrics: true,
		},
		Logging: core.LoggingConfig{
			Level:      "info",
			Format:     "text",
			Output:     "stdout",
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     30,
		},
		Advanced: core.AdvancedConfig{
			WorkerCount:      10,
			EventBufferSize:  1000,
			RequestTimeout:   30 * time.Second,
			PropertyCacheTTL: 5 * time.Minute,
		},
	}

	// 尝试从文件加载配置覆盖
	if data, err := ioutil.ReadFile(filename); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return config, err
		}
	}

	// 从环境变量覆盖配置
	if broker := os.Getenv("MQTT_HOST"); broker != "" {
		config.MQTT.Host = broker
	}
	if productKey := os.Getenv("PRODUCT_KEY"); productKey != "" {
		config.Device.ProductKey = productKey
	}
	if deviceName := os.Getenv("DEVICE_NAME"); deviceName != "" {
		config.Device.DeviceName = deviceName
	}
	if deviceSecret := os.Getenv("DEVICE_SECRET"); deviceSecret != "" {
		config.Device.DeviceSecret = deviceSecret
	}

	return config, nil
}