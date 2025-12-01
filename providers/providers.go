package providers

import (
	"context"
)

type ProviderType int
const (
	ProviderOpenai ProviderType = iota
	ProviderAnthropic
	ProviderGemini
	ProviderGrok
)

type MessageType int
const (
	MessageTypeAssistant MessageType = iota
	MessageTypeUser
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
}

type Provider interface {
	StartStreamingRequest(ctx context.Context, params StreamingRequestParams)
}
