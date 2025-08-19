package llm

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
)

// ProcessTSLContent 处理TSL内容，自动保存TSL文件并生成Rule文件
func ProcessTSLContent(tslContent string) (*TSLProcessResult, error) {
	// 从TSL内容中提取产品名称
	productName, err := extractProductNameFromTSL(tslContent)
	if err != nil {
		return nil, fmt.Errorf("提取产品名称失败: %v", err)
	}

	// 生成文件名
	safeName := strings.ReplaceAll(productName, " ", "_")
	safeName = strings.ReplaceAll(safeName, "/", "_")
	safeName = strings.ReplaceAll(safeName, "\\", "_")
	
	tslFileName := fmt.Sprintf("tsl_%s.json", safeName)
	ruleFileName := fmt.Sprintf("rule_%s.json", safeName)
	
	tslFilePath := filepath.Join("configs", tslFileName)
	ruleFilePath := filepath.Join("configs", ruleFileName)

	// 修复TSL内容中的数据类型问题
	fixedTSLContent := strings.ReplaceAll(tslContent, `"type":"int64"`, `"type":"int"`)
	fixedTSLContent = strings.ReplaceAll(fixedTSLContent, `"type":"int32"`, `"type":"int"`)
	
	// 保存修复后的TSL文件
	if err := ioutil.WriteFile(tslFilePath, []byte(fixedTSLContent), 0644); err != nil {
		return nil, fmt.Errorf("保存TSL文件失败: %v", err)
	}

	// 使用LLM生成Rule
	ruleContent, err := GenerateDeviceRule(tslContent)
	if err != nil {
		return nil, fmt.Errorf("生成Rule失败: %v", err)
	}

	// 保存Rule文件
	if err := ioutil.WriteFile(ruleFilePath, []byte(ruleContent), 0644); err != nil {
		return nil, fmt.Errorf("保存Rule文件失败: %v", err)
	}

	return &TSLProcessResult{
		ProductName:   productName,
		TSLFile:       tslFilePath,
		RuleFile:      ruleFilePath,
		TSLContent:    fixedTSLContent,
		RuleContent:   ruleContent,
	}, nil
}

// TSLProcessResult TSL处理结果
type TSLProcessResult struct {
	ProductName string `json:"productName"`
	TSLFile     string `json:"tslFile"`
	RuleFile    string `json:"ruleFile"`
	TSLContent  string `json:"tslContent"`
	RuleContent string `json:"ruleContent"`
}

// extractProductNameFromTSL 从TSL内容中提取产品名称
func extractProductNameFromTSL(tslContent string) (string, error) {
	// 尝试解析TSL JSON以查找产品名称
	var tslData map[string]interface{}
	if err := json.Unmarshal([]byte(tslContent), &tslData); err != nil {
		return "", fmt.Errorf("TSL JSON解析失败: %v", err)
	}

	// 优先级1: 尝试直接的产品名称字段
	productNameFields := []string{"productName", "product_name", "name", "deviceType", "device_type"}
	for _, field := range productNameFields {
		if value, exists := tslData[field]; exists {
			if productName, ok := value.(string); ok && productName != "" {
				return productName, nil
			}
		}
	}

	// 优先级2: 从actions(服务)名称中提取设备类型
	if actions, exists := tslData["actions"]; exists {
		if actionArray, ok := actions.([]interface{}); ok && len(actionArray) > 0 {
			// 查找包含设备类型信息的action名称
			for _, actionInterface := range actionArray {
				if action, ok := actionInterface.(map[string]interface{}); ok {
					if name, exists := action["name"]; exists {
						if nameStr, ok := name.(string); ok {
							// 从动作名称中提取设备类型
							if deviceType := extractDeviceTypeFromActionName(nameStr); deviceType != "" {
								return deviceType, nil
							}
						}
					}
				}
			}
		}
	}

	// 优先级3: 从events(事件)名称中提取设备类型
	if events, exists := tslData["events"]; exists {
		if eventArray, ok := events.([]interface{}); ok && len(eventArray) > 0 {
			for _, eventInterface := range eventArray {
				if event, ok := eventInterface.(map[string]interface{}); ok {
					if name, exists := event["name"]; exists {
						if nameStr, ok := name.(string); ok {
							// 从事件名称中提取设备类型
							if deviceType := extractDeviceTypeFromEventName(nameStr); deviceType != "" {
								return deviceType, nil
							}
						}
					}
				}
			}
		}
	}

	// 优先级4: 从属性名称中推断设备类型
	if properties, exists := tslData["properties"]; exists {
		if propArray, ok := properties.([]interface{}); ok && len(propArray) > 0 {
			// 收集所有属性描述和名称，尝试推断设备类型
			var descriptions []string
			var names []string
			for _, propInterface := range propArray {
				if prop, ok := propInterface.(map[string]interface{}); ok {
					if desc, exists := prop["desc"]; exists {
						if descStr, ok := desc.(string); ok && descStr != "" {
							descriptions = append(descriptions, descStr)
						}
					}
					if name, exists := prop["name"]; exists {
						if nameStr, ok := name.(string); ok && nameStr != "" {
							names = append(names, nameStr)
						}
					}
				}
			}
			
			// 优先从属性名称中推断设备类型
			if deviceType := inferDeviceTypeFromNames(names); deviceType != "" {
				return deviceType, nil
			}
			
			// 从描述中推断设备类型
			if deviceType := inferDeviceTypeFromDescriptions(descriptions); deviceType != "" {
				return deviceType, nil
			}
		}
	}

	// 如果仍然没有找到，使用默认名称
	return "未知设备", nil
}

