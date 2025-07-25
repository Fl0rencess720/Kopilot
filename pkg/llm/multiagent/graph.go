package multiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Fl0rencess720/Kopilot/pkg/llm"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type state struct {
	originalInput    string
	resourceYaml     string
	autoFixResult    string
	searchResult     string
	humanHelpResult  string
	hasKnowledgeBase bool
	messages         []*schema.Message
	language         string
}

type HostDecision struct {
	Option  string `json:"option"`
	Context struct {
		AutoFix string `json:"autofix"`
		Search  string `json:"search"`
	} `json:"context"`
}

const (
	nodeKeyHost               = "host"
	nodeKeyAutoFixer          = "autofixer"
	nodeKeySearcher           = "searcher"
	nodeKeyHumanHelper        = "humanhelper"
	nodeKeyHostToList         = "host_to_list"
	nodeKeyAutoFixerToList    = "autofixer_to_list"
	nodeKeySearcherToList     = "searcher_to_list"
	nodeKeyHumanHelperToList  = "humanhelper_to_list"
	nodeKeyAutoFixerRetriever = "autofixer_retriever"
	nodeKeyFinish             = "finish"
)

func buildGraphRunnable(ctx context.Context, config *LogMultiAgentConfig) (compose.Runnable[[]*schema.Message, *SinkMessageContent], error) {
	hasKnowledgeBase := false
	if config.Retriever != nil {
		hasKnowledgeBase = true
	}

	autoFixerAgent, autoFixerOpts, err := newAutoFixerAgent(ctx, config.Autofixer, config.dynamicClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create auto fix node: %w", err)
	}
	searcherAgent, searcherOpts, err := newSearcherAgent(ctx, config.Searcher)
	if err != nil {
		return nil, fmt.Errorf("failed to create searcher node: %w", err)
	}
	humanHelperAgent, humanHelperOpts, err := newHumanHelperAgent(ctx, config.Humanhelper)
	if err != nil {
		return nil, fmt.Errorf("failed to create human helper node: %w", err)
	}

	graph := compose.NewGraph[[]*schema.Message, *SinkMessageContent](
		compose.WithGenLocalState(func(ctx context.Context) *state {
			return &state{
				hasKnowledgeBase: hasKnowledgeBase,
				language:         config.language,
			}
		}),
	)

	_ = graph.AddChatModelNode(nodeKeyHost, config.Host,
		compose.WithStatePreHandler(hostPreHandle),
		compose.WithNodeName(nodeKeyHost))

	autoFixerOpts = append(autoFixerOpts, compose.WithStatePreHandler(autoFixerPreHandle),
		compose.WithStatePostHandler(func(ctx context.Context, output *schema.Message, state *state) (*schema.Message, error) {
			state.autoFixResult = output.Content
			return output, nil
		}),
		compose.WithNodeName(nodeKeyAutoFixer))
	_ = graph.AddGraphNode(nodeKeyAutoFixer, autoFixerAgent, autoFixerOpts...)

	searcherOpts = append(searcherOpts, compose.WithStatePreHandler(searcherPreHandle),
		compose.WithStatePostHandler(func(ctx context.Context, output *schema.Message, state *state) (*schema.Message, error) {
			state.searchResult = output.Content
			return output, nil
		}),
		compose.WithNodeName(nodeKeySearcher))
	_ = graph.AddGraphNode(nodeKeySearcher, searcherAgent, searcherOpts...)

	humanHelperOpts = append(humanHelperOpts, compose.WithStatePreHandler(humanHelperPreHandle),
		compose.WithNodeName(nodeKeyHumanHelper))

	_ = graph.AddGraphNode(nodeKeyHumanHelper, humanHelperAgent, humanHelperOpts...)

	_ = graph.AddLambdaNode(nodeKeyHostToList, compose.ToList[*schema.Message]())
	_ = graph.AddLambdaNode(nodeKeyAutoFixerToList, compose.ToList[*schema.Message]())
	_ = graph.AddLambdaNode(nodeKeySearcherToList, compose.ToList[*schema.Message]())
	_ = graph.AddLambdaNode(nodeKeyHumanHelperToList, compose.ToList[*schema.Message]())
	_ = graph.AddLambdaNode(nodeKeyFinish, compose.InvokableLambda(buildSinkMsg))

	ragChain, err := newRAGChain(ctx, config.Retriever)
	if err != nil {
		return nil, fmt.Errorf("failed to create RAG chain: %w", err)
	}

	_ = graph.AddGraphNode(nodeKeyAutoFixerRetriever, ragChain)

	_ = graph.AddEdge(compose.START, nodeKeyHost)

	_ = graph.AddBranch(nodeKeyHost, compose.NewGraphBranch(hostBranchCondition, map[string]bool{
		nodeKeyHostToList:         true,
		nodeKeyAutoFixerToList:    true,
		nodeKeySearcherToList:     true,
		nodeKeyHumanHelperToList:  true,
		nodeKeyAutoFixerRetriever: true,
		nodeKeyFinish:             true,
	}))

	_ = graph.AddEdge(nodeKeyHostToList, nodeKeyHost)

	_ = graph.AddEdge(nodeKeyAutoFixerRetriever, nodeKeyAutoFixerToList)
	_ = graph.AddEdge(nodeKeyAutoFixerToList, nodeKeyAutoFixer)
	_ = graph.AddEdge(nodeKeyAutoFixer, nodeKeyHostToList)

	_ = graph.AddEdge(nodeKeySearcherToList, nodeKeySearcher)
	_ = graph.AddEdge(nodeKeySearcher, nodeKeyHostToList)

	_ = graph.AddEdge(nodeKeyHumanHelperToList, nodeKeyHumanHelper)
	_ = graph.AddEdge(nodeKeyHumanHelper, nodeKeyHostToList)

	_ = graph.AddEdge(nodeKeyFinish, compose.END)

	runnable, err := graph.Compile(ctx,
		compose.WithNodeTriggerMode(compose.AnyPredecessor))
	if err != nil {
		return nil, err
	}

	return runnable, nil
}

