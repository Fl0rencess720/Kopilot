package multiagent

import (
	"context"

	"github.com/Fl0rencess720/Kopilot/pkg/llm/tools"
	"github.com/cloudwego/eino/components/model"
	"k8s.io/client-go/dynamic"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
)

func newAutoFixerAgent(ctx context.Context, cm model.ToolCallingChatModel, dynamicClient dynamic.Interface) (compose.AnyGraph, []compose.GraphAddNodeOpt, error) {
	kubectlPatchTool, err := tools.CreateKubectlPatchTool(dynamicClient)
	if err != nil {
		return nil, nil, err
	}
	ragent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: cm,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools: []tool.BaseTool{kubectlPatchTool},
		},
	})
	g, opt := ragent.ExportGraph()
	if err != nil {
		return nil, nil, err
	}
	return g, opt, nil
}
