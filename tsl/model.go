package tsl

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// TSLModel 定义完整的 TSL 结构
type TSLModel struct {
	Version    string     `json:"version"`
	Properties []Property `json:"properties"`
	Events     []Event    `json:"events"`
	Actions    []Action   `json:"actions"`
}

// Property 定义属性结构
type Property struct {
	Identifier string   `json:"identifier"`
	Name       string   `json:"name"`
	AccessMode string   `json:"accessMode"`
	Required   bool     `json:"required"`
	DataType   DataType `json:"dataType"`
	DataType2  DataType `json:"data_type"` // 支持下划线格式
	Desc       string   `json:"desc"`      // 添加描述字段
}

// GetDataType 获取属性的数据类型，支持两种字段格式
func (p *Property) GetDataType() DataType {
	// 如果DataType2有值，优先使用data_type字段
	if p.DataType2.Type != "" {
		return p.DataType2
	}
	// 否则使用dataType字段
	return p.DataType
}

// Event 定义事件结构
type Event struct {
	Identifier  string       `json:"identifier"`
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	Required    bool         `json:"required"`
	Desc        string       `json:"desc"`
	Method      string       `json:"method"`
	OutputData  []EventParam `json:"outputData"`
	OutputData2 []EventParam `json:"output_data"` // 支持下划线格式
	EventType   string       `json:"eventType"`
	EventType2  string       `json:"event_type"` // 支持下划线格式
}

// EventParam 定义事件参数结构
type EventParam struct {
	Identifier string   `json:"identifier"`
	Name       string   `json:"name"`
	DataType   DataType `json:"dataType"`
	DataType2  DataType `json:"data_type"` // 支持下划线格式
	Desc       string   `json:"desc"`      // 添加描述字段
}

// GetDataType 获取事件参数的数据类型，支持两种字段格式
func (ep *EventParam) GetDataType() DataType {
	if ep.DataType2.Type != "" {
		return ep.DataType2
	}
	return ep.DataType
}

// GetOutputData 获取事件的输出数据，支持两种字段格式
func (e *Event) GetOutputData() []EventParam {
	if len(e.OutputData2) > 0 {
		return e.OutputData2
	}
	return e.OutputData
}

// GetEventType 获取事件类型，支持两种字段格式
func (e *Event) GetEventType() string {
	if e.EventType2 != "" {
		return e.EventType2
	}
	return e.EventType
}

// Action 定义服务结构
type Action struct {
	Identifier  string        `json:"identifier"`
	Name        string        `json:"name"`
	Required    bool          `json:"required"`
	CallType    string        `json:"callType"`
	Desc        string        `json:"desc"`
	Method      string        `json:"method"`
	InputData   []ActionParam `json:"inputData"`
	InputData2  []ActionParam `json:"input_data"`  // 支持下划线格式
	OutputData  []ActionParam `json:"outputData"`
	OutputData2 []ActionParam `json:"output_data"` // 支持下划线格式
}

// GetInputData 获取动作的输入数据，支持两种字段格式
func (a *Action) GetInputData() []ActionParam {
	if len(a.InputData2) > 0 {
		return a.InputData2
	}
	return a.InputData
}

// GetOutputData 获取动作的输出数据，支持两种字段格式
func (a *Action) GetOutputData() []ActionParam {
	if len(a.OutputData2) > 0 {
		return a.OutputData2
	}
	return a.OutputData
}

// ActionParam 定义服务参数结构
type ActionParam struct {
	Identifier string   `json:"identifier"`
	Name       string   `json:"name"`
	DataType   DataType `json:"dataType"`
	DataType2  DataType `json:"data_type"` // 支持下划线格式
	Desc       string   `json:"desc"`      // 添加描述字段
}

// GetDataType 获取动作参数的数据类型，支持两种字段格式
func (ap *ActionParam) GetDataType() DataType {
	if ap.DataType2.Type != "" {
		return ap.DataType2
	}
	return ap.DataType
}

// DataType 定义数据类型结构
type DataType struct {
	Type  string    `json:"type"`
	Specs DataSpecs `json:"specs"`
}

// DataSpecs 定义数据规格结构
type DataSpecs struct {
	Length    int     `json:"length"`
	Unit      string  `json:"unit"`
	UnitName  string  `json:"unitName"`
	Min       float64 `json:"min"`
	Max       float64 `json:"max"`
	Step      float64 `json:"step"`
	Accuracy  int     `json:"accuracy"`
	Enum      string  `json:"enum"`
	True      string  `json:"true"`
	False     string  `json:"false"`
	EnumValue string  `json:"enumValue"`
}

