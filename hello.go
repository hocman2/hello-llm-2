package main

import (
	"io"
	"os"
	"fmt"
	"log"
	"bufio"
	"errors"
	"context"
	"strings"
	"syscall"

	"github.com/gdamore/tcell/v2"
	"github.com/adrg/xdg"

	"github.com/hello-llm-2/providers"
	"github.com/hello-llm-2/ui"
)

var apiKeys = map[string]string{
	"OPENAI_API_KEY": "OpenAI's GPT models",
	"ANTHROPIC_API_KEY": "Anthropic's Claude models",
	"GEMINI_API_KEY": "Google's Gemini models",
};

var systemPrompt providers.AgnosticConversationMessage = providers.AgnosticConversationMessage{
	Type: providers.MessageTypeSystem,
	Content: "You are a helpful assistant prompted from a terminal sheel. User expects straight to the point factual answers with minimal noise unless specified otherwise.",
}

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

type AppState struct {
	FreeScrollMode bool
	ScrollPosition int
	
	userPromptBuf []rune
	chatHistory []providers.AgnosticConversationMessage
	currentLlmResponse string
	provider providers.Provider
	namedPipe NamedPipeFile
	pipedContent string
	userPromptCached string
	dirtyUserPrompt bool
}

func NewAppState(providerType providers.ProviderType, namedPipe NamedPipeFile) *AppState {
	userPromptBuf := make([]rune, 0, 100)
	userPromptBuf = append(userPromptBuf, '>')
	userPromptBuf = append(userPromptBuf, ' ')

	chatHistory := []providers.AgnosticConversationMessage{
		systemPrompt,
	}

	currentLlmResponse := ""

	var provider providers.Provider
	switch providerType {
	case providers.ProviderOpenai:
		provider = providers.NewOpenaiProvider()
	default:
		log.Fatal(providerType, "Unimplemented provider")
	}

	return &AppState {
		false,
		0,
		userPromptBuf,
		chatHistory,
		currentLlmResponse,
		provider,
		namedPipe,
		"",
		string(userPromptBuf), 
		false,
	}
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
	a.dirtyUserPrompt = true
}

func (a *AppState) UserPromptSet(val string) {
	a.UserPromptClear()
	a.userPromptBuf = append(a.userPromptBuf, []rune(val)...)
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
	a.currentLlmResponse += chunk
}

func (a *AppState) LlmResponseFinalize() {
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
	if a.dirtyUserPrompt {
		a.userPromptCached = string(a.userPromptBuf)
	}

	return a.userPromptCached
}

func (a *AppState) UserPromptEmpty() bool {
	return len(a.userPromptBuf) == 2
}

// Returns the whole chat history appended with the current user prompt
func (a *AppState) ChatHistory() []providers.AgnosticConversationMessage {
	return a.chatHistory
}

// Builds the chat history as a single string, each message separated by newline. Also contains the current LLM response
func (a *AppState) BuildChatHistory() string {
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
	return a.namedPipe
}

func (a *AppState) SetPipedContent(content string) {
	a.pipedContent = content
}

func (a *AppState) PipedContent() string {
	return a.pipedContent
}

func UserPromptSubmit(ctx context.Context, msgs []providers.AgnosticConversationMessage, provider providers.Provider, evTx chan<- AppEvent) {
	ctx, _ = context.WithCancel(ctx)

	streamingParams := providers.StreamingRequestParams {
		Messages: msgs,
		OnChunkReceived: func(chunk string) {
			evTx <- AppEvent {Type: EvLlmContentArrived, Data: chunk}
		},
		OnStreamingEnd: func(_content string) {
			evTx <- AppEvent {Type: EvLlmContentFinished}
		},
	}

	go provider.StartStreamingRequest(ctx, streamingParams)
}

func BuildFifoFileElement(app *AppState) *ui.Text {
	pipe := app.NamedPipe();
	switch pipe.Failure {
	case 0:
		content := app.PipedContent()
		if content == "" {
			content = fmt.Sprintf("FIFO file listening. Writing to \"%s\" will add context to the conversation", pipe.Path)
		} else if len(content) > 30 {
			content = content[:30]+"..."
		}
		return ui.NewText(
			content,
			ui.TextParams{
				Color: tcell.ColorDarkBlue,
			})
	default:
		return ui.NewText(
			"Not listening to FIFO file... It is not possible to add context to this conversation. I'll implement error message another day 😴",
			ui.TextParams{
				Color: tcell.ColorDarkOrange,
			})
	}
}

func DrawScreen(app *AppState, screen tcell.Screen) {
	screen.Clear()

	stackElements := []ui.StackElement{
		ui.NewText(
			app.BuildChatHistory(),
			ui.TextParams{
				HeightMode: ui.HeightFillOrFit,
			}),
		BuildFifoFileElement(app),
		ui.NewText(
			app.UserPromptPrefixed(),
			ui.TextParams{},
			),
	}
	stack := ui.VerticalStack{Elements: stackElements}
	view := ui.View {Element: &stack}
	if app.FreeScrollMode {
		view.Mode = ui.ViewModeFree
		view.Yoffset = app.ScrollPosition
	} else {
		view.Mode = ui.ViewModeAutoCompute
	}

	view.Compute(screen)
	// This whole thing gives more responsability than required to that drawing function
	// I'll have to fix that someday
	if app.FreeScrollMode && view.AtBottom() {
		app.FreeScrollMode = false
		app.ScrollPosition = view.Yoffset
	} else {
		app.ScrollPosition = view.Yoffset
	}
	view.Draw(screen)

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
				case tcell.KeyUp:
					appEvTx <- AppEvent {Type: EvViewScrollUp}
				case tcell.KeyDown:
					appEvTx <- AppEvent {Type: EvViewScrollDown}
				case tcell.KeyRune:
					appEvTx <- AppEvent{Type: EvUserPromptInput, Rune: ev.(*tcell.EventKey).Rune()}
			}
		case *tcell.EventResize:
			appEvTx <- AppEvent{Type: EvTermResize}
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
	EvTermResize
	EvViewScrollUp
	EvViewScrollDown
	EvUserPromptInput
	EvUserPromptPop
	EvUserPromptSubmit
	EvLlmContentArrived
	EvLlmContentFinished
	EvFifoReceived
	EvFifoErr
)

