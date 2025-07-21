package multiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type state struct {
	originalInput string
	autoFixResult string
	searchResult  string
	messages      []*schema.Message
}

type HostDecision struct {
	Option  string `json:"option"`
	Context struct {
		AutoFix string `json:"autofix"`
		Search  string `json:"search"`
	} `json:"context"`
}

const (
	nodeKeyHost            = "host"
	nodeKeyAutoFixer       = "autofixer"
	nodeKeySearcher        = "searcher"
	nodeKeyHumanHelper     = "humanhelper"
	nodeKeyHostToList      = "host_to_list"
	nodeKeyAutoFixerToList = "autofixer_to_list"
	nodeKeySearcherToList  = "searcher_to_list"
	defaultMaxStep         = 50
)

func newGraphRunnable(ctx context.Context, config *LogMultiAgentConfig) (compose.Runnable[[]*schema.Message, *schema.Message], error) {
	graph := compose.NewGraph[[]*schema.Message, *schema.Message](
		compose.WithGenLocalState(func(ctx context.Context) *state {
			return &state{}
		}),
	)

	hostPreHandle := func(ctx context.Context, input []*schema.Message, state *state) ([]*schema.Message, error) {
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

		userMessage := state.originalInput
		if contextInfo != "" {
			userMessage = fmt.Sprintf("%s\n\n历史上下文:\n%s", state.originalInput, contextInfo)
		}

		return []*schema.Message{
			HostSystemPrompt,
			schema.UserMessage(userMessage),
		}, nil
	}

	autoFixerPreHandle := func(ctx context.Context, input []*schema.Message, state *state) ([]*schema.Message, error) {
		return []*schema.Message{
			AutoFixerSystemPrompt,
			schema.UserMessage(fmt.Sprintf("请对以下K8s问题进行自动修复：\n%s", state.originalInput)),
		}, nil
	}

	searcherPreHandle := func(ctx context.Context, input []*schema.Message, state *state) ([]*schema.Message, error) {
		return []*schema.Message{
			SearcherSystemPrompt,
			schema.UserMessage(fmt.Sprintf("请搜索以下K8s问题的解决方案：\n原始问题：%s\nAutoFix失败结果：%s", state.originalInput, state.autoFixResult)),
		}, nil
	}

	humanHelperPreHandle := func(ctx context.Context, input []*schema.Message, state *state) ([]*schema.Message, error) {
		return []*schema.Message{
			HumanHelperSystemPrompt,
			schema.UserMessage(fmt.Sprintf(`请生成问题处理文档：  
			原始问题：%s  
			自动修复结果：%s    
			搜索结果：%s  
			
			请生成包含问题描述、失败分析、建议解决方案的完整文档。`, state.originalInput, state.autoFixResult, state.searchResult)),
		}, nil
	}

	_ = graph.AddChatModelNode(nodeKeyHost, config.Host,
		compose.WithStatePreHandler[[]*schema.Message, *state](hostPreHandle),
		compose.WithNodeName(nodeKeyHost))

	_ = graph.AddChatModelNode(nodeKeyAutoFixer, config.Autofixer,
		compose.WithStatePreHandler[[]*schema.Message, *state](autoFixerPreHandle),
		compose.WithStatePostHandler[*schema.Message, *state](func(ctx context.Context, output *schema.Message, state *state) (*schema.Message, error) {
			state.autoFixResult = output.Content
			return output, nil
		}),
		compose.WithNodeName(nodeKeyAutoFixer))

	_ = graph.AddChatModelNode(nodeKeySearcher, config.Searcher,
		compose.WithStatePreHandler[[]*schema.Message, *state](searcherPreHandle),
		compose.WithStatePostHandler[*schema.Message, *state](func(ctx context.Context, output *schema.Message, state *state) (*schema.Message, error) {
			state.searchResult = output.Content
			return output, nil
		}),
		compose.WithNodeName(nodeKeySearcher))

	_ = graph.AddChatModelNode(nodeKeyHumanHelper, config.Searcher,
		compose.WithStatePreHandler[[]*schema.Message, *state](humanHelperPreHandle),
		compose.WithNodeName(nodeKeyHumanHelper))

	_ = graph.AddLambdaNode(nodeKeyHostToList, compose.ToList[*schema.Message]())
	_ = graph.AddLambdaNode(nodeKeyAutoFixerToList, compose.ToList[*schema.Message]())
	_ = graph.AddLambdaNode(nodeKeySearcherToList, compose.ToList[*schema.Message]())

	hostBranchCondition := func(ctx context.Context, msg *schema.Message) (string, error) {
		var decision HostDecision
		if err := json.Unmarshal([]byte(msg.Content), &decision); err != nil {
			content := strings.ToLower(msg.Content)
			if strings.Contains(content, "autofixer") {
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
			return nodeKeyHostToList, nil
		case "Searcher":
			return nodeKeySearcherToList, nil
		case "HumanHelper":
			return nodeKeyHumanHelper, nil
		case "Finish":
			return compose.END, nil
		default:
			return compose.END, nil
		}
	}

	autoFixerBranchCondition := func(ctx context.Context, msg *schema.Message) (string, error) {
		if strings.Contains(strings.ToLower(msg.Content), "修复成功") {
			return nodeKeyAutoFixerToList, nil
		}
		return nodeKeyAutoFixerToList, nil
	}

	searcherBranchCondition := func(ctx context.Context, msg *schema.Message) (string, error) {
		return nodeKeySearcherToList, nil
	}

	_ = graph.AddEdge(compose.START, nodeKeyHost)

	_ = graph.AddBranch(nodeKeyHost, compose.NewGraphBranch(hostBranchCondition, map[string]bool{
		nodeKeyHostToList:     true,
		nodeKeySearcherToList: true,
		nodeKeyHumanHelper:    true,
		compose.END:           true,
	}))

	_ = graph.AddEdge(nodeKeyHostToList, nodeKeyAutoFixer)
	_ = graph.AddBranch(nodeKeyAutoFixer, compose.NewGraphBranch(autoFixerBranchCondition, map[string]bool{
		nodeKeyAutoFixerToList: true,
	}))
	_ = graph.AddEdge(nodeKeyAutoFixerToList, nodeKeyHost)

	_ = graph.AddEdge(nodeKeySearcherToList, nodeKeySearcher)
	_ = graph.AddBranch(nodeKeySearcher, compose.NewGraphBranch(searcherBranchCondition, map[string]bool{
		nodeKeySearcherToList: true,
	}))
	_ = graph.AddEdge(nodeKeySearcherToList, nodeKeyHost)

	_ = graph.AddEdge(nodeKeyHumanHelper, compose.END)

	runnable, err := graph.Compile(ctx,
		compose.WithNodeTriggerMode(compose.AnyPredecessor),
		compose.WithMaxRunSteps(6))
	if err != nil {
		return nil, err
	}
	return runnable, nil
}