func hostPreHandle(ctx context.Context, input []*schema.Message, state *state) ([]*schema.Message, error) {
	if len(input) > 0 {
		state.originalInput = input[0].Content
	}

	contextInfo := ""
	if state.autoFixResult != "" {
		contextInfo += fmt.Sprintf("AutoFix结果: %s\n", state.autoFixResult)
	}
	if state.searchResult != "" {
		contextInfo += fmt.Sprintf("Search结果: %s\n", state.searchResult)
	}
	if state.humanHelpResult != "" {
		contextInfo += fmt.Sprintf("HumanHelp结果: %s\n", state.humanHelpResult)
	}

	userMessage := state.originalInput
	if contextInfo != "" {
		userMessage = fmt.Sprintf("%s\n\n历史上下文:\n%s", state.originalInput, contextInfo)
	}
	systemPrompt, err := HostSystemPrompt.Format(ctx, map[string]any{
		"lang": llm.GetLanguageName(state.language),
	}, schema.GoTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to format system prompt: %w", err)
	}
	return []*schema.Message{
		systemPrompt[0],
		schema.UserMessage(userMessage),
	}, nil
}

func autoFixerPreHandle(ctx context.Context, input []*schema.Message, state *state) ([]*schema.Message, error) {
	msg := []*schema.Message{}
	if state.hasKnowledgeBase {
		systemPrompt, err := AutoFixerSystemPrompt.Format(ctx, map[string]any{
			"lang": llm.GetLanguageName(state.language),
		}, schema.GoTemplate)
		if err != nil {
			return nil, fmt.Errorf("failed to format system prompt: %w", err)
		}
		msg = []*schema.Message{
			systemPrompt[0],
			schema.UserMessage(fmt.Sprintf("请参照运维文档对以下K8s问题进行自动修复：\n%s\n运维文档：\n%s\n", state.originalInput, input[0].Content)),
		}
	} else {
		systemPrompt, err := AutoFixerSystemPrompt.Format(ctx, map[string]any{
			"lang": llm.GetLanguageName(state.language),
		}, schema.GoTemplate)
		if err != nil {
			return nil, fmt.Errorf("failed to format system prompt: %w", err)
		}
		msg = []*schema.Message{
			systemPrompt[0],
			schema.UserMessage(fmt.Sprintf("请对以下K8s问题进行自动修复：\n%s", state.originalInput)),
		}
	}
	return msg, nil
}

