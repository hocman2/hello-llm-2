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

	messages := strings.Builder{}
	for i, msg := range params.Messages {
		content := strings.ReplaceAll(msg.Content, `\`, `\\`)
		content = strings.ReplaceAll(content, `"`, `\"`)

		var role string
		switch msg.Type {
		case MessageTypeSystem:
			role = "developer"
		case MessageTypeAssistant:
			role = "assistant"
		case MessageTypeUser, MessageTypeUserContext:
			role = "user"
		}

		messages.WriteString(fmt.Sprintf(`{"content":"%s","role":"%s"}`, content, role))
		if i < len(params.Messages)-1 {
			 messages.WriteByte(',')
		}
	}

	body := []byte(fmt.Sprintf(`{"model":"%s","messages":[%s],"stream":true}`, model, messages.String()))

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
				return
			}

			if params.OnStreamingErr != nil {
				params.OnStreamingErr(err)
			}
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
