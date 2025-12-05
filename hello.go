package main

import (
	"io"
	"os"
	"fmt"
	"log"
	"flag"
	"bufio"
	"errors"
	"context"
	"strings"
	"syscall"

	"github.com/gdamore/tcell/v2"
	"github.com/adrg/xdg"

	"github.com/hello-llm-2/app"
	"github.com/hello-llm-2/providers"
	"github.com/hello-llm-2/ui"
)
const SystemPrompt string = "You are a helpful assistant prompted from a terminal shell. User expects straight to the point factual answers with minimal noise unless specified otherwise."

// Returns the new YOffset (if computed, else unchanged) and if the view is at the bottom or not
func DrawScreen(app *app.AppState, screen tcell.Screen) (int, bool) {
	screen.Clear()

	elements := []ui.StackElement{
		ui.NewText(
			app.ChatHistoryBuild(),
			ui.TextParams{
				HeightMode: ui.HeightFillOrFit,
			}),
		ui.BuildUserErrorUiElement(app.UserError),
		ui.BuildFifoFileUiElement(
			app.PipedContent(),
			app.NamedPipe().Path,
			app.NamedPipe().Failure != 0,
			),
		ui.NewText(
			app.UserPromptPrefixed(),
			ui.TextParams{},
			),
	}

	view := ui.View {
		Element: ui.NewVerticalStack(elements),
	}

	if app.FreeScrollMode {
		view.Mode = ui.ViewModeFree
		view.Yoffset = app.ScrollPosition
	} else {
		view.Mode = ui.ViewModeAutoCompute
	}

	view.Draw(screen)
	screen.Show()
	return view.Yoffset, view.AtBottom()
}

