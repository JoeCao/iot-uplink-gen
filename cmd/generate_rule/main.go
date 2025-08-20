package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"znb/iot-uplink-gen/llm"
)

func main() {
	var tslPath = flag.String("tsl", "", "TSL文件路径（必需）")
	var tslContent = flag.String("content", "", "TSL文件内容（可选，优先于文件路径）")
	var deviceNum = flag.Int("device", 0, "设备编号（生成deviceX目录，可选）")
	var templateName = flag.String("template", "", "使用模板名称（air_conditioner/motor/等）")
	var productKey = flag.String("product-key", "", "设备ProductKey（生成配置文件时需要）")
	var deviceName = flag.String("device-name", "", "设备DeviceName（生成配置文件时需要）")
	var deviceSecret = flag.String("device-secret", "", "设备DeviceSecret（生成配置文件时需要）")
	flag.Parse()

	// 检查参数
	if *tslContent == "" && *tslPath == "" && *templateName == "" {
		printUsage()
		os.Exit(1)
	}

	// 模式1: 从模板创建设备目录
	if *templateName != "" && *deviceNum > 0 {
		if err := createDeviceFromTemplate(*templateName, *deviceNum, *productKey, *deviceName, *deviceSecret); err != nil {
			fmt.Printf("从模板创建设备失败: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// 模式2: 从TSL生成设备目录
	var tslText string
	var err error

	// 优先使用内容参数
	if *tslContent != "" {
		tslText = *tslContent
		fmt.Println("正在处理TSL内容...")
	} else {
		// 读取TSL文件
		var data []byte
		data, err = ioutil.ReadFile(*tslPath)
		if err != nil {
			fmt.Printf("读取TSL文件失败: %v\n", err)
			os.Exit(1)
		}
		tslText = string(data)
		fmt.Printf("正在处理TSL文件: %s\n", *tslPath)
	}

	// 根据设备编号生成到设备目录或configs目录
	if *deviceNum > 0 {
		if err := generateDeviceDirectory(tslText, *deviceNum, *productKey, *deviceName, *deviceSecret); err != nil {
			fmt.Printf("生成设备目录失败: %v\n", err)
			os.Exit(1)
		}
	} else {
		// 传统模式：生成到configs目录
		result, err := llm.ProcessTSLContent(tslText)
		if err != nil {
			fmt.Printf("处理TSL失败: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("处理完成:\n")
		fmt.Printf("  产品名称: %s\n", result.ProductName)
		fmt.Printf("  TSL文件已保存: %s\n", result.TSLFile)
		fmt.Printf("  Rule文件已保存: %s\n", result.RuleFile)
	}
}

func printUsage() {
	fmt.Println("IoT 设备规则生成工具")
	fmt.Println()
	fmt.Println("使用方法:")
	fmt.Println("  1. 从TSL生成设备目录:")
	fmt.Println("     go run cmd/generate_rule/main.go -tsl <TSL文件路径> -device <设备编号> [选项]")
	fmt.Println("     go run cmd/generate_rule/main.go -content '<TSL JSON内容>' -device <设备编号> [选项]")
	fmt.Println()
	fmt.Println("  2. 从模板创建设备:")
	fmt.Println("     go run cmd/generate_rule/main.go -template <模板名> -device <设备编号> [选项]")
	fmt.Println()
	fmt.Println("  3. 传统模式（生成到configs目录）:")
	fmt.Println("     go run cmd/generate_rule/main.go -tsl <TSL文件路径>")
	fmt.Println("     go run cmd/generate_rule/main.go -content '<TSL JSON内容>'")
	fmt.Println()
	fmt.Println("参数说明:")
	fmt.Println("  -tsl <路径>           TSL文件路径")
	fmt.Println("  -content '<内容>'     TSL JSON内容")
	fmt.Println("  -device <编号>        设备编号（生成deviceX目录）")
	fmt.Println("  -template <名称>      模板名称（air_conditioner, motor等）")
	fmt.Println("  -product-key <key>    设备ProductKey")
	fmt.Println("  -device-name <name>   设备DeviceName")
	fmt.Println("  -device-secret <secret> 设备DeviceSecret")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  # 从TSL生成device3目录")
	fmt.Println("  go run cmd/generate_rule/main.go -tsl 智能空调物模型.json -device 3 -product-key YEclvKPu -device-name myAC001 -device-secret xxxx")
	fmt.Println()
	fmt.Println("  # 从模板创建device4目录")
	fmt.Println("  go run cmd/generate_rule/main.go -template air_conditioner -device 4 -product-key YEclvKPu -device-name myAC002 -device-secret yyyy")
}

func generateDeviceDirectory(tslText string, deviceNum int, productKey, deviceName, deviceSecret string) error {
	deviceDir := filepath.Join("configs", fmt.Sprintf("device%d", deviceNum))
	
	// 检查目录是否存在
	if _, err := os.Stat(deviceDir); err == nil {
		return fmt.Errorf("目录 %s 已存在", deviceDir)
	}
	
	// 创建设备目录
	if err := os.MkdirAll(deviceDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}
	
	fmt.Printf("创建设备目录: %s\n", deviceDir)
	
	// 修复TSL内容中的数据类型问题
	fixedTSLContent := strings.ReplaceAll(tslText, `"type":"int64"`, `"type":"int"`)
	fixedTSLContent = strings.ReplaceAll(fixedTSLContent, `"type":"int32"`, `"type":"int"`)
	
	// 保存TSL文件
	tslFile := filepath.Join(deviceDir, "tsl.json")
	if err := ioutil.WriteFile(tslFile, []byte(fixedTSLContent), 0644); err != nil {
		return fmt.Errorf("保存TSL文件失败: %v", err)
	}
	
	// 生成Rule文件
	ruleContent, err := llm.GenerateDeviceRule(tslText)
	if err != nil {
		return fmt.Errorf("生成Rule失败: %v", err)
	}
	
	// 保存Rule文件
	ruleFile := filepath.Join(deviceDir, "rule.json")
	if err := ioutil.WriteFile(ruleFile, []byte(ruleContent), 0644); err != nil {
		return fmt.Errorf("保存Rule文件失败: %v", err)
	}
	
	// 生成配置文件
	configContent := generateConfigFile(productKey, deviceName, deviceSecret)
	configFile := filepath.Join(deviceDir, "config.json")
	if err := ioutil.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("保存配置文件失败: %v", err)
	}
	
	fmt.Printf("设备目录生成完成:\n")
	fmt.Printf("  设备目录: %s\n", deviceDir)
	fmt.Printf("  TSL文件: %s\n", tslFile)
	fmt.Printf("  Rule文件: %s\n", ruleFile)
	fmt.Printf("  配置文件: %s\n", configFile)
	
	return nil
}

func createDeviceFromTemplate(templateName string, deviceNum int, productKey, deviceName, deviceSecret string) error {
	templateDir := filepath.Join("configs", "device_templates", templateName)
	deviceDir := filepath.Join("configs", fmt.Sprintf("device%d", deviceNum))
	
	// 检查模板是否存在
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		return fmt.Errorf("模板 %s 不存在", templateName)
	}
	
	// 检查设备目录是否存在
	if _, err := os.Stat(deviceDir); err == nil {
		return fmt.Errorf("目录 %s 已存在", deviceDir)
	}
	
	// 创建设备目录
	if err := os.MkdirAll(deviceDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}
	
	fmt.Printf("从模板 %s 创建设备目录: %s\n", templateName, deviceDir)
	
	// 复制TSL和Rule文件
	filesToCopy := []string{"tsl.json", "rule.json"}
	for _, fileName := range filesToCopy {
		srcFile := filepath.Join(templateDir, fileName)
		dstFile := filepath.Join(deviceDir, fileName)
		
		if err := copyFile(srcFile, dstFile); err != nil {
			return fmt.Errorf("复制 %s 失败: %v", fileName, err)
		}
	}
	
	// 生成新的配置文件
	configContent := generateConfigFile(productKey, deviceName, deviceSecret)
	configFile := filepath.Join(deviceDir, "config.json")
	if err := ioutil.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("保存配置文件失败: %v", err)
	}
	
	fmt.Printf("设备目录创建完成:\n")
	fmt.Printf("  设备目录: %s\n", deviceDir)
	fmt.Printf("  模板来源: %s\n", templateName)
	fmt.Printf("  配置文件: %s\n", configFile)
	
	return nil
}

func generateConfigFile(productKey, deviceName, deviceSecret string) string {
	// 如果没有提供三元组信息，使用默认值
	if productKey == "" {
		productKey = "YOUR_PRODUCT_KEY"
	}
	if deviceName == "" {
		deviceName = "YOUR_DEVICE_NAME"
	}
	if deviceSecret == "" {
		deviceSecret = "YOUR_DEVICE_SECRET"
	}
	
	return fmt.Sprintf(`{
  "Device": {
    "ProductKey": "%s",
    "DeviceName": "%s",
    "DeviceSecret": "%s",
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
    "Output": "stdout",
    "MaxSize": 100,
    "MaxBackups": 3,
    "MaxAge": 30
  },
  "Advanced": {
    "WorkerCount": 10,
    "EventBufferSize": 1000,
    "RequestTimeout": 30000000000,
    "PropertyCacheTTL": 300000000000
  }
}`, productKey, deviceName, deviceSecret)
}

func copyFile(src, dst string) error {
	data, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dst, data, 0644)
}