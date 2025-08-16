package simulator

import (
	"math/rand"
	"time"
)

// ServiceSimulator 服务模拟器
type ServiceSimulator struct {
	// 可以添加服务相关的内部状态
}

// NewServiceSimulator 创建服务模拟器
func NewServiceSimulator() *ServiceSimulator {
	return &ServiceSimulator{}
}

// SimulateServiceResponse 根据配置生成服务响应
func (ss *ServiceSimulator) SimulateServiceResponse(config ServiceSimConfig) ServiceResponse {
	switch config.ResponseStrategy {
	case "fixed":
		return ss.getFixedResponse(config)
	case "random", "randomPick":
		return ss.getRandomResponse(config)
	default:
		// 默认返回成功响应
		return ServiceResponse{
			Code: 200,
			Msg:  "ok",
			Desc: "操作成功",
		}
	}
}

// getFixedResponse 获取固定响应（总是返回第一个响应）
func (ss *ServiceSimulator) getFixedResponse(config ServiceSimConfig) ServiceResponse {
	if len(config.PossibleResponses) > 0 {
		return config.PossibleResponses[0]
	}
	
	return ServiceResponse{
		Code: 200,
		Msg:  "ok",
		Desc: "操作成功",
	}
}

// getRandomResponse 获取随机响应
func (ss *ServiceSimulator) getRandomResponse(config ServiceSimConfig) ServiceResponse {
	if len(config.PossibleResponses) == 0 {
		return ServiceResponse{
			Code: 200,
			Msg:  "ok",
			Desc: "操作成功",
		}
	}
	
	idx := rand.Intn(len(config.PossibleResponses))
	return config.PossibleResponses[idx]
}

// SimulateServiceDelay 模拟服务处理延时
func (ss *ServiceSimulator) SimulateServiceDelay(minDelayMs, maxDelayMs int) {
	if minDelayMs <= 0 && maxDelayMs <= 0 {
		return // 无延时
	}
	
	if minDelayMs > maxDelayMs {
		minDelayMs, maxDelayMs = maxDelayMs, minDelayMs
	}
	
	var delayMs int
	if minDelayMs == maxDelayMs {
		delayMs = minDelayMs
	} else {
		delayMs = minDelayMs + rand.Intn(maxDelayMs-minDelayMs)
	}
	
	if delayMs > 0 {
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}
}

// ValidateServiceConfig 验证服务配置
func (ss *ServiceSimulator) ValidateServiceConfig(config ServiceSimConfig) error {
	// 使用之前在 rule_types.go 中定义的验证逻辑
	rm := &RuleManager{}
	return rm.validateServiceConfig(config)
}

// GetResponseByCode 根据状态码获取响应
func (ss *ServiceSimulator) GetResponseByCode(config ServiceSimConfig, code int) *ServiceResponse {
	for _, response := range config.PossibleResponses {
		if response.Code == code {
			return &response
		}
	}
	return nil
}

// GetSuccessResponse 获取成功响应（2xx状态码）
func (ss *ServiceSimulator) GetSuccessResponse(config ServiceSimConfig) *ServiceResponse {
	for _, response := range config.PossibleResponses {
		if response.Code >= 200 && response.Code < 300 {
			return &response
		}
	}
	return nil
}

// GetErrorResponse 获取错误响应（非2xx状态码）
func (ss *ServiceSimulator) GetErrorResponse(config ServiceSimConfig) *ServiceResponse {
	for _, response := range config.PossibleResponses {
		if response.Code < 200 || response.Code >= 300 {
			return &response
		}
	}
	return nil
}

// CalculateSuccessRate 计算成功率（基于配置的响应列表）
func (ss *ServiceSimulator) CalculateSuccessRate(config ServiceSimConfig) float64 {
	if len(config.PossibleResponses) == 0 {
		return 1.0 // 默认100%成功率
	}
	
	successCount := 0
	for _, response := range config.PossibleResponses {
		if response.Code >= 200 && response.Code < 300 {
			successCount++
		}
	}
	
	return float64(successCount) / float64(len(config.PossibleResponses))
}

// GenerateResponseWithSuccessRate 根据指定成功率生成响应
func (ss *ServiceSimulator) GenerateResponseWithSuccessRate(config ServiceSimConfig, successRate float64) ServiceResponse {
	if rand.Float64() < successRate {
		// 返回成功响应
		successResp := ss.GetSuccessResponse(config)
		if successResp != nil {
			return *successResp
		}
	}
	
	// 返回错误响应
	errorResp := ss.GetErrorResponse(config)
	if errorResp != nil {
		return *errorResp
	}
	
	// 如果没有配置错误响应，返回默认错误
	return ServiceResponse{
		Code: 500,
		Msg:  "error",
		Desc: "操作失败",
	}
}