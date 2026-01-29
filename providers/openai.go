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

var OpenaiProviderOpenai OpenaiProvider = OpenaiProvider {
	Endpoint: "https://api.openai.com/v1/responses",
	Models: NewModelSelector("gpt-5-nano", "gpt-5-nano", "gpt-5.2"),
	ApiKey: os.Getenv("OPENAI_API_KEY"),
	UseDeveloperRole: true,
}

type OpenaiProvider struct {
	Endpoint string
	Models ModelSelector
	ApiKey string
	// Openai uses "role":"developer" while some providers use "role":"system"
	UseDeveloperRole bool
}

func (p *OpenaiProvider) StartStreamingRequest(ctx context.Context, params StreamingRequestParams) {
	p.Models.SetCurrentSelection(params.ModelPreference)
	model := p.Models.Get()

	url := fmt.Sprintf(p.Endpoint)

	type ApiMessage struct {
		Content string `json:"content"`
		Role string `json:"role"`
	}
	messages := make([]ApiMessage, 0, len(params.Messages))
	for _, msg := range params.Messages {
		var role string
		switch msg.Type {
		case MessageTypeSystem:
			if p.UseDeveloperRole {
				role = "developer"
			} else {
				role = "system"
			}
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
		"input": messages,
		"stream": true,
	}

	if params.AllowWebSearch {
		bodyStruct["tool_choice"] = "auto"
		bodyStruct["tools"] = []map[string]any {
			{
				"type": "web_search",
			},
		}
	}

	body, err := json.Marshal(bodyStruct)
	if err != nil {
		panic(err)
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Authorization", "Bearer " + p.ApiKey)

	reader, err := startSseRequest(req)
	if err != nil && params.OnStreamingErr != nil {
		params.OnStreamingErr(err)
		return
	}
	defer reader.Close()

	wholeContent := strings.Builder{}
	for {
		eventRes, err := reader.Next()
		if err != nil {
			if err == io.EOF && params.OnStreamingEnd != nil {
				params.OnStreamingEnd(wholeContent.String())
			}

			if params.OnStreamingErr != nil {
				params.OnStreamingErr(err)
			}

			return
		}

		// Highly minimal handling of responses api
		eventData := eventRes.eventData
		if len(eventData) > 0 {
			var typePayload = struct{
				Type string `json:"type"`
			}{}
			json.Unmarshal([]byte(eventData), &typePayload)

			if eventData == "[DONE]" || typePayload.Type == "response.output_text.done" {
				params.OnStreamingEnd(wholeContent.String())
				return
			}

			if typePayload.Type == "response.output_text.delta" {
				var jsonPayload = struct {
					Delta string `json:"delta"`
				}{}
				json.Unmarshal([]byte(eventData), &jsonPayload)
				wholeContent.WriteString(jsonPayload.Delta)
				params.OnChunkReceived(jsonPayload.Delta)
			} else {
				//params.OnStreamingErr(errors.New(fmt.Sprintf("Unhandled event type: %s", typePayload.Type)))
			}
		}
	}
}
