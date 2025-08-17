package simulator

import (
	"encoding/json"
	"fmt"

	"znb/iot-uplink-gen/tsl"
)

// DeviceFactory 设备工厂
type DeviceFactory struct {
	tslManager  *tsl.TSLManager
	ruleManager *RuleManager
	baseDir     string
}

// NewDeviceFactory 创建设备工厂
func NewDeviceFactory(baseDir string) *DeviceFactory {
	return &DeviceFactory{
		tslManager:  tsl.NewTSLManager(baseDir),
		ruleManager: NewRuleManager(baseDir),
		baseDir:     baseDir,
	}
}

// CreateDevice 创建模拟设备
func (df *DeviceFactory) CreateDevice(productKey, deviceName, deviceSecret, productType string) (*SimulatedDevice, error) {
	// 加载TSL文件
	tslFile := tsl.GenerateTSLFileName(productType)
	tslModel, err := df.tslManager.LoadTSL(tslFile)
	if err != nil {
		return nil, fmt.Errorf("加载TSL文件失败: %v", err)
	}

	// 验证TSL
	if err := df.tslManager.ValidateTSL(tslModel); err != nil {
		return nil, fmt.Errorf("TSL验证失败: %v", err)
	}

	// 加载规则文件
	ruleFile := GenerateRuleFileName(productType)
	rule, err := df.ruleManager.LoadRule(ruleFile)
	if err != nil {
		return nil, fmt.Errorf("加载规则文件失败: %v", err)
	}

	// 验证规则
	if err := df.ruleManager.ValidateRule(rule); err != nil {
		return nil, fmt.Errorf("规则验证失败: %v", err)
	}

	// 验证TSL和规则的一致性
	if err := df.validateTSLRuleConsistency(tslModel, rule); err != nil {
		return nil, fmt.Errorf("TSL和规则不一致: %v", err)
	}

	// 创建模拟设备
	device := NewSimulatedDevice(productKey, deviceName, deviceSecret, tslModel, rule)

	return device, nil
}

// CreateDeviceFromFiles 从指定的TSL和规则文件创建设备
func (df *DeviceFactory) CreateDeviceFromFiles(productKey, deviceName, deviceSecret, tslFile, ruleFile string) (*SimulatedDevice, error) {
	// 加载TSL文件
	tslModel, err := df.tslManager.LoadTSL(tslFile)
	if err != nil {
		return nil, fmt.Errorf("加载TSL文件失败: %v", err)
	}

	// 验证TSL
	if err := df.tslManager.ValidateTSL(tslModel); err != nil {
		return nil, fmt.Errorf("TSL验证失败: %v", err)
	}

	// 加载规则文件
	rule, err := df.ruleManager.LoadRule(ruleFile)
	if err != nil {
		return nil, fmt.Errorf("加载规则文件失败: %v", err)
	}

	// 验证规则
	if err := df.ruleManager.ValidateRule(rule); err != nil {
		return nil, fmt.Errorf("规则验证失败: %v", err)
	}

	// 验证TSL和规则的一致性
	if err := df.validateTSLRuleConsistency(tslModel, rule); err != nil {
		return nil, fmt.Errorf("TSL和规则不一致: %v", err)
	}

	// 创建模拟设备
	device := NewSimulatedDevice(productKey, deviceName, deviceSecret, tslModel, rule)

	return device, nil
}

// GenerateEmptyRule 根据TSL生成空的模拟规则
func (df *DeviceFactory) GenerateEmptyRule(tslFile string) (*SimulationRule, error) {
	// 加载TSL文件
	tslModel, err := df.tslManager.LoadTSL(tslFile)
	if err != nil {
		return nil, fmt.Errorf("加载TSL文件失败: %v", err)
	}

	// 从文件名推断产品名称
	productName := tsl.GetProductNameFromTSLFile(tslFile)
	if productName == "" {
		productName = "unknown"
	}

	// 创建空规则
	rule := &SimulationRule{
		ProductName:      productName,
		SimulationConfig: make(map[string]PropertySimConfig),
		Events:           make([]EventSimConfig, 0),
		Services:         make(map[string]ServiceSimConfig),
	}

	// 为每个属性生成默认配置
	for _, prop := range tslModel.Properties {
		config := df.generateDefaultPropertyConfig(prop)
		rule.SimulationConfig[prop.Identifier] = config
	}

	// 为每个事件生成默认配置
	for _, event := range tslModel.Events {
		config := EventSimConfig{
			Identifier:       event.Identifier,
			TriggerCondition: "", // 需要用户手动配置
			Cooldown:         60, // 默认60秒冷却
		}
		rule.Events = append(rule.Events, config)
	}

	// 为每个服务生成默认配置
	for _, action := range tslModel.Actions {
		config := ServiceSimConfig{
			ResponseStrategy: "fixed",
			PossibleResponses: []ServiceResponse{
				{
					Code: 200,
					Msg:  "ok",
					Desc: fmt.Sprintf("%s执行成功", action.Name),
				},
				{
					Code: 500,
					Msg:  "fail",
					Desc: fmt.Sprintf("%s执行失败", action.Name),
				},
			},
		}
		rule.Services[action.Identifier] = config
	}

	return rule, nil
}

