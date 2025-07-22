package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// ApplyJSONPatchParams 定义大模型传递的参数
type ApplyJSONPatchParams struct {
	Group     string `json:"group" description:"API group of the resource"`
	Version   string `json:"version" description:"API version of the resource"`
	Kind      string `json:"kind" description:"Kind of the resource"`
	Namespace string `json:"namespace" description:"Namespace of the resource"`
	Name      string `json:"name" description:"Name of the resource"`
	Patch     string `json:"patch" description:"JSON patch operations as string"`
}

// PatchResult 定义返回结果
type PatchResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// KubectlPatchTool 实现 kubectl patch 功能的 tool
type KubectlPatchTool struct {
	dynamicClient dynamic.Interface
}

// NewKubectlPatchTool 创建新的 KubectlPatchTool 实例
func NewKubectlPatchTool(client dynamic.Interface) *KubectlPatchTool {
	return &KubectlPatchTool{
		dynamicClient: client,
	}
}

// ApplyPatch 执行 JSON patch 操作
func (k *KubectlPatchTool) ApplyPatch(ctx context.Context, params *ApplyJSONPatchParams) (*PatchResult, error) {
	// 1. 基本验证：确保传入的 JSON Patch 字符串是合法的 JSON 格式
	var patchOps []interface{}
	if err := json.Unmarshal([]byte(params.Patch), &patchOps); err != nil {
		return &PatchResult{
			Success: false,
			Message: fmt.Sprintf("invalid json patch format: %v", err),
		}, nil
	}
	if len(patchOps) == 0 {
		return &PatchResult{
			Success: false,
			Message: "json patch cannot be empty",
		}, nil
	}

	// 2. 构建 GroupVersionKind
	gvk := schema.GroupVersionKind{
		Group:   params.Group,
		Version: params.Version,
		Kind:    params.Kind,
	}

	// 3. 获取对应资源的动态客户端接口
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	resourceClient := k.dynamicClient.Resource(gvr).Namespace(params.Namespace)

	// 4. 执行 Patch 操作
	zap.L().Info("Applying JSON patch",
		zap.String("GVK", gvk.String()),
		zap.String("Namespace", params.Namespace),
		zap.String("Name", params.Name),
		zap.String("Patch", params.Patch))

	_, err := resourceClient.Patch(
		ctx,
		params.Name,
		types.JSONPatchType,
		[]byte(params.Patch),
		metav1.PatchOptions{},
	)

	if err != nil {
		return &PatchResult{
			Success: false,
			Message: fmt.Sprintf("failed to apply patch for %s/%s: %v", params.Namespace, params.Name, err),
		}, nil
	}

	zap.L().Info("JSON patch applied successfully",
		zap.String("GVK", gvk.String()),
		zap.String("Namespace", params.Namespace),
		zap.String("Name", params.Name))

	return &PatchResult{
		Success: true,
		Message: fmt.Sprintf("Successfully applied patch to %s/%s in namespace %s", params.Kind, params.Name, params.Namespace),
	}, nil
}

func CreateKubectlPatchTool(dynamicClient dynamic.Interface) (tool.InvokableTool, error) {
	kubectlTool := NewKubectlPatchTool(dynamicClient)

	return utils.InferTool(
		"kubectl_patch",
		"Apply JSON patch to Kubernetes resources",
		kubectlTool.ApplyPatch,
	)
}
