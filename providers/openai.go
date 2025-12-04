package providers

import (
	"fmt"
	"bytes"
	"os"
	"io"
	"net/http"
	"context"
	"strings"
	"encoding/json"
)

type OpenaiProvider struct {}

func (_ OpenaiProvider) StartStreamingRequest(ctx context.Context, params StreamingRequestParams) {
	model := "gpt-4o-mini"
	url := fmt.Sprintf("https://api.openai.com/v1/chat/completions")


	type ApiMessage struct {
		Content string `json:"content"`
		Role string `json:"role"`
	}
	messages := make([]ApiMessage, 0, len(params.Messages))
	for _, msg := range params.Messages {
		var role string
		switch msg.Type {
		case MessageTypeSystem:
			role = "developer"
		case MessageTypeAssistant:
			role = "assistant"
		case MessageTypeUser, MessageTypeUserContext:
			role = "user"
		}
		messages = append(messages, ApiMessage{
			Content: msg.Content,
			Role: role,
		})
	}

	bodyStruct := map[string]any {
		"model": model,
		"messages": messages,
		"stream": true,
	}

	body, err := json.Marshal(bodyStruct)
	if err != nil {
		panic(err)
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Authorization", "Bearer " + os.Getenv("OPENAI_API_KEY"))

	reader, err := startSseRequest(req)
	if err != nil && params.OnStreamingErr != nil {
		params.OnStreamingErr(err)
		return
	}
	defer reader.Close()

	wholeContent := strings.Builder{}
	for {
		eventData, err := reader.Next()
		if err != nil {
			if err == io.EOF && params.OnStreamingEnd != nil {
				params.OnStreamingEnd(wholeContent.String())
			}

			if params.OnStreamingErr != nil {
				params.OnStreamingErr(err)
			}

			return
		}

		if len(eventData) > 0 {
			if eventData == "[DONE]" {
				params.OnStreamingEnd(wholeContent.String())
				return
			}

			var jsonPayload = struct{
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}{}

			json.Unmarshal([]byte(eventData), &jsonPayload)
			if len(jsonPayload.Choices) > 0 {
				wholeContent.WriteString(jsonPayload.Choices[0].Delta.Content)
				params.OnChunkReceived(jsonPayload.Choices[0].Delta.Content)
			}
		}
	}
}
