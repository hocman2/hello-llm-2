package providers

import (
	"io"
	"bytes"
	"os"
	"context"
	"net/http"
	"strings"
	"encoding/json"
)

type AnthropicProvider struct {}

func (provider AnthropicProvider) StartStreamingRequest(ctx context.Context, params StreamingRequestParams) {
	model := "claude-haiku-4-5"
	url := "https://api.anthropic.com/v1/messages"

	type ApiMessage struct {
		Content string `json:"content"`
		Role string `json:"role"`
	}
	messages := make([]ApiMessage, 0, len(params.Messages))
	systemPrompt := strings.Builder{}
	for _, msg := range params.Messages {
		var role string
		switch msg.Type {
		case MessageTypeSystem:
			systemPrompt.WriteString(msg.Content)
			continue
		case MessageTypeUser, MessageTypeUserContext:
			role = "user"
		case MessageTypeAssistant:
			role = "assistant"
		}

		messages = append(messages, ApiMessage{
			Content: msg.Content,
			Role: role,
		})
	}

	bodyStruct := map[string]any {
		"model": model,
		"max_tokens": 1024,
		"messages": messages,
		"stream": true,
		"system": systemPrompt.String(),
	}

	if params.AllowWebSearch {
		bodyStruct["tools"] = []map[string]any {
			map[string]any {
				"type": "web_search_20250305",
				"name": "web_search",
				"max_uses": 5,
			},
}
	}

	body, err := json.Marshal(bodyStruct)
	if err != nil {
		panic(err)
	}

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	req.Header.Set("X-Api-Key", os.Getenv("ANTHROPIC_API_KEY"))
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	reader, err := startSseRequest(req)
	if err != nil && params.OnStreamingErr != nil {
		params.OnStreamingErr(err)
		return
	}
	defer reader.Close()

	wholeContent := strings.Builder{}
	for {
		readResult, err := reader.Next()
		if err != nil {
			if err == io.EOF && params.OnStreamingEnd != nil {
				params.OnStreamingEnd(wholeContent.String())
				return
			}

			if params.OnStreamingErr != nil {
				params.OnStreamingErr(err)
				return
			}
		}

		if len(readResult.eventData) > 0 {
			// Here we are assuming we only use a single content_block so message = content_block
			switch readResult.eventName {
			case "message_start":
				continue
			case "content_block_delta":
				var jsonPayload = struct {
					Index int `json:"index"`
					Delta struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"delta"`
				}{}

				json.Unmarshal([]byte(readResult.eventData), &jsonPayload)
				if jsonPayload.Index == 0 && jsonPayload.Delta.Type == "text_delta" {
					wholeContent.WriteString(jsonPayload.Delta.Text)
					params.OnChunkReceived(jsonPayload.Delta.Text)
				}
			case "message_stop":
				params.OnStreamingEnd(wholeContent.String())
			default:
			}
		}
	}
}
