package device

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/iot-go-sdk/pkg/framework/core"
)

// SensorDevice represents a smart sensor device with temperature, humidity, pressure monitoring
type SensorDevice struct {
	core.BaseDevice

	// Properties
	temperature float64 // 当前温度
	humidity    float64 // 当前湿度 
	pressure    float64 // 当前气压
	battery     float64 // 电池电量

	// Internal state
	isRunning    bool
	mutex        sync.RWMutex
	framework    core.Framework
	stopCh       chan struct{}
	lastReportTime time.Time
}

// NewSensorDevice creates a new sensor device
func NewSensorDevice(productKey, deviceName, deviceSecret string) *SensorDevice {
	return &SensorDevice{
		BaseDevice: core.BaseDevice{
			DeviceInfo: core.DeviceInfo{
				ProductKey:   productKey,
				DeviceName:   deviceName,
				DeviceSecret: deviceSecret,
				Model:        "SmartSensor-X1",
				Version:      "1.0.0",
			},
		},
		temperature: 25.0, // Room temperature
		humidity:    45.0, // Normal humidity
		pressure:    1013.25, // Standard pressure
		battery:     100.0, // Full battery
		stopCh:      make(chan struct{}),
	}
}

// OnInitialize is called when the device is initialized
func (s *SensorDevice) OnInitialize(ctx context.Context) error {
	log.Printf("[%s] Initializing sensor device...", s.DeviceInfo.DeviceName)

	// Register properties
	log.Printf("[%s] Registering properties...", s.DeviceInfo.DeviceName)
	s.framework.RegisterProperty("temperature", s.getTemperature, nil)
	s.framework.RegisterProperty("humidity", s.getHumidity, nil)
	s.framework.RegisterProperty("pressure", s.getPressure, nil)
	s.framework.RegisterProperty("battery", s.getBattery, nil)

	// Register services
	log.Printf("[%s] Registering services...", s.DeviceInfo.DeviceName)
	s.framework.RegisterService("calibrate_sensor", s.calibrateSensorService)
	s.framework.RegisterService("reset_device", s.resetDeviceService)

	// Start simulation
	log.Printf("[%s] Starting sensor simulation...", s.DeviceInfo.DeviceName)
	s.startSimulation()

	log.Printf("[%s] Sensor device initialized successfully", s.DeviceInfo.DeviceName)
	return nil
}

// OnConnect is called when the device connects to the platform
func (s *SensorDevice) OnConnect(ctx context.Context) error {
	log.Printf("[%s] Sensor device connected to IoT platform", s.DeviceInfo.DeviceName)

	// Report initial state
	s.reportFullStatus()

	return nil
}

// OnDisconnect is called when the device disconnects from the platform
func (s *SensorDevice) OnDisconnect(ctx context.Context) error {
	log.Printf("[%s] Sensor device disconnected from IoT platform", s.DeviceInfo.DeviceName)
	return nil
}

// OnDestroy is called when the device is being destroyed
func (s *SensorDevice) OnDestroy(ctx context.Context) error {
	log.Printf("[%s] Destroying sensor device...", s.DeviceInfo.DeviceName)

	// Stop simulation gracefully
	select {
	case <-s.stopCh:
		// Already closed
	default:
		close(s.stopCh)
	}

	log.Printf("[%s] Sensor device destroyed successfully", s.DeviceInfo.DeviceName)
	return nil
}

// OnPropertySet handles property set requests from the cloud
func (s *SensorDevice) OnPropertySet(property core.Property) error {
	log.Printf("[%s] Property set request: %s = %v", s.DeviceInfo.DeviceName, property.Name, property.Value)

	// For this example, all properties are read-only
	return fmt.Errorf("property %s is read-only", property.Name)
}

// OnServiceInvoke handles service invocation from the cloud
func (s *SensorDevice) OnServiceInvoke(service core.ServiceRequest) (core.ServiceResponse, error) {
	log.Printf("[%s] Service invoke: %s with params %v", s.DeviceInfo.DeviceName, service.Service, service.Params)

	// Services are handled via registered handlers
	return core.ServiceResponse{
		ID:        service.ID,
		Code:      -1,
		Message:   "Service handled by framework",
		Timestamp: time.Now(),
	}, nil
}

// OnPropertyGet handles property get requests
func (s *SensorDevice) OnPropertyGet(name string) (interface{}, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	switch name {
	case "temperature":
		return s.temperature, nil
	case "humidity":
		return s.humidity, nil
	case "pressure":
		return s.pressure, nil
	case "battery":
		return s.battery, nil
	default:
		return nil, fmt.Errorf("property %s not found", name)
	}
}

