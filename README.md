# 🚀 IoT多设备模拟器

高性能IoT设备仿真平台，支持TSL物模型驱动的多设备并发模拟。采用简化架构设计，一个命令启动多个设备，轻松扩展IoT设备生态。

## ✨ 核心特性

### 🏗️ 简化多设备架构 ⭐ 新特性
- **📂 一目录一设备** - 每个`deviceX/`目录包含完整的设备配置
- **🔍 自动设备发现** - 启动时自动扫描所有`device*`目录  
- **⚡ 并发模拟** - 多设备独立进程，真正的并行运行
- **🎛️ 即插即用** - 添加设备目录即可自动加入模拟
- **🔧 模板化生成** - 从预定义模板快速创建新设备

### 💡 智能模拟引擎
- **📱 TSL物模型驱动** - 完整支持阿里云IoT物模型规范
- **🎯 多种模拟算法** - 随机范围、正弦波、累积、枚举等模拟方法
- **🔄 实时数据上报** - 智能属性值生成和定时上报
- **⚡ 服务调用响应** - 模拟真实设备的服务处理逻辑  
- **🎪 事件条件触发** - 基于属性值条件的自动事件触发

### 🌐 企业级连接
- **🔐 一机一密认证** - 支持阿里云IoT标准认证
- **🌍 稳定MQTT连接** - 自动重连和错误恢复机制
- **📡 OTA升级支持** - 集成固件升级模拟
- **📊 实时监控** - 详细的设备状态和性能统计

## 🚀 10秒上手

### 1. 安装依赖
```bash
go mod download
```

### 2. 启动多设备模拟 ⭐ 推荐方式
```bash
# 一键启动所有设备（自动发现device1, device2等目录）
go run . -mode=simple
```

**输出示例**：
```
2025/08/20 15:58:15 启动简化多设备模式...
2025/08/20 15:58:15 发现 2 个设备配置目录: [device1 device2]
2025/08/20 15:58:15 设备进程[device1]启动成功，PID: 15421
2025/08/20 15:58:15 设备进程[device2]启动成功，PID: 15422
[device1] Connected to MQTT broker: tcp://121.40.253.229:1883
[device2] Connected to MQTT broker: tcp://121.40.253.229:1883
[device1] [智能调速电机] 设备已连接到IoT平台
[device2] [智能空调] 设备已连接到IoT平台
```

### 3. 添加新设备（3种方式）

#### 方式1: 从模板创建 ⭐ 推荐
```bash
# 从空调模板创建device3
go run cmd/generate_rule/main.go -template air_conditioner -device 3 \
  -product-key YEclvKPu -device-name myAC001 -device-secret yourSecret

# 立即启动（自动发现新设备）
go run . -mode=simple
```

#### 方式2: 从TSL物模型生成
```bash
# 从物模型生成device4目录
go run cmd/generate_rule/main.go -tsl 智能空调物模型.json -device 4 \
  -product-key YEclvKPu -device-name myAC002 -device-secret yourSecret
```

#### 方式3: 手动创建
```bash
# 复制现有设备目录
cp -r configs/device1 configs/device5
# 修改configs/device5/config.json中的设备信息
```

### 4. 传统单设备模式（兼容性支持）
```bash
# 智能调速电机
go run . -mode simulator -product 电机 -config config_smart_motor.json

# 简单传感器
go run . -mode sensor
```

## 📂 简化设备目录结构

### 设备目录组织 ⭐ 核心架构
```
configs/
├── device1/              # 智能调速电机
│   ├── config.json       # 设备配置（三元组+MQTT配置）
│   ├── tsl.json          # 物模型定义
│   └── rule.json         # 模拟规则
├── device2/              # 智能空调
│   ├── config.json
│   ├── tsl.json
│   └── rule.json
├── device3/              # 新设备（自动发现）
│   └── ...
└── device_templates/     # 设备模板库
    ├── air_conditioner/  # 空调模板
    └── motor/           # 电机模板
```

