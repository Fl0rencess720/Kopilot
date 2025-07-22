package llm

import (
	"context"
	"fmt"

	kopilotv1 "github.com/Fl0rencess720/Kopilot/api/v1"
	"github.com/Fl0rencess720/Kopilot/internal/controller/utils"
	"github.com/cloudwego/eino/components/model"
	"github.com/getkin/kin-openapi/openapi3"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

type LLMClient interface {
	Analyze(ctx context.Context, namespace, podName, logs string) (string, error)
	GetModel(ctx context.Context, responseSchema *openapi3.Schema) (model.ToolCallingChatModel, error)
}

func NewLLMClient(ctx context.Context, clientset kubernetes.Interface, llmSpec kopilotv1.LLMSpec, retriever *HybridRetriever) (LLMClient, error) {
	switch llmSpec.Model {
	case "gemini":
		apikey, err := utils.GetSecret(clientset, llmSpec.Gemini.APIKeySecretRef.Key, "default", llmSpec.Gemini.APIKeySecretRef.Name)
		if err != nil {
			zap.L().Error("unable to get LLM API key", zap.Error(err))
			return nil, err
		}
		return NewGeminiClient(llmSpec.Gemini.ModelName, apikey, llmSpec.Language, llmSpec.Gemini.Thinking, retriever)
	case "deepseek":
		apikey, err := utils.GetSecret(clientset, llmSpec.DeepSeek.APIKeySecretRef.Key, "default", llmSpec.DeepSeek.APIKeySecretRef.Name)
		if err != nil {
			zap.L().Error("unable to get LLM API key", zap.Error(err))
			return nil, err
		}
		return NewDeepSeekClient(llmSpec.DeepSeek.ModelName, apikey, llmSpec.Language, retriever)
	default:
		return nil, fmt.Errorf("unsupported LLM model: %s", llmSpec.Model)
	}
}
