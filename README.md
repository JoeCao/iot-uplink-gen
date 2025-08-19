# IoT设备模拟器框架

基于IoT框架的智能设备模拟器，支持TSL物模型驱动的设备仿真和一机一密认证。

## 🚀 功能特性

### 核心功能
- **📱 TSL物模型支持** - 自动加载和解析JSON格式的物模型文件
- **🎯 智能模拟规则** - 多种属性模拟方法（随机范围、波形、累积、枚举等）
- **🔄 实时数据上报** - 定时生成和发布模拟数据到IoT平台
- **⚡ 服务调用响应** - 模拟设备服务的智能响应
- **🎪 事件触发器** - 基于条件的自动事件触发
- **🏭 多设备支持** - 支持多种设备类型并发模拟

### 架构特性
- **🔐 一机一密认证** - 支持阿里云IoT标准认证方式
- **🌐 稳定MQTT连接** - 完善的重连机制和错误处理
- **📡 OTA升级支持** - 集成固件升级功能
- **📊 性能监控** - 实时模拟器状态和性能统计
- **🔧 插件化架构** - 基于framework的可扩展设计

## 📦 快速开始

### 安装依赖

```bash
go mod download
```

### 运行示例

#### 1. 智能调速电机模拟器

使用预配置的电机物模型：

```bash
# 运行智能调速电机模拟器
go run main.go -mode simulator -product 电机 -config config_smart_motor.json
```

#### 2. 从物模型生成配置

使用物模型文件自动生成模拟配置：

```bash
# 从物模型生成TSL和规则文件
go run cmd/generate_rule/main.go -tsl 智能调速电机物模型.json

# 运行生成的配置
go run main.go -mode simulator -product 电机 -config config_smart_motor.json
```

#### 3. 简单传感器模式

运行基础传感器模拟：

```bash
go run main.go -mode sensor
```

## 🛠️ 配置文件

### 设备配置 (config_smart_motor.json)

```json
{
  "Device": {
    "ProductKey": "FuWtDWoy",
    "DeviceName": "AzEYXBjJY5", 
    "DeviceSecret": "sbGnxntUSnDsKTc7",
    "Region": "cn-shanghai"
  },
  "MQTT": {
    "Host": "121.40.253.229",
    "Port": 1883,
    "UseTLS": false,
    "KeepAlive": 60,
    "CleanSession": true,
    "AutoReconnect": true
  },
  "Features": {
    "EnableOTA": true,
    "EnableShadow": false,
    "EnableRules": false,
    "EnableMetrics": true
  }
}
```

### TSL物模型文件

物模型定义设备的属性、事件和服务：

```json
{
  "properties": [
    {
      "identifier": "speed",
      "name": "转速",
      "data_type": {
        "type": "float",
        "specs": {"min": 0.0, "max": 3000.0, "unit": "rpm"}
      },
      "desc": "当前电机转速"
    }
  ],
  "events": [
    {
      "identifier": "overheat_alarm",
      "name": "温度过高告警",
      "event_type": "EVENT_TYPE_ALERT",
      "output_data": [
        {
          "identifier": "temperature",
          "name": "当前温度",
          "data_type": {"type": "float"}
        }
      ]
    }
  ],
  "actions": [
    {
      "identifier": "start_motor",
      "name": "启动电机",
      "input_data": [
        {
          "identifier": "mode",
          "name": "启动模式",
          "data_type": {"type": "text"}
        }
      ]
    }
  ]
}
```

### 模拟规则文件

定义属性值的模拟规则：

```json
{
  "productName": "电机",
  "simulationConfig": {
    "speed": {
      "method": "randomRange",
      "min": 500.0,
      "max": 2500.0,
      "step": 10.0
    },
    "temperature": {
      "method": "wave",
      "min": 20.0,
      "max": 80.0,
      "amplitude": 5.0,
      "wavePeriod": 300
    },
    "runtime": {
      "method": "accumulate",
      "start": 0,
      "step": 1
    }
  },
  "events": [
    {
      "identifier": "overheat_alarm",
      "triggerCondition": "temperature >= 85",
      "cooldown": 300
    }
  ],
  "services": {
    "start_motor": {
      "responseStrategy": "fixed",
      "possibleResponses": [
        {
          "code": 200,
          "msg": "启动成功",
          "desc": "电机启动成功"
        }
      ]
    }
  }
}
```

## 📚 模拟方法

### 属性模拟方法

| 方法 | 描述 | 配置参数 |
|------|------|----------|
| `randomRange` | 随机数值范围 | `min`, `max`, `step` |
| `wave` | 正弦波模拟 | `min`, `max`, `amplitude`, `wavePeriod` |
| `accumulate` | 累积增长 | `start`, `step` |
| `enumPick` | 枚举选择 | `enumValues`, `switchProbability` |

### 事件触发

支持基于条件的自动事件触发：

```json
{
  "identifier": "overheat_alarm",
  "triggerCondition": "temperature > 85.0",
  "cooldown": 300
}
```

### 服务响应

模拟设备服务调用的智能响应：

