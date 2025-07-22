package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func newRunnableWithRetriever(ctx context.Context, cm model.BaseChatModel, retriever retriever.Retriever) (compose.Runnable[map[string]any, *schema.Message], error) {

	chain := compose.NewChain[map[string]any, *schema.Message]()

	parallel := compose.NewParallel()

	ragChain := compose.NewChain[[]*schema.Message, string]()
	ragChain.
		AppendLambda(compose.InvokableLambda(func(_ context.Context, input []*schema.Message) (string, error) {
			return input[1].Content, nil
		})).
		AppendRetriever(retriever).
		AppendLambda(compose.InvokableLambda(func(_ context.Context, docs []*schema.Document) (string, error) {
			var contents []string
			for _, doc := range docs {
				contents = append(contents, doc.Content)
			}
			knowledge := strings.Join(contents, "\n")
			return knowledge, nil
		}))
	parallel.
		AddGraph("rag_chain", ragChain).
		AddPassthrough("pass_msg")
	ragReplacer := compose.InvokableLambda(func(_ context.Context, inputs map[string]any) ([]*schema.Message, error) {
		knowledge := inputs["rag_chain"].(string)
		msgs := inputs["pass_msg"].([]*schema.Message)
		msgs[1].Content += fmt.Sprintf("\n运维文档: %s\n", knowledge)
		msgs[1].Content += `你需要从运维文档找到对于给出日志中所显示的问题的合适的解决方案，若运维文档为空或没有找到合适的解决方案，
		则由你自己给出合适的解决方案。`
		return msgs, nil
	})
	chain.
		AppendChatTemplate(KubernetesLogAnalyzeSystemPrompt).
		AppendParallel(parallel).
		AppendLambda(ragReplacer).
		AppendChatModel(cm)

	runnable, err := chain.Compile(ctx)
	if err != nil {
		return nil, err
	}
	return runnable, nil
}

func newRunnable(ctx context.Context, cm model.BaseChatModel) (compose.Runnable[map[string]any, *schema.Message], error) {
	chain := compose.NewChain[map[string]any, *schema.Message]()
	chain.
		AppendChatTemplate(KubernetesLogAnalyzeSystemPrompt).
		AppendLambda(compose.InvokableLambda(func(_ context.Context, inputs []*schema.Message) ([]*schema.Message, error) {
			inputs[1].Content = fmt.Sprintf("日志内容: %s\n运维文档: %s\n", inputs[1].Content, "无运维文档")
			return inputs, nil
		})).
		AppendChatModel(cm)

	runnable, err := chain.Compile(ctx)
	if err != nil {
		return nil, err
	}
	return runnable, nil
}
