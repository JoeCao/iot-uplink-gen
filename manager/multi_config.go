package manager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/iot-go-sdk/pkg/framework/core"
)

// MultiDeviceConfig 多设备配置
type MultiDeviceConfig struct {
	DeviceGroups []DeviceGroup `json:"device_groups"`
	GlobalConfig GlobalConfig  `json:"global_config"`
}

// DeviceGroup 设备组配置
type DeviceGroup struct {
	GroupName   string       `json:"group_name"`
	ProductType string       `json:"product_type"`
	Devices     []DeviceInfo `json:"devices"`
	Template    string       `json:"template"`       // 模板目录名
	Enabled     bool         `json:"enabled"`        // 是否启用
	MaxInstances int         `json:"max_instances"`  // 最大实例数
}

// DeviceInfo 设备信息
type DeviceInfo struct {
	DeviceID     string                 `json:"device_id"`     // 设备唯一标识
	DeviceName   string                 `json:"device_name"`   // 设备名称
	ProductKey   string                 `json:"product_key"`   // 产品密钥
	DeviceSecret string                 `json:"device_secret"` // 设备密钥
	Enabled      bool                   `json:"enabled"`       // 是否启用
	CustomConfig map[string]interface{} `json:"custom_config"` // 自定义配置
	Interval     int                    `json:"interval"`      // 上报间隔(秒)
	Tags         []string               `json:"tags"`          // 设备标签
}

// GlobalConfig 全局配置
type GlobalConfig struct {
	MQTT        MQTTGlobalConfig `json:"mqtt"`
	Web         WebConfig        `json:"web"`
	Logging     LoggingConfig    `json:"logging"`
	DefaultInterval int          `json:"default_interval"` // 默认上报间隔
}

// MQTTGlobalConfig MQTT全局配置
type MQTTGlobalConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	UseTLS       bool   `json:"use_tls"`
	KeepAlive    int    `json:"keep_alive"`
	CleanSession bool   `json:"clean_session"`
	AutoReconnect bool  `json:"auto_reconnect"`
	Region       string `json:"region"`
}

// WebConfig Web管理配置
type WebConfig struct {
	Enabled bool   `json:"enabled"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	APIPath string `json:"api_path"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level      string `json:"level"`
	OutputPath string `json:"output_path"`
	MaxSize    int    `json:"max_size"`    // MB
	MaxBackups int    `json:"max_backups"`
	MaxAge     int    `json:"max_age"`     // 天
}

// DeviceTemplate 设备模板
type DeviceTemplate struct {
	Name         string `json:"name"`
	ProductType  string `json:"product_type"`
	Description  string `json:"description"`
	TSLFile      string `json:"tsl_file"`
	RuleFile     string `json:"rule_file"`
	ConfigFile   string `json:"config_file"`
	TemplatePath string `json:"template_path"`
}

// LoadMultiDeviceConfig 加载多设备配置
func LoadMultiDeviceConfig(configPath string) (*MultiDeviceConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config MultiDeviceConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 验证配置
	if err := validateMultiDeviceConfig(&config); err != nil {
		return nil, fmt.Errorf("配置验证失败: %v", err)
	}

	return &config, nil
}

// SaveMultiDeviceConfig 保存多设备配置
func SaveMultiDeviceConfig(config *MultiDeviceConfig, configPath string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("保存配置文件失败: %v", err)
	}

	return nil
}

// GenerateDeviceConfig 根据设备信息和模板生成framework配置
func (di *DeviceInfo) GenerateDeviceConfig(template *DeviceTemplate, globalConfig *GlobalConfig) (*core.Config, error) {
	config := &core.Config{
		Device: core.DeviceConfig{
			ProductKey:   di.ProductKey,
			DeviceName:   di.DeviceName,
			DeviceSecret: di.DeviceSecret,
			Region:       globalConfig.MQTT.Region,
		},
		MQTT: core.MQTTConfig{
			Host:          globalConfig.MQTT.Host,
			Port:          globalConfig.MQTT.Port,
			UseTLS:        globalConfig.MQTT.UseTLS,
			KeepAlive:     globalConfig.MQTT.KeepAlive,
			CleanSession:  globalConfig.MQTT.CleanSession,
			AutoReconnect: globalConfig.MQTT.AutoReconnect,
		},
		Features: core.FeatureConfig{
			EnableOTA:     true,
			EnableShadow:  false,
			EnableRules:   false,
			EnableMetrics: true,
		},
		Logging: core.LoggingConfig{
			Level: globalConfig.Logging.Level,
		},
	}

	// 应用自定义配置
	if di.CustomConfig != nil {
		if mqttConfig, ok := di.CustomConfig["mqtt"].(map[string]interface{}); ok {
			if host, ok := mqttConfig["host"].(string); ok && host != "" {
				config.MQTT.Host = host
			}
			if port, ok := mqttConfig["port"].(float64); ok && port > 0 {
				config.MQTT.Port = int(port)
			}
		}
	}

	return config, nil
}

