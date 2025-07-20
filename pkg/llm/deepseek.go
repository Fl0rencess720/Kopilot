package llm

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

type DeepSeekClient struct {
	model     string
	apiKey    string
	language  string
	retriever *HybridRetriever
}

func NewDeepSeekClient(model, apiKey, language string, retriever *HybridRetriever) (*DeepSeekClient, error) {
	return &DeepSeekClient{
		model:    model,
		apiKey:   apiKey,
		language: language,
	}, nil
}

func (c *DeepSeekClient) Analyze(ctx context.Context, namespace, podName, logs string) (string, error) {
	cm, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey:             c.apiKey,
		Model:              c.model,
		MaxTokens:          2000,
		BaseURL:            "https://api.deepseek.com/beta",
		ResponseFormatType: deepseek.ResponseFormatTypeJSONObject,
	})
	if err != nil {
		zap.L().Error("NewChatModel of deepseek failed", zap.Error(err))
		return "", err
	}

	var runnable compose.Runnable[map[string]any, *schema.Message]

	if c.retriever != nil {
		runnable, err = newRunnableWithRetriever(ctx, cm, c.retriever)
		if err != nil {
			zap.L().Error("newChainWithRetriever failed", zap.Error(err))
			return "", err
		}
	} else {
		runnable, err = newRunnable(ctx, cm)
		if err != nil {
			zap.L().Error("newChain failed", zap.Error(err))
			return "", err
		}
	}

	input := map[string]any{
		"logs": logs,
		"lang": c.language,
	}
	result, err := runnable.Invoke(ctx, input)
	if err != nil {
		zap.L().Error("Invoke chain failed", zap.Error(err))
		return "", err
	}
	return result.Content, nil
}
