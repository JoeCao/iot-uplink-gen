package simulator

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/iot-go-sdk/pkg/framework/core"
	"znb/iot-uplink-gen/llm"
	"znb/iot-uplink-gen/tsl"
)

// SimulatedDevice 基于framework的模拟设备
type SimulatedDevice struct {
	core.BaseDevice

	// TSL和规则
	tslModel *tsl.TSLModel
	rule     *SimulationRule

	// 模拟器组件
	propertySim *PropertySimulator
	eventSim    *EventSimulator
	serviceSim  *ServiceSimulator

	// Framework引用
	framework core.Framework

	// 运行时状态
	running        bool
	stopCh         chan struct{}
	ticker         *time.Ticker
	mutex          sync.RWMutex
	lastReportTime time.Time

	// 统计信息
	stats SimulatorStats

	// 配置
	uploadInterval time.Duration
	logCallback    func(string)
}

// SimulatorStats 模拟器统计信息
type SimulatorStats struct {
	PropertyUpdates int64 `json:"propertyUpdates"`
	EventTriggers   int64 `json:"eventTriggers"`
	ServiceCalls    int64 `json:"serviceCalls"`
	Errors          int64 `json:"errors"`
	StartTime       int64 `json:"startTime"`
}

// NewSimulatedDevice 创建模拟设备
func NewSimulatedDevice(productKey, deviceName, deviceSecret string, tslModel *tsl.TSLModel, rule *SimulationRule) *SimulatedDevice {
	return &SimulatedDevice{
		BaseDevice: core.BaseDevice{
			DeviceInfo: core.DeviceInfo{
				ProductKey:   productKey,
				DeviceName:   deviceName,
				DeviceSecret: deviceSecret,
				Model:        "Simulator-" + rule.ProductName,
				Version:      "1.0.0",
			},
		},
		tslModel:       tslModel,
		rule:           rule,
		propertySim:    NewPropertySimulator(),
		eventSim:       NewEventSimulator(),
		serviceSim:     NewServiceSimulator(),
		stopCh:         make(chan struct{}),
		uploadInterval: 30 * time.Second, // 默认30秒上报间隔
		stats: SimulatorStats{
			StartTime: time.Now().Unix(),
		},
	}
}

// SetFramework 设置框架引用
func (sd *SimulatedDevice) SetFramework(framework core.Framework) {
	sd.framework = framework
}

// SetUploadInterval 设置上报间隔
func (sd *SimulatedDevice) SetUploadInterval(interval time.Duration) {
	sd.uploadInterval = interval
}

// SetLogCallback 设置日志回调
func (sd *SimulatedDevice) SetLogCallback(callback func(string)) {
	sd.logCallback = callback
}

// OnInitialize 设备初始化
func (sd *SimulatedDevice) OnInitialize(ctx context.Context) error {
	sd.log(fmt.Sprintf("[%s] 初始化模拟设备: %s", sd.DeviceInfo.DeviceName, sd.rule.ProductName))

	// 注册TSL定义的属性
	sd.log(fmt.Sprintf("[%s] 注册属性...", sd.DeviceInfo.DeviceName))
	for _, prop := range sd.tslModel.Properties {
		getter := func(identifier string) func() interface{} {
			return func() interface{} {
				return sd.getPropertyValue(identifier)
			}
		}(prop.Identifier)

		var setter func(interface{}) error = nil
		if prop.AccessMode == "rw" {
			setter = func(identifier string) func(interface{}) error {
				return func(value interface{}) error {
					return sd.setPropertyValue(identifier, value)
				}
			}(prop.Identifier)
		}

		if err := sd.framework.RegisterProperty(prop.Identifier, getter, setter); err != nil {
			return fmt.Errorf("注册属性[%s]失败: %v", prop.Identifier, err)
		}
	}

	// 注册TSL定义的服务
	sd.log(fmt.Sprintf("[%s] 注册服务...", sd.DeviceInfo.DeviceName))
	for _, action := range sd.tslModel.Actions {
		handler := func(identifier string) func(map[string]interface{}) (interface{}, error) {
			return func(params map[string]interface{}) (interface{}, error) {
				return sd.handleService(identifier, params)
			}
		}(action.Identifier)

		if err := sd.framework.RegisterService(action.Identifier, handler); err != nil {
			return fmt.Errorf("注册服务[%s]失败: %v", action.Identifier, err)
		}
	}

	sd.log(fmt.Sprintf("[%s] 模拟设备初始化完成", sd.DeviceInfo.DeviceName))
	return nil
}