// OnEventReceive handles incoming events
func (s *SensorDevice) OnEventReceive(event core.DeviceEvent) error {
	log.Printf("[%s] Received event: %s", s.DeviceInfo.DeviceName, event.Name)
	return nil
}

// OnOTANotify handles OTA notifications
func (s *SensorDevice) OnOTANotify(task core.OTATask) error {
	log.Printf("[%s] OTA notification: version %s", s.DeviceInfo.DeviceName, task.Version)
	return nil
}

// Property getters
func (s *SensorDevice) getTemperature() interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.temperature
}

func (s *SensorDevice) getHumidity() interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.humidity
}

func (s *SensorDevice) getPressure() interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.pressure
}

func (s *SensorDevice) getBattery() interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.battery
}

// Service handlers
func (s *SensorDevice) calibrateSensorService(params map[string]interface{}) (interface{}, error) {
	log.Printf("[%s] Calibrating sensors...", s.DeviceInfo.DeviceName)
	
	// Simulate calibration
	time.Sleep(2 * time.Second)
	
	return map[string]interface{}{
		"success": true,
		"message": "Sensors calibrated successfully",
	}, nil
}

func (s *SensorDevice) resetDeviceService(params map[string]interface{}) (interface{}, error) {
	log.Printf("[%s] Resetting device...", s.DeviceInfo.DeviceName)
	
	s.mutex.Lock()
	s.temperature = 25.0
	s.humidity = 45.0
	s.pressure = 1013.25
	s.battery = 100.0
	s.mutex.Unlock()
	
	s.reportFullStatus()
	
	return map[string]interface{}{
		"success": true,
		"message": "Device reset successfully",
	}, nil
}

// startSimulation starts the sensor simulation
func (s *SensorDevice) startSimulation() {
	// Data collection loop
	go s.dataCollectionLoop()
	
	// Status reporting loop
	go s.statusReportingLoop()
}

// dataCollectionLoop simulates sensor data collection
func (s *SensorDevice) dataCollectionLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.updateSensorData()
		}
	}
}

// updateSensorData simulates sensor readings
func (s *SensorDevice) updateSensorData() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Simulate realistic sensor variations
	s.temperature += (rand.Float64() - 0.5) * 2.0 // ±1°C variation
	if s.temperature < 15.0 {
		s.temperature = 15.0
	} else if s.temperature > 35.0 {
		s.temperature = 35.0
	}

	s.humidity += (rand.Float64() - 0.5) * 5.0 // ±2.5% variation
	if s.humidity < 20.0 {
		s.humidity = 20.0
	} else if s.humidity > 80.0 {
		s.humidity = 80.0
	}

	s.pressure += (rand.Float64() - 0.5) * 10.0 // ±5 hPa variation
	if s.pressure < 980.0 {
		s.pressure = 980.0
	} else if s.pressure > 1050.0 {
		s.pressure = 1050.0
	}

	// Simulate battery drain
	s.battery -= rand.Float64() * 0.1
	if s.battery < 0 {
		s.battery = 0
	}

	// Check for low battery alert
	if s.battery < 20.0 && s.battery > 19.0 {
		s.triggerLowBatteryAlert()
	}
}

// statusReportingLoop periodically reports device status
func (s *SensorDevice) statusReportingLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.reportFullStatus()
		}
	}
}

// reportFullStatus reports all properties to the platform
func (s *SensorDevice) reportFullStatus() {
	s.mutex.RLock()
	status := map[string]interface{}{
		"temperature": s.temperature,
		"humidity":    s.humidity,
		"pressure":    s.pressure,
		"battery":     s.battery,
	}
	s.mutex.RUnlock()

	log.Printf("[%s] Reporting status: temp=%.1f°C, humidity=%.1f%%, pressure=%.1f hPa, battery=%.1f%%",
		s.DeviceInfo.DeviceName, status["temperature"],
		status["humidity"], status["pressure"], status["battery"])

	if err := s.framework.ReportProperties(status); err != nil {
		log.Printf("[%s] Failed to report properties: %v", s.DeviceInfo.DeviceName, err)
	}
}

// triggerLowBatteryAlert triggers a low battery alert event
func (s *SensorDevice) triggerLowBatteryAlert() {
	log.Printf("[%s] ALERT: Low battery! %.1f%%", s.DeviceInfo.DeviceName, s.battery)

	// Create low battery event
	payload := map[string]interface{}{
		"battery_level": s.battery,
		"message":      "Battery level is below 20%",
	}
	if err := s.framework.ReportEvent("low_battery_alert", payload); err != nil {
		log.Printf("[%s] Failed to report low battery event: %v", s.DeviceInfo.DeviceName, err)
	}
}

// SetFramework sets the framework reference
func (s *SensorDevice) SetFramework(framework core.Framework) {
	s.framework = framework
}
