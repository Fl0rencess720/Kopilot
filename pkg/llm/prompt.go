package llm

import (
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
)

var (
	KubernetesLogAnalyzeSystemPrompt = prompt.FromMessages(
		schema.GoTemplate,
		schema.SystemMessage(
			`你是一个Kubernetes运维专家,请根据输入的日志内容评估日志的严重程度。
		对于严重程度高的日志，请给出原因分析和解决方案，并判断是否需要上报。  
		你需要从运维文档找到对于给出日志中所显示的问题的解决方案，若运维文档为空或没有找到合适的解决方案，
		则由你自己给出合适的解决方案。

        以下是返回结果的格式要求：  
  
        返回结果应该仅以JSON格式返回;
        对于每个日志项，返回字段包括：  
            reason: 原因分析  
            solution: 解决方案  
            sink: 是否需要上报,如果需要上报,值为true,否则为false  
        请根据以下示例格式返回结果：  
        {  
        "reason": "error reason",  
        "solution": "error solution",  
        "sink": true  
        }
		请使用{{.lang}}回答
		以下是该Pod的yaml, 日志内容和运维文档:`),
		schema.UserMessage("Pod yaml: {{.pod_yaml}}\n日志内容：{{.logs}}"),
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
