package main

import (
	"os"
	"log"
	"errors"
	"context"

	"github.com/gdamore/tcell/v2"
	"github.com/openai/openai-go/v3"

	"github.com/hello-llm-2/ui"
)

var apiKeys  = map[string]string{
	"OPENAI_API_KEY": "OpenAI's GPT models",
	"ANTHROPIC_API_KEY": "Anthropic's Claude models",
	"GEMINI_API_KEY": "Google's Gemini models",
};

type AppState struct {
	userPromptBuf []rune
	llmResponse string
	client openai.Client

	userPromptCached string
	dirtyUserPrompt bool
}

func NewAppState() *AppState {
	userPromptBuf := make([]rune, 0, 100)
	userPromptBuf = append(userPromptBuf, '>')
	userPromptBuf = append(userPromptBuf, ' ')

	llmResponse := ""

	client := openai.NewClient()

	return &AppState {
		userPromptBuf,
		llmResponse,
		client,
		string(userPromptBuf), 
		false,
	}
}

func (a *AppState) UserPromptAppendRune(r rune) {
	a.userPromptBuf = append(a.userPromptBuf, r)
	a.dirtyUserPrompt = true
}

func (a *AppState) UserPromptPop() {
	// Prefix len is 2, too lazy to make a const
	// will come in the future
	if len(a.userPromptBuf) == 2 {
		return
	}

	a.userPromptBuf = a.userPromptBuf[:len(a.userPromptBuf)-1]
	a.dirtyUserPrompt = true
}

func (a *AppState) UserPromptClear() {
	if len(a.userPromptBuf) == 2 {
		return
	}

	a.userPromptBuf = a.userPromptBuf[:2]
	a.dirtyUserPrompt = true
}

func (a *AppState) LlmResponsePush(chunk string) {
	a.llmResponse += chunk
}

func (a *AppState) LlmResponseClear() {
	a.llmResponse = ""
}

func (a *AppState) UserPromptContent() []rune {
	return a.userPromptBuf[2:]
}

func (a *AppState) UserPromptPrefixed() string {
	if a.dirtyUserPrompt {
		a.userPromptCached = string(a.userPromptBuf)
	}

	return a.userPromptCached
}

func (a *AppState) LlmResponse() string {
	return a.llmResponse
}

func (a *AppState) LlmClient() openai.Client {
	return a.client
}

func UserPromptSubmit(ctx context.Context, client openai.Client, prompt string, evTx chan<- AppEvent) {
	ctx, _ = context.WithCancel(ctx)
	
	go func() {
		stream := client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams {
			Messages: []openai.ChatCompletionMessageParamUnion {
				openai.UserMessage(prompt),
			},
			Seed: openai.Int(0),
			Model: openai.ChatModelGPT4oMini,
		})

		acc := openai.ChatCompletionAccumulator{}

		for stream.Next() {
			chunk := stream.Current()
			acc.AddChunk(chunk)

			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				evTx <-	AppEvent {Type: EvLlmContentArrived, Data: chunk.Choices[0].Delta.Content}
			}
		}
	}()
}

func DrawScreen(app *AppState, screen tcell.Screen) {
	screen.Clear()

	_, height := screen.Size()

	cursorX, cursorY := ui.NewText(
		app.UserPromptPrefixed(),
		ui.TextParams{
			OffsetsUpwards: true,
		}).Draw(
		screen,
		height-1,
	)

	ui.NewText(
		app.LlmResponse(),
		ui.TextParams{},
		).Draw(
		screen,
		0,
	)

	screen.ShowCursor(cursorX, cursorY)
	screen.Show()
}

func ReceiveTuiEvent(tuiEv <-chan tcell.Event, appEvTx chan<- AppEvent) {
	for ev := range tuiEv {
		switch ev.(type) {
		case *tcell.EventKey:
			switch ev.(*tcell.EventKey).Key() {
				case tcell.KeyCtrlC:
					appEvTx <- AppEvent {Type: EvQuit}
				case tcell.KeyBackspace:
					appEvTx <- AppEvent {Type: EvUserPromptPop}
				case tcell.KeyEnter:
					appEvTx <- AppEvent {Type: EvUserPromptSubmit}
				case tcell.KeyRune:
					appEvTx <- AppEvent{Type: EvUserPromptInput, Rune: ev.(*tcell.EventKey).Rune()}
			}
		}
	}
}

func WarmupCache(cacheFilePath string) {
}

type AppEvent struct {
	Type AppEventType
	Rune rune
	Data string
}

type AppEventType int
const (
	EvQuit AppEventType = iota
	EvUserPromptInput
	EvUserPromptPop
	EvUserPromptSubmit
	EvLlmContentArrived
)

func RunEventLoop(ctx context.Context, app *AppState, screen tcell.Screen, evRxTx chan AppEvent) {
	var evRx <-chan AppEvent = evRxTx
	// Allows the event loop to send transmitters to other parts of the app
	var evTx chan<- AppEvent = evRxTx

	DrawScreen(app, screen)

	for ev := range evRx {
		switch ev.Type {
		case EvQuit:
			return
		case EvUserPromptInput:
			app.UserPromptAppendRune(ev.Rune)
		case EvUserPromptPop:
			app.UserPromptPop()
		case EvUserPromptSubmit:
			app.LlmResponseClear() // Clears pv response
			requestCtx, _ := context.WithCancel(ctx)
			prompt := string(app.UserPromptContent())
			app.UserPromptClear()
			UserPromptSubmit(requestCtx, app.LlmClient(), prompt, evTx)
		case EvLlmContentArrived:
			app.LlmResponsePush(ev.Data)
		}

		DrawScreen(app, screen)
	}
}

func main() {
	screen, err := tcell.NewScreen();
	err = screen.Init();
	if err != nil {
		log.Fatal("Failed to create a screen: ", err);
	}
	defer screen.Fini();

	app := NewAppState()

	cacheDir, err := os.UserCacheDir();
	if err != nil {
		log.Fatal("Failed to retrieve user cache dir", err);
	}
	cacheFilePath := cacheDir + "/hello-llm/cache.json";

	if _, err := os.Stat(cacheFilePath);
	errors.Is(err, os.ErrNotExist) {
		WarmupCache(cacheFilePath);
	}

	tuiQuit := make(chan struct{})
	tuiEventsCh := make(chan tcell.Event)
	appEv := make(chan AppEvent, 50)

	go screen.ChannelEvents(tuiEventsCh, tuiQuit)
	defer close(tuiQuit)

	go ReceiveTuiEvent(tuiEventsCh, appEv)

	ctx, cancelCtx := context.WithCancel(context.Background())
	RunEventLoop(ctx, app, screen, appEv)
	defer cancelCtx()
}
