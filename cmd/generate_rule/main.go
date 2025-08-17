package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"znb/iot-uplink-gen/llm"
)

func main() {
	var tslPath = flag.String("tsl", "", "TSL文件路径（必需）")
	var tslContent = flag.String("content", "", "TSL文件内容（可选，优先于文件路径）")
	flag.Parse()

	if *tslContent == "" && *tslPath == "" {
		fmt.Println("错误: 必须指定TSL文件路径或TSL内容")
		fmt.Println("使用方法:")
		fmt.Println("  通过文件: go run cmd/generate_rule/main.go -tsl <TSL文件路径>")
		fmt.Println("  通过内容: go run cmd/generate_rule/main.go -content '<TSL JSON内容>'")
		fmt.Println("示例:")
		fmt.Println("  go run cmd/generate_rule/main.go -tsl configs/面包房设备物模型.json")
		os.Exit(1)
	}

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

	// 使用统一的TSL处理流程
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