### 设备配置文件 (configs/device1/config.json)
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
    "AutoReconnect": true,
    "ReconnectMax": 10,
    "Timeout": 30000000000
  },
  "Features": {
    "EnableOTA": true,
    "EnableShadow": false,
    "EnableRules": false,
    "EnableMetrics": true
  },
  "Logging": {
    "Level": "info",
    "Format": "text",
    "Output": "stdout"
  },
  "Advanced": {
    "WorkerCount": 10,
    "EventBufferSize": 1000,
    "RequestTimeout": 30000000000
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

## ⚙️ 命令行参考

### 主程序运行模式
```bash
go run . [选项]

运行模式:
  -mode=simple         # 简化多设备模式 ⭐ 推荐
  -mode=sensor         # 简单传感器模式  
  -mode=simulator      # 单设备TSL模拟器模式
  -mode=multi          # 传统多设备管理器模式
  -mode=process        # 多进程管理器模式

简化模式选项:
  -device-path string  # 设备配置目录路径 (默认 "configs")
  -web bool           # 是否启用Web管理界面 (默认 true)

传统模式选项:
  -product string     # 产品类型（TSL模拟器模式必需）
  -tsl string         # TSL文件路径（可选）
  -rule string        # 规则文件路径（可选）  
  -config string      # 配置文件路径 (默认 "config.json")
```

### 设备生成工具
```bash
go run cmd/generate_rule/main.go [选项]

生成模式:
  # 从模板创建设备 ⭐ 推荐
  -template <名称> -device <编号> -product-key <key> -device-name <name> -device-secret <secret>
  
  # 从TSL生成设备
  -tsl <文件路径> -device <编号> [三元组选项]
  
  # 传统模式
  -tsl <文件路径> 或 -content '<TSL内容>'

可用模板:
  air_conditioner     # 智能空调模板
  motor              # 智能调速电机模板

示例:
  # 从空调模板创建device3
  go run cmd/generate_rule/main.go -template air_conditioner -device 3 \
    -product-key YEclvKPu -device-name myAC -device-secret yourSecret
```

## 📁 项目架构

```
iot-uplink-gen/
├── 📂 configs/                 # 设备配置中心
│   ├── device1/               # 智能调速电机
│   │   ├── config.json        # 设备配置
│   │   ├── tsl.json          # 物模型
│   │   └── rule.json         # 模拟规则
│   ├── device2/               # 智能空调
│   ├── device_templates/      # 设备模板库
│   │   ├── air_conditioner/   # 空调模板
│   │   └── motor/            # 电机模板
│   └── backup/               # 备份文件
├── 🔧 cmd/
│   └── generate_rule/         # 设备生成工具
├── ⚙️ 核心模块/
│   ├── config/               # 配置管理
│   ├── simulator/            # 模拟引擎
│   │   ├── device_factory.go # 设备工厂
│   │   ├── simulated_device.go # 模拟设备
│   │   ├── property_simulator.go # 属性模拟器
│   │   ├── event_simulator.go # 事件模拟器
│   │   └── service_simulator.go # 服务模拟器
│   ├── tsl/                 # TSL模型管理
│   ├── llm/                 # AI规则生成
│   ├── manager/             # 多设备管理器
│   ├── process/             # 进程管理器
│   └── web/                 # Web管理界面 🔜 即将推出
├── 📋 文档/
│   ├── README.md            # 项目说明
│   └── TODO.md              # 开发计划
└── main.go                  # 程序入口
```

## 🎯 预置设备类型

### 📱 当前可用设备

| 设备类型 | 模板名称 | 特性属性 | 服务功能 | 事件告警 |
|---------|----------|----------|----------|----------|
| **🔧 智能调速电机** | `motor` | 转速、温度、电压、电流、功率、扭矩、振动、效率、频率、运行时间 | 启动/停止电机、调整转速 | 温度过高告警 |  
| **❄️ 智能空调** | `air_conditioner` | 当前/目标温度、湿度、风扇转速、电源状态、工作模式、滤网状态、能耗、压缩机状态 | 设置温度、开关电源、切换模式 | 滤网更换提醒 |

### 🔮 设备扩展能力

- **🏗️ 模板化扩展** - 基于现有模板快速创建新设备类型
- **📱 TSL驱动** - 支持任意符合阿里云IoT规范的物模型  
- **🤖 AI生成** - 使用LLM从物模型自动生成模拟规则
- **🔧 自定义规则** - 手动编写复杂的设备行为模拟逻辑

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

## ✅ 验证测试结果

### 🏆 简化多设备架构验证 - 2025/08/20

#### ✅ **并发多设备测试通过**
- **🎯 设备发现**: 自动识别device1、device2目录，支持动态扩展到deviceX
- **⚡ 并行启动**: 两种不同设备类型同时启动在独立进程中
- **🔗 网络连接**: 全部设备成功连接到IoT平台 `tcp://121.40.253.229:1883`
- **🔐 认证验证**: 一机一密认证成功，使用各自的真实三元组
- **📡 数据上报**: 所有设备按照各自的TSL和规则正常上报数据

#### 📊 **性能指标**
```
设备1 (智能调速电机): FuWtDWoy.AzEYXBjJY5 ✅
├── 属性上报: 10个工业属性 (转速、温度、电压等)
├── 服务注册: 3个控制服务 (启动、停止、调速) 
├── 事件监控: 温度过高告警
└── 上报间隔: 30秒

设备2 (智能空调): YEclvKPu.v4qfzMMLwj ✅  
├── 属性上报: 10个环控属性 (温度、湿度、风速等)
├── 服务注册: 3个控制服务 (温度设定、电源、模式)
├── 事件监控: 滤网更换提醒
└── 上报间隔: 45秒
```

#### 🎛️ **架构优势验证**
- ✅ **即插即用**: 添加device3自动被发现和启动
- ✅ **配置隔离**: 每设备独立配置，互不影响
- ✅ **故障隔离**: 单设备崩溃不影响其他设备运行  
- ✅ **扩展性**: 从模板快速生成新设备，无需修改代码

### 📱 设备数据上报示例

**智能调速电机实时数据**:
```json
{
  "id": "1755575693", "version": "1.0",
  "params": {
    "speed": {"value": "903", "time": 1755575693},
    "temperature": {"value": "54", "time": 1755575693}, 
    "voltage": {"value": "246", "time": 1755575693},
    "power": {"value": "6867", "time": 1755575693},
    "efficiency": {"value": "86", "time": 1755575693}
  }
}
```

**智能空调实时数据**:
```json
{
  "id": "1755575694", "version": "1.0", 
  "params": {
    "current_temperature": {"value": "23.5", "time": 1755575694},
    "target_temperature": {"value": "26", "time": 1755575694},
    "humidity": {"value": "65", "time": 1755575694},
    "power_status": {"value": "1", "time": 1755575694},
    "fan_speed": {"value": "2", "time": 1755575694}
  }
}
```

## 🔮 下一步发展

### 🌐 Web管理界面 (开发中)
- **📊 实时设备监控** - 可视化展示所有设备状态和数据
- **🎛️ 设备远程控制** - 通过Web界面调用设备服务  
- **⚙️ 配置在线编辑** - 在线修改设备配置和模拟规则
- **📈 数据可视化** - 设备属性值的实时图表和历史趋势
- **🔧 设备模板管理** - 可视化创建和编辑设备模板
- **🚀 一键部署** - 快速创建和启动新设备

### 🚀 特性规划
- **🐳 Docker化部署** - 容器化运行，简化环境配置
- **☁️ 云原生支持** - 支持Kubernetes集群部署
- **📡 更多IoT平台** - 支持华为云、腾讯云等其他IoT平台
- **🤖 AI规则优化** - 基于真实设备数据训练的智能模拟
- **📊 性能分析** - 设备模拟性能监控和优化建议

## 🛠️ 故障排除

### 常见问题

| 问题 | 原因 | 解决方案 |
|------|------|----------|
| 🔌 连接失败 | 网络或认证错误 | 检查三元组信息和网络连接 |
| 📁 设备未发现 | 目录结构错误 | 确保deviceX目录包含完整的三个配置文件 |
| 🔧 TSL验证失败 | 数据类型不兼容 | 使用`int`替代`int64/int32` |
| ⚙️ 配置格式错误 | JSON格式问题 | 验证配置文件JSON格式正确性 |

### 调试技巧

**开启详细日志**:
```json
{
  "Logging": {
    "Level": "debug",
    "Output": "stdout"
  }
}
```

**测试设备连接**:
```bash
# 测试单个设备
go run . -mode simulator -config configs/device1/config.json \
  -tsl configs/device1/tsl.json -rule configs/device1/rule.json
```

**验证配置文件**:
```bash
# 验证JSON格式
python -m json.tool configs/device1/config.json
```

## 🤝 参与贡献

我们欢迎各种形式的贡献！

### 🎯 贡献方式
- 🐛 **报告Bug** - 在Issues中提交问题报告
- ✨ **功能建议** - 分享你的想法和需求
- 🔧 **代码贡献** - 提交Pull Request改进代码
- 📖 **文档完善** - 帮助改进文档和示例
- 🧪 **测试反馈** - 分享不同环境下的测试结果

### 🏷️ 项目标签
`iot` `mqtt` `simulator` `golang` `aliyun` `device-simulation` `tsl` `multi-device`

## 📜 开源许可

本项目采用 **MIT License** 开源协议。

---

<div align="center">

**🚀 让IoT设备仿真变得简单高效！**

[⭐ Star](https://github.com/your-org/iot-uplink-gen) · [🐛 Issues](https://github.com/your-org/iot-uplink-gen/issues) · [📖 Wiki](https://github.com/your-org/iot-uplink-gen/wiki)

</div>