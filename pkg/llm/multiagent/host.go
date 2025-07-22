package multiagent

import (
	"context"
	"fmt"

	kopilotv1 "github.com/Fl0rencess720/Kopilot/api/v1"
	"github.com/Fl0rencess720/Kopilot/pkg/llm"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type LogMultiAgent struct {
	config   LogMultiAgentConfig
	runnable compose.Runnable[[]*schema.Message, *SinkMessageContent]
}

type LogMultiAgentConfig struct {
	Host          model.ToolCallingChatModel
	Autofixer     model.ToolCallingChatModel
	Searcher      model.ToolCallingChatModel
	Humanhelper   model.ToolCallingChatModel
	Retriever     *llm.HybridRetriever
	dynamicClient dynamic.Interface
	language      string
}

func NewLogMultiAgent(ctx context.Context, clientset kubernetes.Interface, dynamicClient dynamic.Interface, llmSpec kopilotv1.LLMSpec, retriever *llm.HybridRetriever, language string) (*LogMultiAgent, error) {
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
		Host:          host,
		Autofixer:     autofixer,
		Searcher:      searcher,
		Humanhelper:   humanhelper,
		Retriever:     retriever,
		dynamicClient: dynamicClient,
		language:      language,
	}
	runnable, err := buildGraphRunnable(ctx, &config)
	if err != nil {
		return nil, err
	}
	ma := &LogMultiAgent{
		config:   config,
		runnable: runnable,
	}
	return ma, nil
}

func (ma *LogMultiAgent) Run(ctx context.Context, logs string) (string, error) {
	in := []*schema.Message{{
		Content: logs,
	}}
	output, err := ma.runnable.Invoke(ctx, in)
	if err != nil {
		return "", err
	}
	result := ""
	result += fmt.Sprintf("初始日志：%s\n", output.OriginalInput)
	result += fmt.Sprintf("自动修复结果: %s\n", output.AutoFixResult)
	result += fmt.Sprintf("搜索结果: %s\n", output.SearchResult)
	result += fmt.Sprintf("问题文档: %s\n", output.HumanHelpResult)
	return result, nil
}
