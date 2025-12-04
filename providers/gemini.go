package providers

import (
	"io"
	"os"
	"fmt"
	"bytes"
	"net/http"
	"context"
	"strings"
	"encoding/json"
)

type GeminiProvider struct {}

type part struct {
	Text string `json:"text"`
}

type systemInstruction struct {
	Parts []part `json:"parts"`
}

type apiMessage struct {
	Role string `json:"role"`
	Parts []part `json:"parts"`
}

type thinkingConfig struct {
	ThinkingBudget int `json:"thinkingBudget"`
}

type generationConfig struct {
	ThinkingConfig thinkingConfig `json:"thinkingConfig"`
}

func (_ GeminiProvider) StartStreamingRequest(ctx context.Context, params StreamingRequestParams) {
	model := "gemini-2.0-flash-lite"
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?alt=sse", model)

	systemPrompt := strings.Builder{}	
	messages := make([]apiMessage, 0, len(params.Messages))
	for _, msg := range params.Messages {
		if msg.Type == MessageTypeSystem {
			systemPrompt.WriteString(msg.Content)
			systemPrompt.WriteByte(' ')
		} else {
			var role string
			if msg.Type == MessageTypeAssistant {
				role = "model"
			} else {
				role = "user"
			}

			messages = append(messages, apiMessage{
				Role: role,
				Parts: []part{
					part {
						Text: msg.Content,
					},
				},
			})
		}
	}

	// i hate google
	bodyStruct := map[string]any {
		"system_instruction": systemInstruction {
			Parts: []part{part{Text:systemPrompt.String()}},
		},
		"contents": messages,
		"generationConfig": generationConfig{ThinkingConfig:thinkingConfig{ThinkingBudget: 0}},
	}
	body, err := json.Marshal(bodyStruct)
	if err != nil {
		panic(err)
	}

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("x-goog-api-key", os.Getenv("GEMINI_API_KEY"))

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
			if err == io.EOF && params.OnStreamingEnd != nil{
				params.OnStreamingEnd(wholeContent.String())
				return
			}

			if params.OnStreamingErr != nil {
				params.OnStreamingErr(err)
			}
			return
		}

		if len(eventData) > 0 {
			// mhh nice dirty parsing
			var jsonPayload = struct{ 
				Candidates []struct{ 
					Content struct{ 
						Parts []struct{ Text string `json:"text"` } `json:"parts"` 
					} `json:"content"` 
					FinishReason string `json:"finishReason"`
				} `json:"candidates"` 
			}{}

			json.Unmarshal([]byte(eventData), &jsonPayload)
			result := jsonPayload.Candidates[0].Content.Parts[0].Text
			if jsonPayload.Candidates[0].FinishReason == "STOP" {
				// there is actually a bug here where the first chunk sometimes sends a STOP finish reason for some reasons...
				result = strings.TrimRight(result, "\n")
			}
			wholeContent.WriteString(result)
			params.OnChunkReceived(result)
		}
	}
}
