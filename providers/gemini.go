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

func (_ GeminiProvider) StartStreamingRequest(ctx context.Context, params StreamingRequestParams) {
	model := "gemini-2.0-flash-lite"
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?alt=sse", model)

	messageHistory := strings.Builder{}
	systemPrompt := strings.Builder{}	
	for i, msg := range params.Messages {
		content := strings.ReplaceAll(msg.Content, `\`, `\\`)
		content = strings.ReplaceAll(content, `"`, `\"`)

		if msg.Type == MessageTypeSystem {
			systemPrompt.WriteString(content)
			systemPrompt.WriteByte(' ')
		} else {
			var role string
			if msg.Type == MessageTypeAssistant {
				role = "model"
			} else {
				role = "user"
			}

			messageHistory.WriteString(
				fmt.Sprintf(`{"role": "%s", "parts": [{"text": "%s"}]}`, role, content),
				)

			if i < len(params.Messages)-1 {
				messageHistory.WriteByte(',')
			}
		}
	}

	body := []byte(fmt.Sprintf(`{"system_instruction":{"parts":[{"text":"%s"}]},"contents":[%s],"generationConfig":{"thinkingConfig":{"thinkingBudget":0}}}`, strings.TrimSpace(systemPrompt.String()), messageHistory.String()))

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
