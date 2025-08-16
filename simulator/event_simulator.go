package simulator

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
)

// EventSimulator 事件模拟器
type EventSimulator struct {
	lastTriggerTime map[string]int64 // 记录事件上次触发时间
}

// NewEventSimulator 创建事件模拟器
func NewEventSimulator() *EventSimulator {
	return &EventSimulator{
		lastTriggerTime: make(map[string]int64),
	}
}

// CheckEventTrigger 检查事件是否应该触发
func (es *EventSimulator) CheckEventTrigger(config EventSimConfig, propertyData map[string]interface{}) (bool, map[string]interface{}) {
	// 检查冷却时间
	if !es.canTriggerEvent(config) {
		return false, nil
	}

	// 检查触发条件
	if config.TriggerCondition == "" {
		return false, nil
	}

	triggered, eventData := es.evaluateCondition(config.TriggerCondition, propertyData)
	if triggered {
		// 更新最后触发时间
		es.lastTriggerTime[config.Identifier] = time.Now().Unix()
		
		// 构造事件数据
		eventPayload := map[string]interface{}{
			config.Identifier: map[string]interface{}{
				"value": eventData,
				"time":  time.Now().Unix(),
			},
		}
		
		return true, eventPayload
	}

	return false, nil
}

// canTriggerEvent 检查是否在冷却时间外
func (es *EventSimulator) canTriggerEvent(config EventSimConfig) bool {
	lastTime, exists := es.lastTriggerTime[config.Identifier]
	if !exists {
		return true
	}
	
	now := time.Now().Unix()
	return now-lastTime >= int64(config.Cooldown)
}

// evaluateCondition 评估触发条件
func (es *EventSimulator) evaluateCondition(condition string, propertyData map[string]interface{}) (bool, map[string]interface{}) {
	// 解析条件，支持格式: "temperature > 30", "humidity <= 80", "status == online"
	condition = strings.TrimSpace(condition)
	if condition == "" {
		return false, nil
	}

	// 简单解析条件
	operators := []string{">=", "<=", "==", "!=", ">", "<"}
	var prop, op, value string
	
	for _, operator := range operators {
		if strings.Contains(condition, operator) {
			parts := strings.Split(condition, operator)
			if len(parts) == 2 {
				prop = strings.TrimSpace(parts[0])
				op = operator
				value = strings.TrimSpace(parts[1])
				break
			}
		}
	}
	
	if prop == "" || op == "" || value == "" {
		log.Printf("无法解析触发条件: %s", condition)
		return false, nil
	}

	// 获取属性值
	propValue, exists := propertyData[prop]
	if !exists {
		log.Printf("属性 %s 不存在于当前数据中", prop)
		return false, nil
	}

	// 提取实际值
	var actualValue interface{}
	if propMap, ok := propValue.(map[string]interface{}); ok {
		actualValue = propMap["value"]
	} else {
		actualValue = propValue
	}

	// 进行比较
	result := es.compareValues(actualValue, op, value)
	
	if result {
		// 返回触发时的属性值
		eventData := map[string]interface{}{
			prop: actualValue,
		}
		return true, eventData
	}

	return false, nil
}

// compareValues 比较值
func (es *EventSimulator) compareValues(actualValue interface{}, operator, expectedValue string) bool {
	// 尝试数值比较
	if actualFloat, actualOk := es.convertToFloat(actualValue); actualOk {
		if expectedFloat, expectedOk := es.convertToFloat(expectedValue); expectedOk {
			return es.compareFloat(actualFloat, operator, expectedFloat)
		}
	}

	// 字符串比较
	actualStr := fmt.Sprintf("%v", actualValue)
	expectedStr := strings.Trim(expectedValue, "\"'")
	
	switch operator {
	case "==":
		return actualStr == expectedStr
	case "!=":
		return actualStr != expectedStr
	default:
		log.Printf("字符串类型不支持操作符: %s", operator)
		return false
	}
}

// convertToFloat 尝试将值转换为float64
func (es *EventSimulator) convertToFloat(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// compareFloat 比较float64值
func (es *EventSimulator) compareFloat(actual float64, operator string, expected float64) bool {
	switch operator {
	case ">":
		return actual > expected
	case ">=":
		return actual >= expected
	case "<":
		return actual < expected
	case "<=":
		return actual <= expected
	case "==":
		return math.Abs(actual-expected) < 1e-9 // 浮点数相等比较
	case "!=":
		return math.Abs(actual-expected) >= 1e-9
	default:
		log.Printf("不支持的操作符: %s", operator)
		return false
	}
}

// GetCooldownStatus 获取事件冷却状态
func (es *EventSimulator) GetCooldownStatus(identifier string, cooldown int) (bool, int64) {
	lastTime, exists := es.lastTriggerTime[identifier]
	if !exists {
		return false, 0 // 从未触发，没有冷却
	}
	
	now := time.Now().Unix()
	elapsed := now - lastTime
	remaining := int64(cooldown) - elapsed
	
	if remaining <= 0 {
		return false, 0 // 冷却已结束
	}
	
	return true, remaining // 正在冷却，返回剩余时间
}

// ResetEventCooldown 重置事件冷却时间
func (es *EventSimulator) ResetEventCooldown(identifier string) {
	delete(es.lastTriggerTime, identifier)
}

// GetEventTriggerHistory 获取事件触发历史
func (es *EventSimulator) GetEventTriggerHistory() map[string]int64 {
	history := make(map[string]int64)
	for k, v := range es.lastTriggerTime {
		history[k] = v
	}
	return history
}

// SetEventTriggerTime 设置事件触发时间（用于测试或状态恢复）
func (es *EventSimulator) SetEventTriggerTime(identifier string, timestamp int64) {
	es.lastTriggerTime[identifier] = timestamp
}

