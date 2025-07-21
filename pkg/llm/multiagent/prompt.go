package multiagent

import (
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
)

var (
	HostSystemPrompt = schema.SystemMessage(
		`你是一个k8s集群的负责人,请根据用户输入的信息,从options中选择下一个行动:  
		返回结果应该仅以JSON格式返回;
		对于每个日志项，返回字段包括：  
			option: 下一步行动
			context: 上下文信息 
			autofix: 自动修复失败后,自动修复过程所产生的上下文,需要进行整理后再写入该字段 
			search: 网络搜索结果，需要进行整理后再写入该字段
		请根据以下示例格式返回结果：  
		{  
		"option": "AutoFixer",
		"context": {
			"autofix": "",
			"search": ""
		}
		}	
		或
		{
		"option": "HumanHelper",
		"context": {
			"autofix": "autofix failed,context is...",
			"search": "this is search result..."
		}
		}
		以下是options列表及相应说明:
		列表：
		["AutoFixer","Searcher","HumanHelper","Finish"]
		说明:
		AutoFixer: 进行自动修复,当输入内容仅为日志时请选择此选项,此时context字段必须为空
		Searcher: 网络搜索,当自动修复失败后,请使用此选项,此时context字段必须为空
		HumanHelper:  寻求人类帮助,当自动修复失败且已经进行过网络搜索后,请将自动修复失败所返回的上下文和网络搜索结果整理后写入context字段
		Finish: 任务结束,当你认为问题已经解决时,请使用该选项,例如当自动修复成功或成功寻求人类帮助后,则可以选择Finish
		`)

	HostResponseSchema = &openapi3.Schema{
		Type: "object",
		Properties: map[string]*openapi3.SchemaRef{
			"option": {
				Value: &openapi3.Schema{
					Type: "string",
				},
			},
			"context": {
				Value: &openapi3.Schema{
					Type: "object",
					Properties: map[string]*openapi3.SchemaRef{
						"autofix": {
							Value: &openapi3.Schema{
								Type: "string",
							},
						},
						"search": {
							Value: &openapi3.Schema{
								Type: "string",
							},
						},
					},
				},
			},
		},
		Required: []string{"option", "context"},
	}
)

var (
	AutoFixerSystemPrompt = schema.SystemMessage(
		`你是一个K8s自动修复专家。请分析问题并尝试自动修复。
		如果修复成功，返回'修复成功：[具体修复内容]';
		如果修复失败，返回'修复失败：[失败原因]'。
		`)

	SearcherSystemPrompt = schema.SystemMessage(
		`你是一个网络搜索专家。
		请搜索相关的K8s问题解决方案。
		返回格式：'搜索结果：[相关解决方案]'。
		`)
	HumanHelperSystemPrompt = schema.SystemMessage(
		`你是一个文档撰写专家。
		请基于原始日志及自动修复失败和搜索结果，生成详细的问题文档供人工处理。
		`)
)
