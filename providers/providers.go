package providers

import (
	"context"
	"errors"
	"net/http"
	"bufio"
	"strings"
)

type ProviderType int
const (
	ProviderOpenai ProviderType = iota
	ProviderAnthropic
	ProviderGemini
	ProviderGrok
)

var (
	ErrStatusNotOK error = errors.New("GET request was not 200 OK")
	ErrContentTypeNotEventStream error = errors.New("The response MIME type should be text/event-stream for streaming requests")
	ErrRequestSending error = errors.New("Something wrong happened when emitting request")
	ErrReadingBody error = errors.New("Failed to read body")
)

type MessageType int
const (
	MessageTypeAssistant MessageType = iota
	MessageTypeUser
	// Technically the same as MessageTypeUser but describes that this was context not necessarly worth printing back
	MessageTypeUserContext
	MessageTypeSystem
)

type AgnosticConversationMessage struct {
	Type MessageType
	Content string
}

type StreamingRequestParams struct {
	Messages []AgnosticConversationMessage
	OnChunkReceived func(chunk string)
	OnStreamingEnd func(content string)
	OnStreamingErr func(err error)
}

type Provider interface {
	StartStreamingRequest(ctx context.Context, params StreamingRequestParams)
}

type sseReader struct {
	resp *http.Response
	reader *bufio.Reader
}

func startSseRequest(req *http.Request) (*sseReader, error) {
	client := http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		return nil, ErrRequestSending
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(resp.Status)
	}

	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		return nil, ErrContentTypeNotEventStream
	}

	ssereader := sseReader{resp, bufio.NewReader(resp.Body)}
	return &ssereader, nil
}

func (reader *sseReader) Close() {
	reader.resp.Body.Close()
}

// Read until an event data is formed and return it
func (reader *sseReader) Next() (string, error) {
	eventData := strings.Builder{}
	for {
		line, err := reader.reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		if strings.TrimSpace(line) == "" {
			if eventData.Len() > 0 {
				return eventData.String(), nil
			}
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
