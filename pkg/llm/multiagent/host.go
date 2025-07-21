package multiagent

import (
	"context"

	kopilotv1 "github.com/Fl0rencess720/Kopilot/api/v1"
	"github.com/Fl0rencess720/Kopilot/pkg/llm"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"k8s.io/client-go/kubernetes"
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
	Retriever   *llm.HybridRetriever
}

func NewLogMultiAgent(ctx context.Context, clientset kubernetes.Interface, llmSpec kopilotv1.LLMSpec, retriever *llm.HybridRetriever) (*LogMultiAgent, error) {
	maLLM, err := llm.NewLLMClient(ctx, clientset, llmSpec, nil)
	if err != nil {
		return nil, err
	}
	host, err := maLLM.GetModel(ctx, HostResponseSchema)
	if err != nil {
		return nil, err
	}
	autofixer, err := maLLM.GetModel(ctx, nil)
	if err != nil {
		return nil, err
	}
	searcher, err := maLLM.GetModel(ctx, nil)
	if err != nil {
		return nil, err
	}
	humanhelper, err := maLLM.GetModel(ctx, nil)
	if err != nil {
		return nil, err
	}
	config := LogMultiAgentConfig{
		Host:        host,
		Autofixer:   autofixer,
		Searcher:    searcher,
		Humanhelper: humanhelper,
		Retriever:   retriever,
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

func (ma *LogMultiAgent) Run(ctx context.Context, logs string) error {
	in := []*schema.Message{{
		Content: logs,
	}}
	_, err := ma.runnable.Invoke(ctx, in)
	if err != nil {
		return err
	}
	return nil
}