// GetUploadInterval 获取上报间隔
func (di *DeviceInfo) GetUploadInterval(defaultInterval int) int {
	if di.Interval > 0 {
		return di.Interval
	}
	if defaultInterval > 0 {
		return defaultInterval
	}
	return 30 // 默认30秒
}

// LoadDeviceTemplate 加载设备模板
func LoadDeviceTemplate(templatePath string) (*DeviceTemplate, error) {
	configFile := filepath.Join(templatePath, "template.json")
	
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("读取模板配置失败: %v", err)
	}

	var template DeviceTemplate
	if err := json.Unmarshal(data, &template); err != nil {
		return nil, fmt.Errorf("解析模板配置失败: %v", err)
	}

	// 设置模板路径
	template.TemplatePath = templatePath
	
	// 补全文件路径
	if template.TSLFile != "" && !filepath.IsAbs(template.TSLFile) {
		template.TSLFile = filepath.Join(templatePath, template.TSLFile)
	}
	if template.RuleFile != "" && !filepath.IsAbs(template.RuleFile) {
		template.RuleFile = filepath.Join(templatePath, template.RuleFile)
	}
	if template.ConfigFile != "" && !filepath.IsAbs(template.ConfigFile) {
		template.ConfigFile = filepath.Join(templatePath, template.ConfigFile)
	}

	return &template, nil
}

// validateMultiDeviceConfig 验证多设备配置
func validateMultiDeviceConfig(config *MultiDeviceConfig) error {
	if len(config.DeviceGroups) == 0 {
		return fmt.Errorf("至少需要配置一个设备组")
	}

	// 检查设备ID唯一性
	deviceIDs := make(map[string]bool)
	for _, group := range config.DeviceGroups {
		if group.GroupName == "" {
			return fmt.Errorf("设备组名称不能为空")
		}

		for _, device := range group.Devices {
			if device.DeviceID == "" {
				return fmt.Errorf("设备ID不能为空")
			}
			if deviceIDs[device.DeviceID] {
				return fmt.Errorf("设备ID[%s]重复", device.DeviceID)
			}
			deviceIDs[device.DeviceID] = true

			if device.ProductKey == "" || device.DeviceName == "" || device.DeviceSecret == "" {
				return fmt.Errorf("设备[%s]的认证信息不完整", device.DeviceID)
			}
		}
	}

	// 验证全局配置
	if config.GlobalConfig.MQTT.Host == "" {
		return fmt.Errorf("MQTT服务器地址不能为空")
	}
	if config.GlobalConfig.MQTT.Port <= 0 {
		return fmt.Errorf("MQTT端口必须大于0")
	}

	return nil
}

// GetEnabledDevices 获取所有启用的设备
func (config *MultiDeviceConfig) GetEnabledDevices() []DeviceInfo {
	var devices []DeviceInfo
	
	for _, group := range config.DeviceGroups {
		if !group.Enabled {
			continue
		}

		count := 0
		for _, device := range group.Devices {
			if !device.Enabled {
				continue
			}
			
			// 检查最大实例数限制
			if group.MaxInstances > 0 && count >= group.MaxInstances {
				break
			}
			
			devices = append(devices, device)
			count++
		}
	}
	
	return devices
}

// GetDeviceByID 根据ID获取设备
func (config *MultiDeviceConfig) GetDeviceByID(deviceID string) (*DeviceInfo, *DeviceGroup, error) {
	for _, group := range config.DeviceGroups {
		for i, device := range group.Devices {
			if device.DeviceID == deviceID {
				return &group.Devices[i], &group, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("设备[%s]不存在", deviceID)
}

// CreateDefaultMultiConfig 创建默认多设备配置
func CreateDefaultMultiConfig() *MultiDeviceConfig {
	return &MultiDeviceConfig{
		DeviceGroups: []DeviceGroup{
			{
				GroupName:   "智能电机组",
				ProductType: "电机",
				Template:    "motor",
				Enabled:     true,
				MaxInstances: 10,
				Devices: []DeviceInfo{
					{
						DeviceID:     "motor_001",
						DeviceName:   "电机设备001",
						ProductKey:   "FuWtDWoy",
						DeviceSecret: "sbGnxntUSnDsKTc7",
						Enabled:      true,
						Interval:     30,
						Tags:         []string{"motor", "production"},
					},
				},
			},
		},
		GlobalConfig: GlobalConfig{
			MQTT: MQTTGlobalConfig{
				Host:          "121.40.253.229",
				Port:          1883,
				UseTLS:        false,
				KeepAlive:     60,
				CleanSession:  true,
				AutoReconnect: true,
				Region:        "cn-shanghai",
			},
			Web: WebConfig{
				Enabled: true,
				Host:    "0.0.0.0",
				Port:    8080,
				APIPath: "/api",
			},
			Logging: LoggingConfig{
				Level:      "info",
				OutputPath: "logs/",
				MaxSize:    100,
				MaxBackups: 5,
				MaxAge:     30,
			},
			DefaultInterval: 30,
		},
	}
}