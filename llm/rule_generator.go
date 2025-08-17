package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/spf13/viper"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

var volcAPIKey string
var volcModelID string
var deepseekAPIKey string

func init() {
	// 设置配置文件
	viper.SetConfigName("llm")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("configs")

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// 如果配置文件不存在，使用默认值
			volcAPIKey = "a998008e-575a-46c5-a1df-5f52de136865"
			volcModelID = "deepseek-v3-241226"
			deepseekAPIKey = "sk-a35fc4754186433d97a0d265db710e26"
		} else {
			fmt.Printf("读取配置文件错误: %v\n", err)
		}
		return
	}

	// 根据配置设置API密钥
	volcAPIKey = viper.GetString("llm.volc.api_key")
	volcModelID = viper.GetString("llm.volc.model_id")
	deepseekAPIKey = viper.GetString("llm.deepseek.api_key")
}

// Message 消息结构
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// DeepseekRequest Deepseek API请求结构
type DeepseekRequest struct {
	Messages         []Message   `json:"messages"`
	Model            string      `json:"model"`
	FrequencyPenalty float64     `json:"frequency_penalty"`
	MaxTokens        int         `json:"max_tokens"`
	PresencePenalty  float64     `json:"presence_penalty"`
	ResponseFormat   interface{} `json:"response_format"`
	Stop             interface{} `json:"stop"`
	Stream           bool        `json:"stream"`
	StreamOptions    interface{} `json:"stream_options"`
	Temperature      float64     `json:"temperature"`
	TopP             float64     `json:"top_p"`
	Tools            interface{} `json:"tools"`
	ToolChoice       string      `json:"tool_choice"`
	LogProbs         bool        `json:"logprobs"`
	TopLogProbs      interface{} `json:"top_logprobs"`
}

// DeepseekResponse Deepseek API响应结构
type DeepseekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// GenerateDeviceRule 根据TSL生成设备规则
func GenerateDeviceRule(tslContent string) (string, error) {
	// 根据配置选择使用哪个API
	if viper.GetString("llm.provider") == "deepseek" {
		return generateDeviceRuleByDeepseek(tslContent)
	}
	return generateDeviceRuleByVolc(tslContent)
}

