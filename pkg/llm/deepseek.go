package llm

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

type DeepSeekClient struct {
	model  string
	apiKey string
}

func NewDeepSeekClient(model, apiKey string) (*DeepSeekClient, error) {
	return &DeepSeekClient{
		model:  model,
		apiKey: apiKey}, nil
}

func (g *DeepSeekClient) Analyze(ctx context.Context, namespace, podName, logs string) (string, error) {
	cm, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey:             g.apiKey,
		Model:              g.model,
		MaxTokens:          2000,
		BaseURL:            "https://api.deepseek.com/beta",
		ResponseFormatType: deepseek.ResponseFormatTypeJSONObject,
	})
	if err != nil {
		zap.L().Error("NewChatModel of deepseek failed", zap.Error(err))
		return "", err
	}
	chain := compose.NewChain[map[string]any, *schema.Message]()
	chain.
		AppendChatTemplate(KubernetesLogAnalyzeSystemPrompt).
		AppendChatModel(cm)

	runnable, err := chain.Compile(ctx)
	if err != nil {
		zap.L().Error("Compile chain failed", zap.Error(err))
		return "", err
	}

	input := map[string]any{
		"logs": logs,
	}
	result, err := runnable.Invoke(ctx, input)
	if err != nil {
		zap.L().Error("Invoke chain failed", zap.Error(err))
		return "", err
	}
	return result.Content, nil
}
