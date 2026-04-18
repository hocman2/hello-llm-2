package providers

import (
	"io"
	"fmt"
	"context"
	"errors"
	"net/http"
	"bufio"
	"strings"
	"time"
)

type ProviderType int
const (
	ProviderOpenai ProviderType = iota
	ProviderAnthropic
	ProviderGemini
	ProviderGrok
	ProviderLast
)

func ProviderTypeToString(provider ProviderType) string {
	switch provider {
	case ProviderOpenai:
		return "openai"
	case ProviderAnthropic:
		return "anthropic"
	case ProviderGemini:
		return "google"
	case ProviderGrok:
		return "elonmusk"
	default:
		return "fuck you"
	}
}

func ProviderTypeFromString(t string) (ProviderType, error) {
	switch t {
	case "openai":
		return ProviderOpenai, nil
	case "anthropic":
		return ProviderAnthropic, nil
	case "google":
		return ProviderGemini, nil
	case "elonmusk":
		return ProviderGrok, nil
	default:
		return 0, errors.New("Unknown provider")
	}
}

type ModelPreference int
const (
	ModelPreferenceCheap ModelPreference = iota
	ModelPreferenceFast
	ModelPreferenceSmart
	ModelPreferenceLast
)

func ModelPreferenceToString(preference ModelPreference) string {
	switch preference {
	case ModelPreferenceCheap:
		return "cheap"
	case ModelPreferenceFast:
		return "fast"
	case ModelPreferenceSmart:
		return "smart"
	default:
		return "fuck you"
	}
}

func ModelPreferenceFromString(t string) (ModelPreference, error) {
	switch t {
	case "cheap":
		return ModelPreferenceCheap, nil
	case "fast":
		return ModelPreferenceFast, nil
	case "smart":
		return ModelPreferenceSmart, nil
	default:
		return 0, errors.New("Unknown model preference")
	}
}

type ModelSelector struct {
	models [ModelPreferenceLast]string
	currentSelection ModelPreference
}

func NewModelSelector(cheap, fast, smart string) ModelSelector {
	s := ModelSelector {}
	s.models[ModelPreferenceCheap] = cheap
	s.models[ModelPreferenceFast] = fast
	s.models[ModelPreferenceSmart] = smart
	return s
}

func (s *ModelSelector) SetCurrentSelection(pref ModelPreference) {
	s.currentSelection = pref
}

func (s *ModelSelector) Get() string {
	switch s.currentSelection {
	case ModelPreferenceCheap:
		return s.models[ModelPreferenceCheap]
	case ModelPreferenceFast:
		return s.models[ModelPreferenceFast]
	case ModelPreferenceSmart:
		return s.models[ModelPreferenceSmart]
	default:
		return s.models[ModelPreferenceCheap]
	}
}

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
	ModelPreference  ModelPreference
	AllowWebSearch bool
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
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 30 * time.Second
	client := http.Client{Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		return nil, ErrRequestSending
	}

	if resp.StatusCode != 200 {
		errReason, _ := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		return nil, errors.New(fmt.Sprintf("%s: %s", resp.Status, string(errReason)))
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

type sseReadResult struct {
	eventName string
	eventData string
}

// Read until an event data is formed and return it
func (reader *sseReader) Next() (sseReadResult, error) {
	res := sseReadResult{}
	eventData := strings.Builder{}

	for {
		line, err := reader.reader.ReadString('\n')
		if err != nil {
			return res, err
		}

		if strings.TrimSpace(line) == "" {
			if eventData.Len() > 0 {
				res.eventData = eventData.String()
				return res, nil
			}
		}

		if strings.HasPrefix(line, ":") {
			continue
		}

		if idx := strings.IndexByte(line, ':'); idx != -1 {
			field := line[:idx]
			value := line[idx+1:]
			switch field {
			case "event":
				res.eventName = strings.TrimSpace(value)
			case "data":
				eventData.WriteString(strings.TrimSpace(value))
			default:
			//dont care 😆🍭
			}
		}
	}
}