// generateDeviceRuleByVolc 使用火山引擎API生成设备规则
func generateDeviceRuleByVolc(tslContent string) (string, error) {
	client := arkruntime.NewClientWithApiKey(volcAPIKey)
	ctx := context.Background()

	// 使用系统提示词
	systemPrompt := getSystemPrompt()
	userPrompt := fmt.Sprintf("请根据以下TSL文件生成对应的模拟规则。只返回JSON内容，不要包含任何其他文字：\n%s", tslContent)

	req := model.CreateChatCompletionRequest{
		Model: volcModelID,
		Messages: []*model.ChatCompletionMessage{
			{
				Role: model.ChatMessageRoleSystem,
				Content: &model.ChatCompletionMessageContent{
					StringValue: volcengine.String(systemPrompt),
				},
			},
			{
				Role: model.ChatMessageRoleUser,
				Content: &model.ChatCompletionMessageContent{
					StringValue: volcengine.String(userPrompt),
				},
			},
		},
		Temperature: volcengine.Float32(1.0),
		TopP:        volcengine.Float32(1.0),
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("火山引擎API调用失败: %v", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("火山引擎API返回内容为空")
	}

	content := *resp.Choices[0].Message.Content.StringValue

	// 验证返回的内容是否是有效的JSON
	var jsonCheck interface{}
	if err := json.Unmarshal([]byte(content), &jsonCheck); err != nil {
		return "", fmt.Errorf("生成的内容不是有效的JSON: %v, content: %s", err, content)
	}

	return content, nil
}

// generateDeviceRuleByDeepseek 使用Deepseek API生成设备规则
func generateDeviceRuleByDeepseek(tslContent string) (string, error) {
	url := "https://api.deepseek.com/chat/completions"

	systemPrompt := getSystemPrompt()
	userPrompt := fmt.Sprintf("请根据以下TSL文件生成对应的模拟规则。只返回JSON内容，不要包含任何其他文字：\n%s", tslContent)

	request := DeepseekRequest{
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Model:            "deepseek-chat",
		MaxTokens:        8129,
		Temperature:      1,
		TopP:             1,
		FrequencyPenalty: 0,
		PresencePenalty:  0,
		Stream:           false,
		ToolChoice:       "none",
		ResponseFormat:   map[string]string{"type": "json_object"},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("marshal request failed: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request failed: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", deepseekAPIKey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("api request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("api request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response DeepseekResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("unmarshal response failed: %v, body: %s", err, string(body))
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response content")
	}

	content := response.Choices[0].Message.Content

	// 验证返回的内容是否是有效的JSON
	var jsonCheck interface{}
	if err := json.Unmarshal([]byte(content), &jsonCheck); err != nil {
		return "", fmt.Errorf("generated content is not valid JSON: %v, content: %s", err, content)
	}

	return content, nil
}

// getSystemPrompt 获取系统提示词
func getSystemPrompt() string {
	return `你是一个设备模拟规则生成器。你的任务是根据给定的TSL(物模型)文件，生成对应的模拟规则文件。
你必须严格按照以下JSON格式生成规则，并遵循数值范围的建议：

{
  "productName": "产品名称",
  "simulationConfig": {
    "属性标识符": {
      "method": "模拟方法名称",
      // 根据method的不同，需要以下对应的参数
      // 1. 当method为"randomRange"时：
      "min": 最小值,
      "max": 最大值,
      "step": 步长值(可选),

      // 2. 当method为"wave"时：
      "min": 最小值,
      "max": 最大值,
      "amplitude": 振幅值,
      "wavePeriod": 波动周期(秒),

      // 3. 当method为"enumPick"时（用于枚举类型）：
      "enumValues": ["值1", "值2", "值3"],
      "switchProbability": 切换概率(0-1之间),

      // 4. 当method为"accumulate"时（用于计数器类型）：
      "start": 起始值,
      "step": 每次增加的值,

      // 5. 当method为"increase"时（用于持续增长的值）：
      "start": 起始值,
      "step": 每秒增加的值
    }
  },
  "events": [
    {
      "identifier": "事件标识符",
      "triggerCondition": "触发条件表达式",
      "cooldown": 冷却时间(秒)
    }
  ],
  "services": {
    "服务标识符": {
      "responseStrategy": "fixed或randomPick",
      "possibleResponses": [
        {
          "code": 响应码,
          "msg": "响应消息",
          "desc": "响应描述"
        }
      ]
    }
  }
}

对于数值范围和参数的具体要求：

1. randomRange方法（用于连续数值类型）：
   - 如果TSL中定义了min/max，应该选择其中的一个合理子区间
   - 对于温度类型，通常选择正常工作温度范围，比如15-35℃
   - 对于湿度类型，通常在20-80%之间
   - 对于压力类型，根据设备类型选择合适范围
   - step值应该根据精度需求选择，比如温度可以是0.1或0.5，湿度可以是1
   - 选择的区间应该能反映设备的正常工作状态

2. wave方法（用于波动型数值）：
   - min/max定义波动的整体范围
   - amplitude（振幅）应该是合理的波动幅度，通常是(max-min)的10%-20%
   - wavePeriod（周期）建议值：
     * 温度波动：300-600秒
     * 湿度波动：120-300秒
     * 压力波动：60-180秒
     * 其他类型：根据实际情况选择合适的周期

3. enumPick方法（用于枚举类型）：
   - 请关注他原来的在tsl中类型是boolean还是Int32、int64,如果是boolean，则enumValues应该为["true", "false"]，如果是Int32、int64，则enumValues应该为["0", "1"]
   - enumValues应该包含该属性所有可能的枚举值，所有值必须是字符串类型
   - 如果是数字枚举，也要转换成字符串，如：["0", "1", "2"]
   - switchProbability通常设置在0.1-0.3之间，表示每次上报时切换值的概率
   - 对于状态类的枚举，可以设置较低的切换概率（如0.1）
   - 对于工作模式类的枚举，可以设置较高的切换概率（如0.3）
   - 请关注他原来的在tsl中类型是boolean还是Int32、int64,如果是boolean，则enumValues应该为["true", "false"]，如果是Int32、int64，则enumValues应该为["0", "1"]

4. accumulate方法（用于计数器类型）：
   - start通常从0开始
   - step根据实际计数需求设置，比如：
     * 生产计数：1或2（每次增加1或2个）
     * 批次计数：10或100（每次增加一批）
   - 适用于产量统计、工件计数等场景

5. increase方法（用于持续增长的值）：
   - start通常从0开始
   - step需要考虑实际的增长速率，比如：
     * 工作时间：0.00027778（1/3600，每秒增加1/3600小时）
     * 累计用电量：根据功率计算每秒增加值
   - 适用于工作时间、累计值等场景

6. 事件触发：
   - triggerCondition应该设置在正常值的边界附近
   - cooldown通常设置在300-600秒之间，避免事件触发过于频繁
   - 对于严重告警，可以设置较短的cooldown（如60秒）
   - 对于提示性事件，可以设置较长的cooldown（如600秒）

注意：
1. 必须生成合法的JSON格式
2. 数值类型不要带引号
3. 字符串必须用双引号
4. 不要添加任何注释
5. 不要添加任何额外的说明文字
6. 所有数值都应该是合理且实用的
7. 根据属性的实际含义选择合适的模拟方法
8. 模拟参数要符合实际设备的运行特征`
}