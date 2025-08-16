package simulator

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// PropertySimulator 属性模拟器
type PropertySimulator struct {
	internalStates map[string]float64 // 保存累加、上次值等状态
}

// NewPropertySimulator 创建属性模拟器
func NewPropertySimulator() *PropertySimulator {
	return &PropertySimulator{
		internalStates: make(map[string]float64),
	}
}

// SimulateValue 根据配置和方法生成属性值
func (ps *PropertySimulator) SimulateValue(identifier string, config PropertySimConfig) interface{} {
	switch config.Method {
	case "randomRange":
		return ps.simulateRandomRange(identifier, config)
	case "wave":
		return ps.simulateWave(identifier, config)
	case "accumulate", "increase":
		return ps.simulateAccumulate(identifier, config)
	case "enum", "enumPick":
		return ps.simulateEnum(identifier, config)
	case "fixed":
		return ps.simulateFixed(identifier, config)
	default:
		return "0"
	}
}

// simulateRandomRange 模拟随机范围值
func (ps *PropertySimulator) simulateRandomRange(identifier string, config PropertySimConfig) interface{} {
	minF, _ := config.Min.Float64()
	maxF, _ := config.Max.Float64()
	
	// 计算min和max中最大的小数位数
	minDecimal := countDecimalPlaces(config.Min.String())
	maxDecimal := countDecimalPlaces(config.Max.String())
	decimalPlaces := minDecimal
	if maxDecimal > minDecimal {
		decimalPlaces = maxDecimal
	}
	
	// 生成随机数并格式化到指定小数位
	randomValue := minF + rand.Float64()*(maxF-minF)
	if decimalPlaces == 0 {
		// 整数类型，返回字符串形式
		return fmt.Sprintf("%d", int64(math.Round(randomValue)))
	}
	// 浮点数类型，返回字符串形式
	scale := math.Pow10(decimalPlaces)
	return fmt.Sprintf("%.*f", decimalPlaces, math.Round(randomValue*scale)/scale)
}

// simulateWave 模拟波形值
func (ps *PropertySimulator) simulateWave(identifier string, config PropertySimConfig) interface{} {
	minF, _ := config.Min.Float64()
	maxF, _ := config.Max.Float64()
	ampF, _ := config.Amplitude.Float64()
	center := (minF + maxF) / 2
	
	// 计算min和max中最大的小数位数
	minDecimal := countDecimalPlaces(config.Min.String())
	maxDecimal := countDecimalPlaces(config.Max.String())
	decimalPlaces := minDecimal
	if maxDecimal > minDecimal {
		decimalPlaces = maxDecimal
	}
	
	period := float64(config.WavePeriod)
	if period <= 0 {
		period = 60
	}
	
	now := time.Now().UnixNano()
	phase := float64(now) / 1e9 / period * 2 * math.Pi
	waveVal := math.Sin(phase)*ampF + center
	
	// 根据小数位数格式化结果
	if decimalPlaces == 0 {
		return fmt.Sprintf("%d", int64(math.Round(waveVal)))
	}
	scale := math.Pow10(decimalPlaces)
	return fmt.Sprintf("%.*f", decimalPlaces, math.Round(waveVal*scale)/scale)
}

// simulateAccumulate 模拟累加值
func (ps *PropertySimulator) simulateAccumulate(identifier string, config PropertySimConfig) interface{} {
	prevVal := ps.internalStates[identifier]
	stepF, _ := config.Step.Float64()
	newVal := prevVal + stepF
	ps.internalStates[identifier] = newVal
	
	// 根据是否有小数位返回对应格式的字符串
	if countDecimalPlaces(fmt.Sprintf("%v", stepF)) == 0 {
		return fmt.Sprintf("%d", int64(newVal))
	}
	return fmt.Sprintf("%.2f", newVal)
}

// simulateEnum 模拟枚举值
func (ps *PropertySimulator) simulateEnum(identifier string, config PropertySimConfig) interface{} {
	if len(config.EnumValues) == 0 {
		return ""
	}
	
	var idx int
	if prev, ok := ps.internalStates[identifier]; ok {
		idx = int(prev)
		// 根据切换概率决定是否切换到新值
		if rand.Float64() < config.SwitchProbability {
			idx = rand.Intn(len(config.EnumValues))
			ps.internalStates[identifier] = float64(idx)
		}
	} else {
		// 第一次选择随机值
		idx = rand.Intn(len(config.EnumValues))
		ps.internalStates[identifier] = float64(idx)
	}
	
	// 确保索引在有效范围内
	if idx >= 0 && idx < len(config.EnumValues) {
		return config.EnumValues[idx]
	}
	return ""
}

// simulateFixed 模拟固定值
func (ps *PropertySimulator) simulateFixed(identifier string, config PropertySimConfig) interface{} {
	return config.Value.String()
}

// ResetState 重置指定属性的内部状态
func (ps *PropertySimulator) ResetState(identifier string) {
	delete(ps.internalStates, identifier)
}

// ResetAllStates 重置所有属性的内部状态
func (ps *PropertySimulator) ResetAllStates() {
	ps.internalStates = make(map[string]float64)
}

// GetState 获取属性的内部状态
func (ps *PropertySimulator) GetState(identifier string) (float64, bool) {
	val, exists := ps.internalStates[identifier]
	return val, exists
}

// SetState 设置属性的内部状态
func (ps *PropertySimulator) SetState(identifier string, value float64) {
	ps.internalStates[identifier] = value
}

// GetAllStates 获取所有内部状态
func (ps *PropertySimulator) GetAllStates() map[string]float64 {
	states := make(map[string]float64)
	for k, v := range ps.internalStates {
		states[k] = v
	}
	return states
}

// countDecimalPlaces 计算小数位数
func countDecimalPlaces(s string) int {
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return 0
	}
	return len(parts[1])
}

// ValidatePropertyValue 验证属性值是否符合配置
func (ps *PropertySimulator) ValidatePropertyValue(value interface{}, config PropertySimConfig) error {
	switch config.Method {
	case "randomRange", "wave":
		// 验证数值范围
		var val float64
		switch v := value.(type) {
		case float64:
			val = v
		case string:
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return fmt.Errorf("值不是有效的数字: %v", value)
			}
			val = f
		default:
			return fmt.Errorf("值类型不支持: %T", value)
		}
		
		minF, _ := config.Min.Float64()
		maxF, _ := config.Max.Float64()
		if val < minF || val > maxF {
			return fmt.Errorf("值 %.2f 超出范围 [%.2f, %.2f]", val, minF, maxF)
		}
		
	case "enum", "enumPick":
		// 验证枚举值
		strVal := fmt.Sprintf("%v", value)
		valid := false
		for _, enumVal := range config.EnumValues {
			if strVal == enumVal {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("值 %v 不在枚举列表中: %v", value, config.EnumValues)
		}
	}
	
	return nil
}