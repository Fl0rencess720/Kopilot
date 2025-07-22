package multiagent

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
)

func newHumanHelperAgent(ctx context.Context, cm model.ToolCallingChatModel) (compose.AnyGraph, []compose.GraphAddNodeOpt, error) {
	humanHelperAgent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: cm,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools: []tool.BaseTool{},
		},
	})
	if err != nil {
		return nil, nil, err
	}
	g, opt := humanHelperAgent.ExportGraph()
	return g, opt, nil
}
