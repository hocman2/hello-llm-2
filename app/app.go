package app

import (
	"log"
	"fmt"
	"strings"
	"github.com/hello-llm-2/providers"
)

type NamedPipeFileFailureType int

const (
	NamedPipeFailureNone NamedPipeFileFailureType = iota
	NamedPipeFailureAlreadyExists 
	NamedPipeFailureNotAllowed
	NamedPipeFailureNoSuitablePath
	NamedPipeFailureOther
)

type NamedPipeFile struct {
	Path string
	Failure NamedPipeFileFailureType
}

type AppConfig struct {
	Provider providers.ProviderType
	AllowWebSearch bool
	SystemPrompt string
	NamedPipe NamedPipeFile
}

type AppState struct {
	FreeScrollMode bool
	ScrollPosition int
	UserError string

	cfg *AppConfig
	userPromptBuf []rune
	chatHistory []providers.AgnosticConversationMessage
	currentLlmResponse string
	provider providers.Provider
	pipedContent string
}

func NewAppState(cfg *AppConfig) *AppState {
	userPromptBuf := make([]rune, 0, 100)
	userPromptBuf = append(userPromptBuf, '>')
	userPromptBuf = append(userPromptBuf, ' ')

	chatHistory := []providers.AgnosticConversationMessage{
		providers.AgnosticConversationMessage{
			Type: providers.MessageTypeSystem,
			Content: cfg.SystemPrompt,
		},
	}

	var provider providers.Provider
	switch cfg.Provider {
	case providers.ProviderOpenai:
		provider = providers.OpenaiProvider{}
	case providers.ProviderGemini:
		provider = providers.GeminiProvider{}
	default:
		log.Fatal(cfg.Provider, "Unimplemented provider")
	}

	return &AppState {
		FreeScrollMode: false,
		ScrollPosition: 0,
		UserError: "",
		cfg: cfg,
		userPromptBuf: userPromptBuf,
		chatHistory: chatHistory,
		currentLlmResponse: "",
		provider: provider,
		pipedContent: "",
	}
}

func (app *AppState) Cfg() AppConfig {
	return *app.cfg
}

func (a *AppState) ContextAppend(extraContext string) {
	msg := providers.AgnosticConversationMessage {
		Type: providers.MessageTypeSystem,
		Content: fmt.Sprintf("Here is some user provided context:\n%s", extraContext),
	}

	if len(a.chatHistory) == 1 {
		a.chatHistory = append(a.chatHistory, msg)
	} else {
		a.chatHistory = append(a.chatHistory[:1], append([]providers.AgnosticConversationMessage{msg}, a.chatHistory[1:]...)...)
	}
}

func (a *AppState) UserPromptAppendRune(r rune) {
	a.userPromptBuf = append(a.userPromptBuf, r)
}

func (a *AppState) UserPromptSet(val string) {
	a.UserPromptClear()
	a.userPromptBuf = append(a.userPromptBuf, []rune(val)...)
}

func (a *AppState) UserPromptPop() {
	// Prefix len is 2, too lazy to make a const
	// will come in the future
	if len(a.userPromptBuf) == 2 {
		return
	}

	a.userPromptBuf = a.userPromptBuf[:len(a.userPromptBuf)-1]
}

func (a *AppState) UserPromptClear() {
	if len(a.userPromptBuf) == 2 {
		return
	}

	a.userPromptBuf = a.userPromptBuf[:2]
}

func (a *AppState) LlmResponsePush(chunk string) {
	a.currentLlmResponse += chunk
}

func (a *AppState) LlmResponseFinalize() {
	if a.currentLlmResponse == "" {
		return
	}

	a.chatHistory = append(
		a.chatHistory,
		providers.AgnosticConversationMessage{
			Type: providers.MessageTypeAssistant,
			Content: a.currentLlmResponse,
		})
	a.currentLlmResponse = ""
}

func (a *AppState) ChatHistoryAppendUserPrompt() {
	if a.pipedContent != "" {
		a.chatHistory = append(
			a.chatHistory,
			providers.AgnosticConversationMessage{
				Type: providers.MessageTypeUserContext,
				Content: a.pipedContent,
			})
		a.pipedContent = ""
	}

	a.chatHistory = append(
		a.chatHistory,
		providers.AgnosticConversationMessage{
			Type: providers.MessageTypeUser,
			Content: string(a.UserPromptContent()),
		})
}

func (a *AppState) UserPromptContent() []rune {
	return a.userPromptBuf[2:]
}

func (a *AppState) UserPromptPrefixed() string {
	return string(a.userPromptBuf)
}

func (a *AppState) UserPromptEmpty() bool {
	return len(a.userPromptBuf) == 2
}

// Returns the whole chat history appended with the current user prompt
func (a *AppState) ChatHistory() []providers.AgnosticConversationMessage {
	return a.chatHistory
}

// Builds the chat history as a single string, each message separated by newline. Also contains the current LLM response
func (a *AppState) ChatHistoryBuild() string {
	builder := strings.Builder{}
	for _, msg := range a.chatHistory {
		switch msg.Type {
		case providers.MessageTypeSystem:
			continue
		case providers.MessageTypeUserContext:
			continue
		case providers.MessageTypeUser:
			builder.WriteString("> ")
		}

		builder.WriteString(msg.Content)
		builder.WriteRune('\n')
	}
	builder.WriteString(a.currentLlmResponse)

	return builder.String()
}

func (a *AppState) Provider() providers.Provider {
	return a.provider
}

func (a *AppState) NamedPipe() NamedPipeFile {
	return a.cfg.NamedPipe
}

func (a *AppState) PipedContentSet(content string) {
	a.pipedContent = content
}

func (a *AppState) PipedContent() string {
	return a.pipedContent
}