func ListenToFifoFile(ctx context.Context, path string, evTx chan<- AppEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		f, err := os.OpenFile(path, os.O_RDONLY, os.ModeNamedPipe)
		if err != nil {
			evTx <- AppEvent{Type: EvFifoErr}
			return
		}

		readBuf := make([]byte, 0, 255)
		scannerDone := make(chan struct{})

		go func() {
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				readBuf = append(readBuf, scanner.Bytes()...)
				readBuf = append(readBuf, '\n')
			}
			scannerDone <- struct{}{}
		}()

		select {
		case <-ctx.Done():
			f.Close()
			<-scannerDone
			return
		case <-scannerDone:
			evTx <- AppEvent{Type: EvFifoReceived, Data: string(readBuf)}
			readBuf = readBuf[:0]
		}

		f.Close()
	}
}

func RunEventLoop(ctx context.Context, app *AppState, screen tcell.Screen, evRxTx chan AppEvent) {
	var evRx <-chan AppEvent = evRxTx
	// Allows the event loop to send transmitters to other parts of the app
	var evTx chan<- AppEvent = evRxTx

	if len(os.Args) > 1 {
		initalPrompt := strings.Builder{}
		initalPrompt.WriteString("Hello, ")
		initalPrompt.WriteString(strings.Join(os.Args[1:], " "))
		app.UserPromptSet(initalPrompt.String())
		app.ChatHistoryAppendUserPrompt()
		requestCtx, _ := context.WithCancel(ctx)
		UserPromptSubmit(
			requestCtx,
			app.ChatHistory(),
			app.Provider(),
			evTx,
			)
		app.UserPromptClear()
	}

	if fifo := app.NamedPipe(); fifo.Failure == 0 {
		ctx, _ := context.WithCancel(ctx)
		go ListenToFifoFile(ctx, fifo.Path, evTx)
	}

	DrawScreen(app, screen)

	for ev := range evRx {
		switch ev.Type {
		case EvQuit:
			return
		case EvTermResize:
			DrawScreen(app, screen)
		case EvViewScrollUp:
			app.FreeScrollMode = true
			app.ScrollPosition -= 1
			DrawScreen(app, screen)
		case EvViewScrollDown:
			app.FreeScrollMode = true
			app.ScrollPosition += 1
			DrawScreen(app, screen)
		case EvUserPromptInput:
			app.UserPromptAppendRune(ev.Rune)
		case EvUserPromptPop:
			app.UserPromptPop()
		case EvUserPromptSubmit:
			if app.UserPromptEmpty() {
				return
			}

			app.ChatHistoryAppendUserPrompt()
			requestCtx, _ := context.WithCancel(ctx)
			UserPromptSubmit(
				requestCtx,
				app.ChatHistory(),
				app.Provider(),
				evTx,
				)
			app.UserPromptClear()
		case EvLlmContentArrived:
			app.LlmResponsePush(ev.Data)
		case EvLlmContentFinished:
			app.LlmResponseFinalize()
		case EvFifoReceived:
			app.SetPipedContent(ev.Data)
		}

		DrawScreen(app, screen)
	}
}

func main() {
	stdinStat, _ := os.Stdin.Stat()
	pipedInput := ""
	if stdinStat.Mode() & os.ModeCharDevice == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal("Failed to read stdin piped input: ", err)
		}
		pipedInput = string(data)
	}

	screen, err := tcell.NewScreen();
	err = screen.Init();
	if err != nil {
		log.Fatal("Failed to create a screen: ", err);
	}
	defer screen.Fini();

	// This is where windows users will lack
	namedPipe := NamedPipeFile{}
	runtimeDir := xdg.RuntimeDir
	if runtimeDir != "" {
		fifoPath := runtimeDir + "/hello-llm"
		namedPipe.Path = fifoPath

		stats, err := os.Lstat(fifoPath)
		if err != nil {
			switch {
			case errors.Is(err, os.ErrNotExist):
				if err := syscall.Mkfifo(fifoPath, 0666); err != nil {
					namedPipe.Failure = NamedPipeFailureOther
				} else {
					// I KNOW ITS MORE RESPECTFUL TO THE USER'S FS TO DO THAT BUT IT SLOWS DOWN SUBSEQUENT STARTUPS
					// defer os.Remove(fifoPath)
				}
			case errors.Is(err, os.ErrPermission):
				namedPipe.Failure = NamedPipeFailureNotAllowed
			default:
				namedPipe.Failure = NamedPipeFailureOther
			}
		} else if (stats.Mode() & os.ModeNamedPipe) != os.ModeNamedPipe {
			// This branch is useful for debugging failure, just create a dir at XDG_RUNTIME_DIR called hello-llm
			namedPipe.Failure = NamedPipeFailureAlreadyExists
		} else if (stats.Mode() & os.ModeNamedPipe) == os.ModeNamedPipe {
			// Should we do anything here ?
			// Its unlikely that the user already has a FIFO file with the app's name being used for other purposes
			// It probably comes from a previous session
		}
	} else {
		namedPipe.Failure = NamedPipeFailureNoSuitablePath
	}

	app := NewAppState(providers.ProviderOpenai, namedPipe)
	if pipedInput != "" {
		app.ContextAppend(pipedInput)
	}

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