// OnConnect 设备连接
func (sd *SimulatedDevice) OnConnect(ctx context.Context) error {
	sd.log(fmt.Sprintf("[%s] 设备已连接到IoT平台", sd.DeviceInfo.DeviceName))

	// 启动模拟器
	sd.startSimulation()

	// 立即上报一次状态
	sd.reportCurrentStatus()

	return nil
}

// OnDisconnect 设备断开连接
func (sd *SimulatedDevice) OnDisconnect(ctx context.Context) error {
	sd.log(fmt.Sprintf("[%s] 设备与IoT平台断开连接", sd.DeviceInfo.DeviceName))
	return nil
}

// OnDestroy 设备销毁
func (sd *SimulatedDevice) OnDestroy(ctx context.Context) error {
	sd.log(fmt.Sprintf("[%s] 销毁模拟设备...", sd.DeviceInfo.DeviceName))

	// 停止模拟
	sd.stopSimulation()

	sd.log(fmt.Sprintf("[%s] 模拟设备已销毁", sd.DeviceInfo.DeviceName))
	return nil
}

// OnPropertySet 处理属性设置
func (sd *SimulatedDevice) OnPropertySet(property core.Property) error {
	sd.log(fmt.Sprintf("[%s] 接收到属性设置请求: %s = %v", sd.DeviceInfo.DeviceName, property.Name, property.Value))

	// 验证属性是否存在于TSL中
	found := false
	for _, prop := range sd.tslModel.Properties {
		if prop.Identifier == property.Name {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("属性[%s]不存在于TSL定义中", property.Name)
	}

	// 这里可以根据需要实现属性设置的业务逻辑
	// 对于模拟器，我们可能需要更新模拟器的内部状态

	return nil
}

// OnServiceInvoke 处理服务调用
func (sd *SimulatedDevice) OnServiceInvoke(service core.ServiceRequest) (core.ServiceResponse, error) {
	sd.log(fmt.Sprintf("[%s] 接收到服务调用: %s, 参数: %v", sd.DeviceInfo.DeviceName, service.Service, service.Params))

	// 处理特殊的系统服务
	switch service.Service {
	case "update_tsl":
		return sd.handleUpdateTSL(service)
	case "generate_rule":
		return sd.handleGenerateRule(service)
	default:
		// 服务已经通过RegisterService注册了处理器
		return core.ServiceResponse{
			ID:        service.ID,
			Code:      200,
			Message:   "服务由注册的处理器处理",
			Timestamp: time.Now(),
		}, nil
	}
}

// OnPropertyGet 处理属性获取
func (sd *SimulatedDevice) OnPropertyGet(name string) (interface{}, error) {
	return sd.getPropertyValue(name), nil
}

// OnEventReceive 处理接收到的事件
func (sd *SimulatedDevice) OnEventReceive(event core.DeviceEvent) error {
	sd.log(fmt.Sprintf("[%s] 接收到事件: %s", sd.DeviceInfo.DeviceName, event.Name))
	return nil
}

// OnOTANotify 处理OTA通知
func (sd *SimulatedDevice) OnOTANotify(task core.OTATask) error {
	sd.log(fmt.Sprintf("[%s] 接收到OTA通知: 版本 %s", sd.DeviceInfo.DeviceName, task.Version))
	// 这里可以实现OTA升级的模拟逻辑
	return nil
}

// startSimulation 启动模拟
func (sd *SimulatedDevice) startSimulation() {
	sd.mutex.Lock()
	defer sd.mutex.Unlock()

	if sd.running {
		return
	}

	sd.running = true
	sd.ticker = time.NewTicker(sd.uploadInterval)

	go sd.simulationLoop()

	sd.log(fmt.Sprintf("[%s] 模拟器已启动，上报间隔: %v", sd.DeviceInfo.DeviceName, sd.uploadInterval))
}

// stopSimulation 停止模拟
func (sd *SimulatedDevice) stopSimulation() {
	sd.mutex.Lock()
	defer sd.mutex.Unlock()

	if !sd.running {
		return
	}

	sd.running = false
	if sd.ticker != nil {
		sd.ticker.Stop()
	}

	select {
	case <-sd.stopCh:
	default:
		close(sd.stopCh)
	}

	sd.log(fmt.Sprintf("[%s] 模拟器已停止", sd.DeviceInfo.DeviceName))
}

// simulationLoop 模拟循环
func (sd *SimulatedDevice) simulationLoop() {
	for {
		select {
		case <-sd.stopCh:
			return
		case <-sd.ticker.C:
			sd.runSimulationCycle()
		}
	}
}

// runSimulationCycle 运行一个模拟周期
func (sd *SimulatedDevice) runSimulationCycle() {
	// 1. 生成属性数据
	propertyData := sd.generatePropertyData()

	// 2. 检查并触发事件
	sd.checkAndTriggerEvents(propertyData)

	// 3. 上报属性数据
	sd.reportProperties(propertyData)

	// 更新统计
	atomic.AddInt64(&sd.stats.PropertyUpdates, 1)
	sd.lastReportTime = time.Now()
}

// generatePropertyData 生成属性数据
func (sd *SimulatedDevice) generatePropertyData() map[string]interface{} {
	properties := make(map[string]interface{})

	for _, prop := range sd.tslModel.Properties {
		// 检查是否有对应的模拟配置
		config, exists := sd.rule.SimulationConfig[prop.Identifier]
		if !exists {
			continue
		}

		// 生成模拟值
		value := sd.propertySim.SimulateValue(prop.Identifier, config)
		properties[prop.Identifier] = value
	}

	return properties
}

// checkAndTriggerEvents 检查并触发事件
func (sd *SimulatedDevice) checkAndTriggerEvents(propertyData map[string]interface{}) {
	for _, eventConfig := range sd.rule.Events {
		if triggered, eventData := sd.eventSim.CheckEventTrigger(eventConfig, propertyData); triggered {
			// 发布事件
			if err := sd.framework.ReportEvent(eventConfig.Identifier, eventData); err != nil {
				sd.log(fmt.Sprintf("[%s] 发布事件[%s]失败: %v", sd.DeviceInfo.DeviceName, eventConfig.Identifier, err))
				atomic.AddInt64(&sd.stats.Errors, 1)
			} else {
				sd.log(fmt.Sprintf("[%s] 事件[%s]已触发", sd.DeviceInfo.DeviceName, eventConfig.Identifier))
				atomic.AddInt64(&sd.stats.EventTriggers, 1)
			}
		}
	}
}

// reportProperties 上报属性
func (sd *SimulatedDevice) reportProperties(properties map[string]interface{}) {
	if len(properties) == 0 {
		return
	}

	if err := sd.framework.ReportProperties(properties); err != nil {
		sd.log(fmt.Sprintf("[%s] 上报属性失败: %v", sd.DeviceInfo.DeviceName, err))
		atomic.AddInt64(&sd.stats.Errors, 1)
	} else {
		sd.log(fmt.Sprintf("[%s] 属性上报成功: %d个属性", sd.DeviceInfo.DeviceName, len(properties)))
	}
}

// reportCurrentStatus 立即上报当前状态
func (sd *SimulatedDevice) reportCurrentStatus() {
	propertyData := sd.generatePropertyData()
	sd.reportProperties(propertyData)
}

// getPropertyValue 获取属性值
func (sd *SimulatedDevice) getPropertyValue(identifier string) interface{} {
	config, exists := sd.rule.SimulationConfig[identifier]
	if !exists {
		return nil
	}

	return sd.propertySim.SimulateValue(identifier, config)
}

// setPropertyValue 设置属性值
func (sd *SimulatedDevice) setPropertyValue(identifier string, value interface{}) error {
	// 验证属性值
	config, exists := sd.rule.SimulationConfig[identifier]
	if exists {
		if err := sd.propertySim.ValidatePropertyValue(value, config); err != nil {
			return fmt.Errorf("属性值验证失败: %v", err)
		}
	}

	sd.log(fmt.Sprintf("[%s] 属性[%s]设置为: %v", sd.DeviceInfo.DeviceName, identifier, value))
	return nil
}

// handleService 处理服务调用
func (sd *SimulatedDevice) handleService(identifier string, params map[string]interface{}) (interface{}, error) {
	sd.log(fmt.Sprintf("[%s] 处理服务[%s]调用, 参数: %v", sd.DeviceInfo.DeviceName, identifier, params))

	// 查找服务配置
	config, exists := sd.rule.Services[identifier]
	if !exists {
		return nil, fmt.Errorf("服务[%s]未配置", identifier)
	}

	// 模拟服务处理延时（固定3秒）
	time.Sleep(3 * time.Second)

	// 生成响应
	response := sd.serviceSim.SimulateServiceResponse(config)

	atomic.AddInt64(&sd.stats.ServiceCalls, 1)

	sd.log(fmt.Sprintf("[%s] 服务[%s]响应: code=%d, msg=%s", sd.DeviceInfo.DeviceName, identifier, response.Code, response.Msg))

	return map[string]interface{}{
		"code": response.Code,
		"msg":  response.Msg,
		"desc": response.Desc,
	}, nil
}

// IsRunning 检查模拟器是否运行
func (sd *SimulatedDevice) IsRunning() bool {
	sd.mutex.RLock()
	defer sd.mutex.RUnlock()
	return sd.running
}

// GetStats 获取统计信息
func (sd *SimulatedDevice) GetStats() SimulatorStats {
	return SimulatorStats{
		PropertyUpdates: atomic.LoadInt64(&sd.stats.PropertyUpdates),
		EventTriggers:   atomic.LoadInt64(&sd.stats.EventTriggers),
		ServiceCalls:    atomic.LoadInt64(&sd.stats.ServiceCalls),
		Errors:          atomic.LoadInt64(&sd.stats.Errors),
		StartTime:       sd.stats.StartTime,
	}
}

// GetProductName 获取产品名称
func (sd *SimulatedDevice) GetProductName() string {
	return sd.rule.ProductName
}

// log 输出日志
func (sd *SimulatedDevice) log(msg string) {
	log.Println(msg)
	if sd.logCallback != nil {
		sd.logCallback(msg)
	}
}

// handleUpdateTSL 处理TSL更新服务
func (sd *SimulatedDevice) handleUpdateTSL(service core.ServiceRequest) (core.ServiceResponse, error) {
	sd.log(fmt.Sprintf("[%s] 处理TSL更新请求", sd.DeviceInfo.DeviceName))

	// 从参数中获取TSL内容
	tslContent, ok := service.Params["tsl_content"].(string)
	if !ok {
		return core.ServiceResponse{
			ID:        service.ID,
			Code:      400,
			Message:   "参数错误: 缺少tsl_content参数",
			Timestamp: time.Now(),
		}, nil
	}

	// 使用统一的TSL处理流程
	result, err := llm.ProcessTSLContent(tslContent)
	if err != nil {
		return core.ServiceResponse{
			ID:        service.ID,
			Code:      500,
			Message:   fmt.Sprintf("处理TSL失败: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	sd.log(fmt.Sprintf("[%s] TSL处理完成: 产品=%s, TSL文件=%s, Rule文件=%s", 
		sd.DeviceInfo.DeviceName, result.ProductName, result.TSLFile, result.RuleFile))

	return core.ServiceResponse{
		ID:        service.ID,
		Code:      200,
		Message:   "TSL更新成功",
		Data: map[string]interface{}{
			"product_name": result.ProductName,
			"tsl_file":     result.TSLFile,
			"rule_file":    result.RuleFile,
		},
		Timestamp: time.Now(),
	}, nil
}

// handleGenerateRule 处理Rule生成服务
func (sd *SimulatedDevice) handleGenerateRule(service core.ServiceRequest) (core.ServiceResponse, error) {
	sd.log(fmt.Sprintf("[%s] 处理Rule生成请求", sd.DeviceInfo.DeviceName))

	// 从参数中获取TSL内容或文件路径
	var tslContent string
	if content, ok := service.Params["tsl_content"].(string); ok {
		tslContent = content
	} else if filePath, ok := service.Params["tsl_file"].(string); ok {
		// 从文件读取TSL内容
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			return core.ServiceResponse{
				ID:        service.ID,
				Code:      400,
				Message:   fmt.Sprintf("读取TSL文件失败: %v", err),
				Timestamp: time.Now(),
			}, nil
		}
		tslContent = string(data)
	} else {
		return core.ServiceResponse{
			ID:        service.ID,
			Code:      400,
			Message:   "参数错误: 需要tsl_content或tsl_file参数",
			Timestamp: time.Now(),
		}, nil
	}

	// 使用统一的TSL处理流程
	result, err := llm.ProcessTSLContent(tslContent)
	if err != nil {
		return core.ServiceResponse{
			ID:        service.ID,
			Code:      500,
			Message:   fmt.Sprintf("处理TSL失败: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	sd.log(fmt.Sprintf("[%s] Rule生成完成: 产品=%s, TSL文件=%s, Rule文件=%s", 
		sd.DeviceInfo.DeviceName, result.ProductName, result.TSLFile, result.RuleFile))

	return core.ServiceResponse{
		ID:        service.ID,
		Code:      200,
		Message:   "Rule生成成功",
		Data: map[string]interface{}{
			"product_name": result.ProductName,
			"tsl_file":     result.TSLFile,
			"rule_file":    result.RuleFile,
			"rule_content": result.RuleContent,
		},
		Timestamp: time.Now(),
	}, nil
}