// extractDeviceTypeFromActionName 从动作名称中提取设备类型
func extractDeviceTypeFromActionName(actionName string) string {
	// 常见的动作模式：启动/停止 + 设备名称
	if strings.Contains(actionName, "启动") {
		if strings.Contains(actionName, "煅烧炉") {
			return "煅烧炉"
		}
		if strings.Contains(actionName, "塔吊") {
			return "塔吊设备"
		}
		if strings.Contains(actionName, "机器人") {
			return "机器人"
		}
		if strings.Contains(actionName, "切割机") {
			return "切割机"
		}
		// 提取"启动"后面的设备名称
		if idx := strings.Index(actionName, "启动"); idx >= 0 && len(actionName) > idx+6 {
			deviceName := actionName[idx+6:] // "启动"是6个字节
			return deviceName
		}
	}
	if strings.Contains(actionName, "停止") {
		if idx := strings.Index(actionName, "停止"); idx >= 0 && len(actionName) > idx+6 {
			deviceName := actionName[idx+6:] // "停止"是6个字节  
			return deviceName
		}
	}
	return ""
}

// extractDeviceTypeFromEventName 从事件名称中提取设备类型
func extractDeviceTypeFromEventName(eventName string) string {
	// 从告警事件名称中推断设备类型
	if strings.Contains(eventName, "温度过高") && strings.Contains(eventName, "告警") {
		return "高温设备"
	}
	if strings.Contains(eventName, "超载") && strings.Contains(eventName, "告警") {
		return "起重设备"
	}
	return ""
}

// inferDeviceTypeFromNames 从属性名称中推断设备类型
func inferDeviceTypeFromNames(names []string) string {
	allNames := strings.Join(names, " ")
	
	// 优先匹配特定设备类型
	if strings.Contains(allNames, "醒发间") && strings.Contains(allNames, "烘烤") {
		return "面包房设备"
	}
	if strings.Contains(allNames, "醒发间") {
		return "醒发间设备" 
	}
	if strings.Contains(allNames, "烘烤") {
		return "烘烤设备"
	}
	if strings.Contains(allNames, "冷却") {
		return "冷却设备"
	}
	if strings.Contains(allNames, "煅烧") || strings.Contains(allNames, "炉") {
		return "煅烧炉"
	}
	if strings.Contains(allNames, "塔吊") {
		return "塔吊设备"
	}
	if strings.Contains(allNames, "机器人") {
		return "机器人设备"
	}
	
	return ""
}

// inferDeviceTypeFromDescriptions 从属性描述中推断设备类型
func inferDeviceTypeFromDescriptions(descriptions []string) string {
	allDesc := strings.Join(descriptions, " ")
	
	// 优先匹配特定设备类型
	if strings.Contains(allDesc, "煅烧炉") || strings.Contains(allDesc, "炉") {
		return "煅烧炉"
	}
	if strings.Contains(allDesc, "塔吊") {
		return "塔吊设备"
	}
	if strings.Contains(allDesc, "机器人") {
		return "机器人设备"
	}
	if strings.Contains(allDesc, "醒发间") {
		return "醒发间设备"
	}
	if strings.Contains(allDesc, "烘烤") {
		return "烘烤设备"
	}
	if strings.Contains(allDesc, "面包房") || strings.Contains(allDesc, "面包") {
		return "面包房设备"
	}
	
	// 通用设备类型推断
	if strings.Contains(allDesc, "温度") && strings.Contains(allDesc, "压力") {
		return "工业设备"
	}
	if strings.Contains(allDesc, "温度") && strings.Contains(allDesc, "湿度") {
		return "环境监测设备"
	}
	
	return ""
}

// GetProductNameFromTSLFile 从TSL文件中提取产品名称
func GetProductNameFromTSLFile(filePath string) (string, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return extractProductNameFromTSL(string(content))
}