func searcherPreHandle(ctx context.Context, input []*schema.Message, state *state) ([]*schema.Message, error) {
	systemPrompt, err := SearcherSystemPrompt.Format(ctx, map[string]any{
		"lang": llm.GetLanguageName(state.language),
	}, schema.GoTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to format system prompt: %w", err)
	}
	return []*schema.Message{
		systemPrompt[0],
		schema.UserMessage(fmt.Sprintf("请搜索以下K8s问题的解决方案：\n原始问题：%s\nAutoFix失败结果：%s", state.originalInput, state.autoFixResult)),
	}, nil
}

func humanHelperPreHandle(ctx context.Context, input []*schema.Message, state *state) ([]*schema.Message, error) {
	systemPrompt, err := HumanHelperSystemPrompt.Format(ctx, map[string]any{
		"lang": llm.GetLanguageName(state.language),
	}, schema.GoTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to format system prompt: %w", err)
	}
	return []*schema.Message{
		systemPrompt[0],
		schema.UserMessage(fmt.Sprintf(`请生成问题处理文档：  
			原始问题：%s  
			自动修复结果：%s    
			搜索结果：%s  
			
			请生成包含问题描述、失败分析、建议解决方案的完整文档。`, state.originalInput, state.autoFixResult, state.searchResult)),
	}, nil
}

func hostBranchCondition(ctx context.Context, msg *schema.Message) (string, error) {
	var decision HostDecision
	if err := json.Unmarshal([]byte(msg.Content), &decision); err != nil {
		content := strings.ToLower(msg.Content)
		if strings.Contains(content, "autofixer") {
			hasKnowledgeBase := false
			if err := compose.ProcessState(ctx, func(ctx context.Context, state *state) error {
				hasKnowledgeBase = state.hasKnowledgeBase
				return nil
			}); err != nil {
				return "", err
			}
			if hasKnowledgeBase {
				return nodeKeyAutoFixerRetriever, nil
			}
			return nodeKeyHostToList, nil
		} else if strings.Contains(content, "searcher") {
			return nodeKeySearcherToList, nil
		} else if strings.Contains(content, "humanhelper") {
			return nodeKeyHumanHelper, nil
		} else {
			return compose.END, nil
		}
	}

	switch decision.Option {
	case "AutoFixer":
		return nodeKeyAutoFixerToList, nil
	case "Searcher":
		return nodeKeySearcherToList, nil
	case "HumanHelper":
		return nodeKeyHumanHelperToList, nil
	case "Finish":
		return nodeKeyFinish, nil
	default:
		return nodeKeyFinish, nil
	}
}

func newRAGChain(ctx context.Context, retriever *llm.HybridRetriever) (*compose.Chain[*schema.Message, *schema.Message], error) {
	ragChain := compose.NewChain[*schema.Message, *schema.Message]()
	ragChain.
		AppendLambda(compose.InvokableLambda(func(_ context.Context, input *schema.Message) (string, error) {
			var originalInput string
			if err := compose.ProcessState(ctx, func(ctx context.Context, state *state) error {
				originalInput = state.originalInput
				return nil
			}); err != nil {
				return "", err
			}
			return originalInput, nil
		})).
		AppendRetriever(retriever).
		AppendLambda(compose.InvokableLambda(func(_ context.Context, docs []*schema.Document) (*schema.Message, error) {
			var contents []string
			for _, doc := range docs {
				contents = append(contents, doc.Content)
			}
			knowledge := strings.Join(contents, "\n")
			return &schema.Message{Content: knowledge}, nil
		}))
	return ragChain, nil
}
