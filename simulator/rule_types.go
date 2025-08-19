package simulator

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// SimulationRule 定义模拟规则结构
type SimulationRule struct {
	ProductName      string                       `json:"productName"`
	SimulationConfig map[string]PropertySimConfig `json:"simulationConfig"`
	Events           []EventSimConfig             `json:"events"`
	Services         map[string]ServiceSimConfig  `json:"services"`
}

// PropertySimConfig 定义属性模拟配置
type PropertySimConfig struct {
	Method            string      `json:"method"`
	Min               json.Number `json:"min,omitempty"`
	Max               json.Number `json:"max,omitempty"`
	Step              json.Number `json:"step,omitempty"`
	Start             json.Number `json:"start,omitempty"`
	Value             json.Number `json:"value,omitempty"`
	EnumValues        []string    `json:"enumValues,omitempty"`
	SwitchProbability float64     `json:"switchProbability,omitempty"`
	Amplitude         json.Number `json:"amplitude,omitempty"`
	WavePeriod        int         `json:"wavePeriod,omitempty"`
}

// EventSimConfig 定义事件模拟配置
type EventSimConfig struct {
	Identifier       string `json:"identifier"`
	TriggerCondition string `json:"triggerCondition"`
	Cooldown         int    `json:"cooldown"`
}

// ServiceSimConfig 定义服务模拟配置
type ServiceSimConfig struct {
	ResponseStrategy  string            `json:"responseStrategy"`
	PossibleResponses []ServiceResponse `json:"possibleResponses"`
}

// ServiceResponse 定义服务响应
type ServiceResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Desc string `json:"desc"`
}

// RuleManager 规则管理器
type RuleManager struct {
	baseDir string
}

// NewRuleManager 创建规则管理器
func NewRuleManager(baseDir string) *RuleManager {
	return &RuleManager{
		baseDir: baseDir,
	}
}

// LoadRule 从文件加载规则
func (m *RuleManager) LoadRule(filename string) (*SimulationRule, error) {
	var filePath string
	if filepath.IsAbs(filename) {
		// 如果是绝对路径，直接使用
		filePath = filename
	} else {
		// 如果是相对路径，添加baseDir和configs前缀
		filePath = filepath.Join(m.baseDir, "configs", filename)
	}
	
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取规则文件失败: %v", err)
	}

	var rule SimulationRule
	if err := json.Unmarshal(data, &rule); err != nil {
		return nil, fmt.Errorf("解析规则失败: %v", err)
	}

	return &rule, nil
}

// SaveRule 保存规则到文件
func (m *RuleManager) SaveRule(filename string, rule *SimulationRule) error {
	filePath := filepath.Join(m.baseDir, "configs", filename)
	
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	data, err := json.MarshalIndent(rule, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化规则失败: %v", err)
	}

	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("保存规则文件失败: %v", err)
	}

	return nil
}

// ListRuleFiles 列出所有规则文件
func (m *RuleManager) ListRuleFiles() ([]string, error) {
	configsDir := filepath.Join(m.baseDir, "configs")
	files, err := os.ReadDir(configsDir)
	if err != nil {
		return nil, err
	}

	var ruleFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "rule_") && strings.HasSuffix(file.Name(), ".json") {
			ruleFiles = append(ruleFiles, file.Name())
		}
	}

	return ruleFiles, nil
}

// ValidateRule 验证规则
func (m *RuleManager) ValidateRule(rule *SimulationRule) error {
	if rule == nil {
		return fmt.Errorf("规则不能为空")
	}

	if rule.ProductName == "" {
		return fmt.Errorf("产品名称不能为空")
	}

	// 验证属性配置
	for identifier, config := range rule.SimulationConfig {
		if identifier == "" {
			return fmt.Errorf("属性标识符不能为空")
		}
		
		if err := m.validatePropertyConfig(config); err != nil {
			return fmt.Errorf("属性[%s]配置无效: %v", identifier, err)
		}
	}

	// 验证事件配置
	for _, event := range rule.Events {
		if event.Identifier == "" {
			return fmt.Errorf("事件标识符不能为空")
		}
		if event.Cooldown < 0 {
			return fmt.Errorf("事件[%s]冷却时间不能为负数", event.Identifier)
		}
	}

	// 验证服务配置
	for identifier, service := range rule.Services {
		if identifier == "" {
			return fmt.Errorf("服务标识符不能为空")
		}
		
		if err := m.validateServiceConfig(service); err != nil {
			return fmt.Errorf("服务[%s]配置无效: %v", identifier, err)
		}
	}

	return nil
}

// validatePropertyConfig 验证属性配置
func (m *RuleManager) validatePropertyConfig(config PropertySimConfig) error {
	validMethods := []string{"randomRange", "wave", "accumulate", "increase", "enum", "enumPick", "fixed"}
	
	valid := false
	for _, method := range validMethods {
		if config.Method == method {
			valid = true
			break
		}
	}
	
	if !valid {
		return fmt.Errorf("不支持的模拟方法: %s", config.Method)
	}

	// 验证不同方法的参数
	switch config.Method {
	case "randomRange":
		if config.Min == "" || config.Max == "" {
			return fmt.Errorf("randomRange方法需要min和max参数")
		}
		minVal, _ := config.Min.Float64()
		maxVal, _ := config.Max.Float64()
		if minVal >= maxVal {
			return fmt.Errorf("min值不能大于等于max值")
		}
		
	case "wave":
		if config.Min == "" || config.Max == "" || config.Amplitude == "" {
			return fmt.Errorf("wave方法需要min、max和amplitude参数")
		}
		if config.WavePeriod <= 0 {
			return fmt.Errorf("波形周期必须大于0")
		}
		
	case "accumulate", "increase":
		if config.Step == "" {
			return fmt.Errorf("%s方法需要step参数", config.Method)
		}
		
	case "enum", "enumPick":
		if len(config.EnumValues) == 0 {
			return fmt.Errorf("%s方法需要enumValues参数", config.Method)
		}
		if config.SwitchProbability < 0 || config.SwitchProbability > 1 {
			return fmt.Errorf("切换概率必须在0-1之间")
		}
		
	case "fixed":
		if config.Value == "" {
			return fmt.Errorf("fixed方法需要value参数")
		}
	}

	return nil
}

// validateServiceConfig 验证服务配置
func (m *RuleManager) validateServiceConfig(config ServiceSimConfig) error {
	validStrategies := []string{"fixed", "random", "randomPick"}
	
	valid := false
	for _, strategy := range validStrategies {
		if config.ResponseStrategy == strategy {
			valid = true
			break
		}
	}
	
	if !valid {
		return fmt.Errorf("不支持的响应策略: %s", config.ResponseStrategy)
	}

	if len(config.PossibleResponses) == 0 {
		return fmt.Errorf("至少需要一个可能的响应")
	}

	for i, response := range config.PossibleResponses {
		if response.Code < 100 || response.Code > 599 {
			return fmt.Errorf("响应[%d]的状态码无效: %d", i, response.Code)
		}
	}

	return nil
}

// GetProductNameFromRuleFile 从规则文件名提取产品名称
func GetProductNameFromRuleFile(filename string) string {
	base := filepath.Base(filename)
	if strings.HasPrefix(base, "rule_") {
		productName := strings.TrimPrefix(base, "rule_")
		productName = strings.TrimSuffix(productName, ".json")
		return productName
	}
	return ""
}

// GenerateRuleFileName 生成规则文件名
func GenerateRuleFileName(productName string) string {
	return fmt.Sprintf("rule_%s.json", productName)
}