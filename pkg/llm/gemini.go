package llm

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
	"go.uber.org/zap"
	"google.golang.org/genai"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

type GeminiClient struct {
	model     string
	thinking  bool
	client    *genai.Client
	language  string
	retriever *HybridRetriever
}

func NewGeminiClient(model, apiKey, language string, thinking bool, retriever *HybridRetriever) (*GeminiClient, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		zap.L().Error("NewClient of gemini failed", zap.Error(err))
		return nil, err
	}
	return &GeminiClient{
		model:     model,
		client:    client,
		thinking:  thinking,
		language:  language,
		retriever: retriever,
	}, nil
}

func (c *GeminiClient) Analyze(ctx context.Context, pod corev1.Pod, logs string) (string, error) {
	cm, err := c.GetModel(ctx, KubernetesLogAnalyzeResponseSchema)
	if err != nil {
		zap.L().Error("NewChatModel of gemini failed", zap.Error(err))
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

func (c *GeminiClient) GetModel(ctx context.Context, responseSchema *openapi3.Schema) (model.ToolCallingChatModel, error) {
	cm, err := gemini.NewChatModel(ctx, &gemini.Config{
		Client: c.client,
		Model:  c.model,
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: c.thinking,
			ThinkingBudget:  nil,
		},
		ResponseSchema: responseSchema,
	})
	if err != nil {
		return nil, err
	}
	return cm, nil
}
