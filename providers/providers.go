package providers

import (
	"context"
	"errors"
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
