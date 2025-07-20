package llm

import (
	"context"
	"errors"
	"sync"

	kopilotv1 "github.com/Fl0rencess720/Kopilot/api/v1"
	"github.com/Fl0rencess720/Kopilot/internal/controller/utils"
	arkembedding "github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float64, error)
}

type ArkEmbedder struct {
	embedder *arkembedding.Embedder
}

type HybridRetriever struct {
	client         *milvusclient.Client
	embedder       Embedder
	topK           int
	collectionName string
}

func NewMilvusClient(ctx context.Context, clientset kubernetes.Interface, knowledgeBase kopilotv1.KnowledgeBaseSpec) (*milvusclient.Client, error) {

	username, err := utils.GetSecret(clientset, knowledgeBase.UsernameSecretRef.Key, knowledgeBase.UsernameSecretRef.Namespace, knowledgeBase.UsernameSecretRef.Name)
	if err != nil {
		username = ""
	}

	password, err := utils.GetSecret(clientset, knowledgeBase.PasswordSecretRef.Key, knowledgeBase.PasswordSecretRef.Namespace, knowledgeBase.PasswordSecretRef.Name)
	if err != nil {
		password = ""
	}

	client, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address:  knowledgeBase.Address,
		Username: username,
		Password: password,
	})
	if err != nil {
		zap.L().Error("failed to create Milvus client", zap.Error(err))
		return nil, err
	}
	return client, nil
}

func NewHybridRetriever(client *milvusclient.Client, embedder Embedder, knowledgeBase kopilotv1.KnowledgeBaseSpec) (*HybridRetriever, error) {
	hr := &HybridRetriever{
		client:         client,
		embedder:       embedder,
		topK:           knowledgeBase.TopK,
		collectionName: knowledgeBase.CollectionName,
	}
	if err := hr.LoadMilvus(context.Background()); err != nil {
		return nil, err
	}
	return hr, nil
}

func NewEmbedder(ctx context.Context, clientset kubernetes.Interface, knowledgeBaseSpec kopilotv1.KnowledgeBaseSpec) (Embedder, error) {
	switch knowledgeBaseSpec.EmbeddingProvider {
	case "ark":
		apikey, err := utils.GetSecret(clientset, knowledgeBaseSpec.ArkSpec.APIKeySecretRef.Key, "default", knowledgeBaseSpec.ArkSpec.APIKeySecretRef.Name)
		if err != nil {
			zap.L().Error("unable to get LLM API key", zap.Error(err))
			return nil, err
		}
		return newArkEmbedder(ctx, knowledgeBaseSpec.ArkSpec.ModelName, apikey)
	default:
		return nil, errors.New("unknown embedding provider")
	}
}

func newArkEmbedder(ctx context.Context, modelName, apiKey string) (Embedder, error) {
	embedder, err := arkembedding.NewEmbedder(ctx, &arkembedding.EmbeddingConfig{
		APIKey:  apiKey,
		Model:   modelName,
		BaseURL: "https://ark.cn-beijing.volces.com/api/v3",
		Region:  "cn-beijing",
	})
	if err != nil {
		panic("failed to create embedder")
	}
	return &ArkEmbedder{
		embedder: embedder,
	}, nil
}

func (e *ArkEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	v2, err := e.embedder.EmbedStrings(ctx, []string{text})
	if err != nil {
		zap.L().Error("failed to embed text", zap.Error(err))
		return nil, err
	}
	if len(v2) == 0 || len(v2[0]) == 0 {
		return nil, errors.New("embedding result is empty")
	}
	return v2[0], nil
}
func (hr *HybridRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	denseQueryVector64, err := hr.embedder.Embed(ctx, query)
	if err != nil {
		zap.L().Error("failed to embed query", zap.Error(err))
		return nil, err
	}
	denseQuery := convertFloat64ToFloat32([][]float64{denseQueryVector64})[0]
	denseReq := milvusclient.NewAnnRequest("dense", 5, entity.FloatVector(denseQuery)).
		WithAnnParam(index.NewIvfAnnParam(10)).
		WithSearchParam(index.MetricTypeKey, "COSINE")
	annParam := index.NewSparseAnnParam()
	annParam.WithDropRatio(0.2)
	sparseReq := milvusclient.NewAnnRequest("sparse", 5, entity.Text(query)).
		WithAnnParam(annParam).
		WithSearchParam(index.MetricTypeKey, "BM25")
	reranker := milvusclient.NewWeightedReranker([]float64{0.8, 0.3})
	resultSets, err := hr.client.HybridSearch(ctx, milvusclient.NewHybridSearchOption(
		hr.collectionName,
		hr.topK,
		denseReq,
		sparseReq,
	).WithReranker(reranker).WithOutputFields("text"))
	if err != nil {
		zap.L().Error("HybridSearch failed", zap.Error(err))
		return nil, err
	}
	docs := make([]*schema.Document, len(resultSets))
	for i, resultSet := range resultSets {
		text, err := resultSet.GetColumn("text").GetAsString(i)
		if err != nil {
			return nil, err
		}
		docs = append(docs, &schema.Document{
			ID:      resultSet.IDs.FieldData().String(),
			Content: text,
		})
	}
	return docs, nil
}

func (hr *HybridRetriever) LoadMilvus(ctx context.Context) error {
	loadTask, err := hr.client.LoadCollection(ctx, milvusclient.NewLoadCollectionOption("hybrid_test_collection"))
	if err != nil {
		return err
	}
	if err := loadTask.Await(ctx); err != nil {
		return err
	}
	return nil
}

func convertFloat64ToFloat32(input [][]float64) [][]float32 {
	result := make([][]float32, len(input))
	var wg sync.WaitGroup

	for i, row := range input {
		wg.Add(1)
		go func(i int, row []float64) {
			defer wg.Done()
			result[i] = make([]float32, len(row))
			for j, val := range row {
				result[i][j] = float32(val)
			}
		}(i, row)
	}

	wg.Wait()
	return result
}
