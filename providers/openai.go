package providers

import (
	"github.com/openai/openai-go/v3"
	"context"
)

type OpenaiProvider struct {
	client openai.Client
}

func NewOpenaiProvider() *OpenaiProvider {
	return &OpenaiProvider{
		client: openai.NewClient(),
	}
}

func (p *OpenaiProvider) StartStreamingRequest(ctx context.Context, params StreamingRequestParams) {
	var messages []openai.ChatCompletionMessageParamUnion
	for _, msg := range params.Messages {
		switch msg.Type {
		case MessageTypeAssistant:
			messages = append(messages, openai.AssistantMessage(msg.Content))
		case MessageTypeUser:
			messages = append(messages, openai.UserMessage(msg.Content))
		case MessageTypeSystem:
			messages = append(messages, openai.SystemMessage(msg.Content))
		}
	}

	stream := p.client.Chat.Completions.NewStreaming(
		ctx,
		openai.ChatCompletionNewParams {
			Messages: messages,
			Seed: openai.Int(0),
			Model: openai.ChatModelGPT4oMini,
		})

	acc := openai.ChatCompletionAccumulator{}

	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)

		if content, ok := acc.JustFinishedContent(); ok {
			if params.OnStreamingEnd != nil {
				params.OnStreamingEnd(content)
			}
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			if params.OnChunkReceived != nil {
				params.OnChunkReceived(chunk.Choices[0].Delta.Content)
			}
		}
	}
}
