package multiagent

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type LogMultiAgent struct {
	config   LogMultiAgentConfig
	runnable compose.Runnable[[]*schema.Message, *schema.Message]
}

type LogMultiAgentConfig struct {
	Host        model.ToolCallingChatModel
	Autofixer   model.ToolCallingChatModel
	Searcher    model.ToolCallingChatModel
	Humanhelper model.ToolCallingChatModel
}

func NewLogMultiAgent(host model.ToolCallingChatModel, autofixer model.ToolCallingChatModel, searcher model.ToolCallingChatModel, humanhelper model.ToolCallingChatModel) (*LogMultiAgent, error) {
	ctx := context.Background()
	config := LogMultiAgentConfig{
		Host:        host,
		Autofixer:   autofixer,
		Searcher:    searcher,
		Humanhelper: humanhelper,
	}
	runnable, err := newGraphRunnable(ctx, &config)
	if err != nil {
		return nil, err
	}
	ma := &LogMultiAgent{
		config:   config,
		runnable: runnable,
	}
	return ma, nil
}