// generateDefaultPropertyConfig 生成默认属性配置
func (df *DeviceFactory) generateDefaultPropertyConfig(prop tsl.Property) PropertySimConfig {
	config := PropertySimConfig{
		Method: "randomRange", // 默认使用随机范围
	}

	dataType := prop.GetDataType()
	switch dataType.Type {
	case "float", "double":
		if dataType.Specs.Min != 0 || dataType.Specs.Max != 0 {
			config.Min = json.Number(fmt.Sprintf("%v", dataType.Specs.Min))
			config.Max = json.Number(fmt.Sprintf("%v", dataType.Specs.Max))
		} else {
			config.Min = json.Number("0.0")
			config.Max = json.Number("100.0")
		}
		config.Step = json.Number("0.1")

	case "int", "long":
		if dataType.Specs.Min != 0 || dataType.Specs.Max != 0 {
			config.Min = json.Number(fmt.Sprintf("%v", int64(dataType.Specs.Min)))
			config.Max = json.Number(fmt.Sprintf("%v", int64(dataType.Specs.Max)))
		} else {
			config.Min = json.Number("0")
			config.Max = json.Number("100")
		}
		config.Step = json.Number("1")

	case "bool":
		config.Method = "enum"
		config.EnumValues = []string{"true", "false"}
		config.SwitchProbability = 0.3

	case "text", "string":
		config.Method = "enum"
		config.EnumValues = []string{"value1", "value2", "value3"}
		config.SwitchProbability = 0.3

	case "enum":
		config.Method = "enum"
		if dataType.Specs.Enum != "" {
			// 解析枚举值
			// 这里需要根据实际的枚举值格式进行解析
			config.EnumValues = []string{dataType.Specs.Enum}
		} else {
			config.EnumValues = []string{"enum1", "enum2", "enum3"}
		}
		config.SwitchProbability = 0.3
	}

	return config
}

// validateTSLRuleConsistency 验证TSL和规则的一致性
func (df *DeviceFactory) validateTSLRuleConsistency(tslModel *tsl.TSLModel, rule *SimulationRule) error {
	// 检查属性一致性
	for _, prop := range tslModel.Properties {
		if _, exists := rule.SimulationConfig[prop.Identifier]; !exists {
			return fmt.Errorf("属性[%s]在TSL中定义但规则中未配置", prop.Identifier)
		}
	}

	// 检查规则中的属性是否都在TSL中定义
	tslProps := make(map[string]bool)
	for _, prop := range tslModel.Properties {
		tslProps[prop.Identifier] = true
	}

	for identifier := range rule.SimulationConfig {
		if !tslProps[identifier] {
			return fmt.Errorf("属性[%s]在规则中配置但TSL中未定义", identifier)
		}
	}

	// 检查事件一致性
	tslEvents := make(map[string]bool)
	for _, event := range tslModel.Events {
		tslEvents[event.Identifier] = true
	}

	for _, eventConfig := range rule.Events {
		if !tslEvents[eventConfig.Identifier] {
			return fmt.Errorf("事件[%s]在规则中配置但TSL中未定义", eventConfig.Identifier)
		}
	}

	// 检查服务一致性
	tslServices := make(map[string]bool)
	for _, action := range tslModel.Actions {
		tslServices[action.Identifier] = true
	}

	for identifier := range rule.Services {
		if !tslServices[identifier] {
			return fmt.Errorf("服务[%s]在规则中配置但TSL中未定义", identifier)
		}
	}

	return nil
}

// ListAvailableProducts 列出可用的产品类型
func (df *DeviceFactory) ListAvailableProducts() ([]string, error) {
	// 列出TSL文件
	tslFiles, err := df.tslManager.ListTSLFiles()
	if err != nil {
		return nil, err
	}

	// 列出规则文件
	ruleFiles, err := df.ruleManager.ListRuleFiles()
	if err != nil {
		return nil, err
	}

	// 找出同时存在TSL和规则文件的产品
	tslProducts := make(map[string]bool)
	for _, tslFile := range tslFiles {
		product := tsl.GetProductNameFromTSLFile(tslFile)
		if product != "" {
			tslProducts[product] = true
		}
	}

	ruleProducts := make(map[string]bool)
	for _, ruleFile := range ruleFiles {
		product := GetProductNameFromRuleFile(ruleFile)
		if product != "" {
			ruleProducts[product] = true
		}
	}

	// 取交集
	var availableProducts []string
	for product := range tslProducts {
		if ruleProducts[product] {
			availableProducts = append(availableProducts, product)
		}
	}

	return availableProducts, nil
}

// GetTSLManager 获取TSL管理器
func (df *DeviceFactory) GetTSLManager() *tsl.TSLManager {
	return df.tslManager
}

// GetRuleManager 获取规则管理器
func (df *DeviceFactory) GetRuleManager() *RuleManager {
	return df.ruleManager
}