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
	Endpoint: "https://api.openai.com/v1/chat/completions",
	Models: NewModelSelector("gpt-5-nano", "gpt-5-nano", "gpt-5.2"),
	ApiKey: os.Getenv("OPENAI_API_KEY"),
	WebSearchField: map[string]any {
		"web_search_options": map[string]any{},
	},
	UseDeveloperRole: true,
}

type OpenaiProvider struct {
	Endpoint string
	Models ModelSelector
	ApiKey string
	// Varies from provider to provider
	WebSearchField map[string]any
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
		"messages": messages,
		"stream": true,
	}

	if params.AllowWebSearch {
		for k, v := range p.WebSearchField {
			bodyStruct[k] = v
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

		eventData := eventRes.eventData
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