func UserPromptSubmit(ctx context.Context, msgs []providers.AgnosticConversationMessage, provider providers.Provider, allowWebSearch bool, evTx chan<- AppEvent) {
	streamingParams := providers.StreamingRequestParams {
		Messages: msgs,
		AllowWebSearch: allowWebSearch,
		OnChunkReceived: func(chunk string) {
			evTx <- AppEvent {Type: EvLlmContentArrived, Data: chunk}
		},
		OnStreamingEnd: func(_content string) {
			evTx <- AppEvent {Type: EvLlmContentFinished}
		},
		OnStreamingErr: func(err error) {
			evTx <- AppEvent {Type: EvAppShowUserErr, Error: err}
		},
	}

	go provider.StartStreamingRequest(ctx, streamingParams)
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

type AppEvent struct {
	Type AppEventType
	Rune rune
	Data string
	Error error
}

type AppEventType int
const (
	EvQuit AppEventType = iota
	EvAppShowUserErr
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

func RunEventLoop(ctx context.Context, app *app.AppState, args []string, screen tcell.Screen, evRxTx chan AppEvent) {
	var evRx <-chan AppEvent = evRxTx
	// Allows the event loop to send transmitters to other parts of the app
	var evTx chan<- AppEvent = evRxTx

	streamingContent := false

	var requestCancelFunc context.CancelFunc
	tryCancelRequest := func() bool {
		if requestCancelFunc != nil {
			requestCancelFunc()
			requestCancelFunc = nil
			return true
		}
		return false
	}

	submitPrompt := func() {
		if tryCancelRequest() {
			app.LlmResponseFinalize()
		}

		app.ChatHistoryAppendUserPrompt()
		var rCtx context.Context
		rCtx, requestCancelFunc = context.WithCancel(ctx)
		UserPromptSubmit(
			rCtx,
			app.ChatHistory(),
			app.Provider(),
			app.Cfg().AllowWebSearch,
			evTx,
			)
		app.UserPromptClear()
		streamingContent = true
	}

	if len(args) > 0 {
		initalPrompt := strings.Builder{}
		initalPrompt.WriteString("Hello, ")
		initalPrompt.WriteString(strings.Join(args, " "))
		app.UserPromptSet(initalPrompt.String())
		submitPrompt()
	}

	if fifo := app.NamedPipe(); fifo.Failure == 0 {
		fifoCtx, fifoCancel := context.WithCancel(ctx)
		go ListenToFifoFile(fifoCtx, fifo.Path, evTx)
		defer fifoCancel()
	}

	DrawScreen(app, screen)

	for ev := range evRx {
		switch ev.Type {
		case EvQuit:
			return
		case EvAppShowUserErr:
			if ev.Error != context.Canceled {
				app.UserError = ev.Error.Error()
			}
		case EvTermResize:
			// redraw -- Done below
		case EvViewScrollUp:
			app.FreeScrollMode = true
			app.ScrollPosition -= 1
		case EvViewScrollDown:
			app.FreeScrollMode = true
			app.ScrollPosition += 1
		case EvUserPromptInput:
			app.UserPromptAppendRune(ev.Rune)
		case EvUserPromptPop:
			app.UserPromptPop()
		case EvUserPromptSubmit:
			if app.UserPromptEmpty() {
				if !streamingContent {
					return
				} else if tryCancelRequest() {
					app.LlmResponseFinalize()
				}
			} else {
				submitPrompt()
			}
		case EvLlmContentArrived:
			app.LlmResponsePush(ev.Data)
		case EvLlmContentFinished:
			tryCancelRequest()
			app.LlmResponseFinalize()
			streamingContent = false
		case EvFifoReceived:
			app.PipedContentSet(ev.Data)
		}

		newYOffset, atBottom := DrawScreen(app, screen)
		if app.FreeScrollMode && atBottom {
			app.FreeScrollMode = false
		} 
		app.ScrollPosition = newYOffset
	}
}

var (
	ErrConfigCorrupted error = errors.New("Config corrupted")
	ErrConfigCreation error = errors.New("An unknown error occured while writing to config file")
)

func ReadConfig(cfg *app.AppConfig) error {
	cfgDir, err := os.UserConfigDir()
	cfgFile, err := os.Open(cfgDir + "/hello-llm.cfg")
	if err != nil {
		return err
	}
	defer cfgFile.Close()

	scanner := bufio.NewScanner(cfgFile)
	for scanner.Scan() {
		line := scanner.Text()
		key, value, found := strings.Cut(line, "=")
		if !found {
			return ErrConfigCorrupted
		}

		switch key {
		case "default_provider":
			var err error
			cfg.Provider, err = providers.ProviderTypeFromString(value)
			if err != nil {
				return ErrConfigCorrupted
			}
		}
	}

	return nil
}

func InitConfig(cfg *app.AppConfig) error {
	fmt.Println("You are seeing this screen because we need to build the default config for 'hello-llm'")
	fmt.Println("Please select your preferred default LLM provider:")
	fmt.Println("\t1. OpenAI (Uses OPENAI_API_KEY environment variable)")
	fmt.Println("\t2. Anthropic (Uses X environment variable)")
	fmt.Println("\t3. Google (Uses GEMINI_API_KEY environment variable)")
	fmt.Println("\t4. xAI (Uses GROK_API_KEY environment variable)")

	for {
		var choice int
		fmt.Print("> ")
		n, err := fmt.Scan(&choice)
		if err != nil || n != 1 {
			fmt.Println("Please enter a single number matching the selected provider")
			fmt.Scanln()
		}

		switch choice {
		case 1, 2, 4:
			cfg.Provider = providers.ProviderOpenai
		case 3:
			cfg.Provider = providers.ProviderGemini
		}
		break
	}

	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(cfgDir + "/hello-llm.cfg", os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return ErrConfigCreation
	}

	if _, err := f.WriteString("default_provider=" + providers.ProviderTypeToString(cfg.Provider)); err != nil {
		return ErrConfigCreation
	}

	return nil
}

func main() {
	cfg := app.AppConfig{}

	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.BoolVar(&cfg.AllowWebSearch, "w", false, "Allows the LLM to perform web searches. This may increase token usage and response time. The web search is performed on the provider's server unless using local models.")
	flagset.BoolVar(&cfg.AllowWebSearch, "web", false, "Allows the LLM to perform web searches. This may increase token usage and response time. The web search is performed on the provider's server unless using local models.")
	flagset.Parse(os.Args[1:])
	
	if err := ReadConfig(&cfg); err != nil {
		if os.IsNotExist(err) {
			err = InitConfig(&cfg)
		} else if errors.Is(err, ErrConfigCorrupted) {
			fmt.Fprintf(os.Stderr, "Failed to interpret config file. Has it been modified by a third party ?\n") 
			err = InitConfig(&cfg)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not create a config file... Aborting\n") 
			return
		}
	}

	// cfg can now be populated with cmd line args and other stuff

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
	runtimeDir := xdg.RuntimeDir
	if runtimeDir != "" {
		fifoPath := runtimeDir + "/hello-llm"
		cfg.NamedPipe.Path = fifoPath

		stats, err := os.Lstat(fifoPath)
		if err != nil {
			switch {
			case errors.Is(err, os.ErrNotExist):
				if err := syscall.Mkfifo(fifoPath, 0666); err != nil {
					cfg.NamedPipe.Failure = app.NamedPipeFailureOther
				} else {
					// I KNOW ITS MORE RESPECTFUL TO THE USER'S FS TO DO THAT BUT IT SLOWS DOWN SUBSEQUENT STARTUPS
					// defer os.Remove(fifoPath)
				}
			case errors.Is(err, os.ErrPermission):
				cfg.NamedPipe.Failure = app.NamedPipeFailureNotAllowed
			default:
				cfg.NamedPipe.Failure = app.NamedPipeFailureOther
			}
		} else if (stats.Mode() & os.ModeNamedPipe) != os.ModeNamedPipe {
			// This branch is useful for debugging failure, just create a dir at XDG_RUNTIME_DIR called hello-llm
			cfg.NamedPipe.Failure = app.NamedPipeFailureAlreadyExists
		} else if (stats.Mode() & os.ModeNamedPipe) == os.ModeNamedPipe {
			// Should we do anything here ?
			// Its unlikely that the user already has a FIFO file with the app's name being used for other purposes
			// It probably comes from a previous session
		}
	} else {
		cfg.NamedPipe.Failure = app.NamedPipeFailureNoSuitablePath
	}

	app := app.NewAppState(&cfg)
	if pipedInput != "" {
		app.ContextAppend(pipedInput)
	}

	tuiQuit := make(chan struct{})
	tuiEventsCh := make(chan tcell.Event)
	appEv := make(chan AppEvent, 50)

	go screen.ChannelEvents(tuiEventsCh, tuiQuit)
	defer close(tuiQuit)

	go ReceiveTuiEvent(tuiEventsCh, appEv)

	ctx, cancelCtx := context.WithCancel(context.Background())
	RunEventLoop(ctx, app, flagset.Args(), screen, appEv)
	defer cancelCtx()
}