// TSLManager TSL管理器
type TSLManager struct {
	baseDir string
}

// NewTSLManager 创建TSL管理器
func NewTSLManager(baseDir string) *TSLManager {
	return &TSLManager{
		baseDir: baseDir,
	}
}

// LoadTSL 从文件加载TSL
func (m *TSLManager) LoadTSL(filename string) (*TSLModel, error) {
	var filePath string
	if filepath.IsAbs(filename) {
		// 如果是绝对路径，直接使用
		filePath = filename
		log.Printf("TSL加载使用绝对路径: %s", filePath)
	} else {
		// 如果是相对路径，添加baseDir和configs前缀
		filePath = filepath.Join(m.baseDir, "configs", filename)
		log.Printf("TSL加载使用相对路径: %s -> %s", filename, filePath)
	}
	
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取TSL文件失败: %v", err)
	}

	var tslModel TSLModel
	if err := json.Unmarshal(data, &tslModel); err != nil {
		return nil, fmt.Errorf("解析TSL失败: %v", err)
	}

	return &tslModel, nil
}

// SaveTSL 保存TSL到文件
func (m *TSLManager) SaveTSL(filename string, model *TSLModel) error {
	filePath := filepath.Join(m.baseDir, "configs", filename)
	
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	data, err := json.MarshalIndent(model, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化TSL失败: %v", err)
	}

	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("保存TSL文件失败: %v", err)
	}

	return nil
}

// ListTSLFiles 列出所有TSL文件
func (m *TSLManager) ListTSLFiles() ([]string, error) {
	configsDir := filepath.Join(m.baseDir, "configs")
	files, err := os.ReadDir(configsDir)
	if err != nil {
		return nil, err
	}

	var tslFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "tsl_") && strings.HasSuffix(file.Name(), ".json") {
			tslFiles = append(tslFiles, file.Name())
		}
	}

	return tslFiles, nil
}

// ValidateTSL 验证TSL模型
func (m *TSLManager) ValidateTSL(model *TSLModel) error {
	if model == nil {
		return fmt.Errorf("TSL模型不能为空")
	}

	// 验证属性
	for _, prop := range model.Properties {
		if prop.Identifier == "" {
			return fmt.Errorf("属性标识符不能为空")
		}
		if prop.Name == "" {
			return fmt.Errorf("属性名称不能为空")
		}
		if err := m.validateDataType(prop.GetDataType()); err != nil {
			return fmt.Errorf("属性[%s]数据类型无效: %v", prop.Identifier, err)
		}
	}

	// 验证事件
	for _, event := range model.Events {
		if event.Identifier == "" {
			return fmt.Errorf("事件标识符不能为空")
		}
		if event.Name == "" {
			return fmt.Errorf("事件名称不能为空")
		}
	}

	// 验证服务
	for _, action := range model.Actions {
		if action.Identifier == "" {
			return fmt.Errorf("服务标识符不能为空")
		}
		if action.Name == "" {
			return fmt.Errorf("服务名称不能为空")
		}
	}

	return nil
}

// validateDataType 验证数据类型
func (m *TSLManager) validateDataType(dt DataType) error {
	validTypes := []string{"int", "long", "float", "double", "bool", "text", "string", "enum"}
	
	valid := false
	for _, validType := range validTypes {
		if dt.Type == validType {
			valid = true
			break
		}
	}
	
	if !valid {
		return fmt.Errorf("不支持的数据类型: '%s' (长度:%d)", dt.Type, len(dt.Type))
	}

	// 对于数值类型，验证范围
	if dt.Type == "int" || dt.Type == "long" || dt.Type == "float" || dt.Type == "double" {
		if dt.Specs.Min >= dt.Specs.Max && (dt.Specs.Min != 0 || dt.Specs.Max != 0) {
			return fmt.Errorf("最小值不能大于等于最大值")
		}
	}

	return nil
}

// GetProductNameFromTSLFile 从TSL文件名提取产品名称
func GetProductNameFromTSLFile(filename string) string {
	base := filepath.Base(filename)
	if strings.HasPrefix(base, "tsl_") {
		productName := strings.TrimPrefix(base, "tsl_")
		productName = strings.TrimSuffix(productName, ".json")
		return productName
	}
	return ""
}

// GenerateTSLFileName 生成TSL文件名
func GenerateTSLFileName(productName string) string {
	return fmt.Sprintf("tsl_%s.json", productName)
}