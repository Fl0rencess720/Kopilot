package multiagent

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type SinkMessageContent struct {
	OriginalInput   string `json:"originalInput"`
	AutoFixResult   string `json:"autoFixResult"`
	SearchResult    string `json:"searchResult"`
	HumanHelpResult string `json:"humanHelpResult"`
}

func buildSinkMsg(ctx context.Context, input *schema.Message) (SinkMessageContent, error) {
	sinkMessageContent := SinkMessageContent{}
	if err := compose.ProcessState(ctx, func(ctx context.Context, state *state) error {
		sinkMessageContent.OriginalInput = input.Content
		sinkMessageContent.AutoFixResult = state.autoFixResult
		sinkMessageContent.SearchResult = state.searchResult
		sinkMessageContent.HumanHelpResult = state.humanHelpResult
		return nil
	}); err != nil {
		return SinkMessageContent{}, err
	}
	return sinkMessageContent, nil
}