```json
{
  "responseStrategy": "fixed",
  "possibleResponses": [
    {
      "code": 200,
      "msg": "操作成功",
      "desc": "设备操作成功完成"
    },
    {
      "code": 500,
      "msg": "操作失败",
      "desc": "设备操作失败"
    }
  ]
}
```

## 🔧 命令行选项

```bash
go run main.go [选项]

选项:
  -mode string
        运行模式: sensor(传感器) 或 simulator(TSL模拟器) (默认 "sensor")
  -product string
        产品类型（TSL模拟器模式必需）
  -tsl string
        TSL文件路径（可选）
  -rule string
        规则文件路径（可选）
  -config string
        配置文件路径 (默认 "config.json")
```

## 📁 项目结构

```
iot-uplink-gen/
├── cmd/
│   └── generate_rule/          # TSL规则生成工具
├── config/                     # 配置管理
├── configs/                    # 配置文件目录
│   ├── tsl_*.json             # TSL物模型文件
│   └── rule_*.json            # 模拟规则文件
├── device/                     # 设备实现
├── simulator/                  # 模拟器核心
│   ├── device_factory.go      # 设备工厂
│   ├── simulated_device.go    # 模拟设备
│   ├── property_simulator.go  # 属性模拟器
│   ├── event_simulator.go     # 事件模拟器
│   └── service_simulator.go   # 服务模拟器
├── tsl/                       # TSL模型管理
├── llm/                       # LLM规则生成
└── main.go                    # 主程序入口
```

## 🎯 支持的设备类型

当前预配置的设备类型：

- **智能调速电机** - 工业电机模拟，支持转速控制和温度监控
- **空压机** - 气压设备模拟，支持压力控制和振动监测
- **智能传感器** - 通用传感器模拟，支持多种环境参数
- **温度设备** - 温度控制设备模拟
- **煅烧炉** - 工业炉具模拟，支持温度和气体监控

## 🔍 实时监控

模拟器提供详细的运行状态监控：

```
2025/08/19 11:54:53 [AzEYXBjJY5] 设备已连接到IoT平台
2025/08/19 11:54:53 [AzEYXBjJY5] 模拟器已启动，上报间隔: 30s
2025/08/19 11:54:53 [MQTT Plugin] Reported properties to $SYS/FuWtDWoy/AzEYXBjJY5/property/post
2025/08/19 11:54:53 [AzEYXBjJY5] 属性上报成功: 10个属性
```

## 🛡️ 认证支持

### 一机一密认证

支持阿里云IoT标准的一机一密认证方式：

```json
{
  "Device": {
    "ProductKey": "YourProductKey",
    "DeviceName": "YourDeviceName", 
    "DeviceSecret": "YourDeviceSecret"
  }
}
```

### 数据类型兼容性

自动处理数据类型兼容性问题：
- `int64` → `int`
- `int32` → `int`
- 确保与TSL验证器兼容

## 🔄 扩展开发

### 添加新设备类型

1. 创建物模型文件：`device_model.json`
2. 生成配置：`go run cmd/generate_rule/main.go -tsl device_model.json`
3. 运行模拟：`go run main.go -mode simulator -product 设备类型 -config your_config.json`

### 自定义模拟规则

修改`configs/rule_*.json`文件中的模拟配置，支持：
- 调整数值范围和步长
- 修改波形参数
- 设置事件触发条件
- 定制服务响应

## 📋 测试结果

### 智能调速电机测试

✅ **测试通过** - 2025/08/19

- **连接状态**: 成功连接到 `tcp://121.40.253.229:1883`
- **认证方式**: 一机一密认证
- **数据上报**: 10个属性正常上报
- **服务注册**: 3个服务成功注册 (start_motor, stop_motor, adjust_speed)
- **事件支持**: 温度过高告警事件正常工作
- **上报间隔**: 30秒定时上报

### 上报数据示例

```json
{
  "id": "1755575693",
  "version": "1.0",
  "params": {
    "speed": {"value": "903", "time": 1755575693},
    "temperature": {"value": "54", "time": 1755575693},
    "voltage": {"value": "246", "time": 1755575693},
    "current": {"value": "33", "time": 1755575693},
    "power": {"value": "6867", "time": 1755575693},
    "torque": {"value": "295", "time": 1755575693},
    "vibration": {"value": "3.6", "time": 1755575693},
    "efficiency": {"value": "86", "time": 1755575693},
    "frequency": {"value": "52", "time": 1755575693},
    "runtime": {"value": "1", "time": 1755575693}
  }
}
```

## 🤝 贡献

欢迎提交Issues和Pull Requests来改进这个项目。

## 📄 许可证

本项目采用MIT许可证。

## 🆘 故障排除

### 常见问题

1. **连接失败**: 检查网络连接和认证信息
2. **数据类型错误**: 确保TSL文件中使用`int`而非`int64`
3. **配置文件错误**: 验证JSON格式和必需字段

### 调试模式

设置日志级别为`debug`以获取详细信息：

```json
{
  "Logging": {
    "Level": "debug"
  }
}
```