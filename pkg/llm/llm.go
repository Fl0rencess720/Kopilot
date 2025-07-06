package llm

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
	"google.golang.org/genai"
)

type LLM interface {
	Analyze()
}

type GeminiClient struct {
	model    string
	thinking bool
	Client   *genai.Client
}

func NewGeminiClient(model, apiKey string, thinking bool) (*GeminiClient, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		zap.L().Error("NewClient of gemini failed", zap.Error(err))
		return nil, err
	}
	return &GeminiClient{
		model:    model,
		Client:   client,
		thinking: thinking,
	}, nil
}

func (g *GeminiClient) Analyze(ctx context.Context, namespace, podName, logs string) (string, error) {
	cm, err := gemini.NewChatModel(ctx, &gemini.Config{
		Client: g.Client,
		Model:  g.model,
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: g.thinking,
			ThinkingBudget:  nil,
		},
		ResponseSchema: KubernetesLogAnalyzeResponseSchema,
	})
	if err != nil {
		zap.L().Error("NewChatModel of gemini failed", zap.Error(err))
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
