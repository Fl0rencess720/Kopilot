package llm

import (
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
)

var (
	KubernetesLogAnalyzeSystemPrompt = prompt.FromMessages(
		schema.GoTemplate,
		schema.SystemMessage(`你是一个Kubernetes运维专家，请根据以下日志内容评估日志的严重程度。对于严重程度高的日志，请给出原因分析和解决方案，并判断是否需要上报。  
        以下是返回结果的格式要求：  
  
        返回结果应该仅以JSON格式返回；  
        对于每个日志项，返回字段包括：  
            reason: 原因分析  
            solution: 解决方案  
            sink: 是否需要上报，如果需要上报，值为true，否则为false  
        请根据以下示例格式返回结果：  
        {  
        "reason": "error reason",  
        "solution": "error solution",  
        "sink": true  
        }`),
		schema.UserMessage("日志内容：{{.logs}}"),
	)

	KubernetesLogAnalyzeResponseSchema = &openapi3.Schema{
		Type: "object",
		Properties: map[string]*openapi3.SchemaRef{
			"reason": {
				Value: &openapi3.Schema{
					Type: "string",
				},
			},
			"solution": {
				Value: &openapi3.Schema{
					Type: "string",
				},
			},
			"sink": {
				Value: &openapi3.Schema{
					Type: "boolean",
				},
			},
		},
		Required: []string{"reason", "solution", "sink"},
	}
)
