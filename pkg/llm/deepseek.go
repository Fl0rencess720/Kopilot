package llm

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
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

func (c *DeepSeekClient) Analyze(ctx context.Context, pod corev1.Pod, logs string) (string, error) {
	cm, err := c.GetModel(ctx, KubernetesLogAnalyzeResponseSchema)
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

	podYaml, err := yaml.Marshal(pod)
	if err != nil {
		zap.L().Error("Marshal pod to yaml failed", zap.Error(err))
		return "", err
	}

	input := map[string]any{
		"pod_yaml": string(podYaml),
		"logs":     logs,
		"lang":     GetLanguageName(c.language),
	}
	result, err := runnable.Invoke(ctx, input)
	if err != nil {
		zap.L().Error("Invoke chain failed", zap.Error(err))
		return "", err
	}
	return result.Content, nil
}

func (c *DeepSeekClient) GetModel(ctx context.Context, responseSchema *openapi3.Schema) (model.ToolCallingChatModel, error) {
	responseFormatType := deepseek.ResponseFormatTypeText
	if responseSchema != nil {
		responseFormatType = deepseek.ResponseFormatTypeJSONObject
	}
	cm, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey:             c.apiKey,
		Model:              c.model,
		MaxTokens:          2000,
		BaseURL:            "https://api.deepseek.com/beta",
		ResponseFormatType: deepseek.ResponseFormatType(responseFormatType),
	})
	if err != nil {
		return nil, err
	}
	return cm, nil
}
