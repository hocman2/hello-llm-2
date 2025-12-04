package providers

import (
	"io"
	"os"
	"fmt"
	"bytes"
	"bufio"
	"net/http"
	"context"
	"strings"
	"encoding/json"
)

// This provider's client requires a context
// We'll just recreate it each time a new request pops in
type GeminiProvider struct {
}

func NewGeminiProvider() *GeminiProvider {
	return &GeminiProvider{}
}

func (p *GeminiProvider) StartStreamingRequest(ctx context.Context, params StreamingRequestParams) {
	// Given how trash the gemini sdk is i'll use raw json instead
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

	client := http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil && params.OnStreamingErr != nil {
		params.OnStreamingErr(ErrRequestSending)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && params.OnStreamingErr != nil{
		params.OnStreamingErr(ErrStatusNotOK)
	}

	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") && params.OnStreamingErr != nil {
		params.OnStreamingErr(ErrContentTypeNotEventStream)
	}

	reader := bufio.NewReader(resp.Body)
	eventData := strings.Builder{}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				params.OnStreamingEnd("")
				return
			}

			if params.OnStreamingErr != nil {
				params.OnStreamingErr(ErrReadingBody)
			}
		}

		if strings.TrimSpace(line) == "" {
			if eventData.Len() > 0 {
				// mhh nice dirty parsing
				var jsonPayload = struct{ 
					Candidates []struct{ 
						Content struct{ 
							Parts []struct{ Text string `json:"text"` } `json:"parts"` 
						} `json:"content"` 
						FinishReason string `json:"finishReason"`
					} `json:"candidates"` 
				}{}

				json.Unmarshal([]byte(eventData.String()), &jsonPayload)
				result := jsonPayload.Candidates[0].Content.Parts[0].Text
				if jsonPayload.Candidates[0].FinishReason == "STOP" {
					// there is actually a bug here where the first chunk sometimes sends a STOP finish reason for some reasons...
					result = strings.TrimRight(result, "\n")
				}
				params.OnChunkReceived(result)
			}

			eventData = strings.Builder{}
			continue
		}

		if strings.HasPrefix(line, ":") {
			continue
		}

		if idx := strings.IndexByte(line, ':'); idx != -1 {
			field := line[:idx]
			value := line[idx+1:]
			switch field {
			case "data":
				eventData.WriteString(strings.TrimSpace(value))
			default:
			//dont care 😆🍭
			}
		}
	}
}
