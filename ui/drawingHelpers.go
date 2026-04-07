package ui

import (
	"strings"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/hello-llm-2/providers"
)

func BuildChatHistory(messages []providers.AgnosticConversationMessage, currentResponse string, useColor bool) *VerticalStack {
	// Overallocating here
	elements := make([]StackElement, 0, len(messages) + 1)

	builder := strings.Builder{}
	for _, msg := range messages {
		params := TextParams{}

		switch msg.Type {
		case providers.MessageTypeUser:
			if useColor {
				params.ColorForeground = tcell.ColorDarkCyan
			}
			builder.WriteString("> ")
		case providers.MessageTypeUserContext, providers.MessageTypeSystem:
			continue
		}

		builder.WriteString(msg.Content)
		builder.WriteByte('\n')

		elements = append(
			elements,
			NewText(builder.String(), params),
			)
		builder.Reset()
	}

	if currentResponse != "" {
		elements = append(
			elements,
			NewText(currentResponse, TextParams{}),
			)
	}

	return NewVerticalStack(
		elements,
		VerticalStackParams {HeightFillOrFit},
		)
}

func BuildFifoFileUiElement(pipedContent string, pipePath string, pipeFailure bool) *Text {
	if pipeFailure {
		return NewText(
			"Not listening to FIFO file... It is not possible to add context to this conversation. I'll implement error message another day 😴",
			TextParams{
				Color: tcell.ColorDarkOrange,
				ColorForeground: tcell.ColorBlack,
			})
	} else {
		if pipedContent == "" {
			pipedContent = fmt.Sprintf("FIFO file listening. Writing to \"%s\" will add context to the conversation", pipePath)
		} else if len(pipedContent) > 30 {
			pipedContent = pipedContent[:30]+"..."
		}
		return NewText(
			pipedContent,
			TextParams{
				Color: tcell.ColorDarkBlue,
				ColorForeground: tcell.ColorWhite,
			})
	}
}

func BuildUserErrorUiElement(userError string) *Text {
	if userError != "" {
		return NewText(
			userError,
			TextParams{
				Color: tcell.ColorDarkRed,
				ColorForeground: tcell.ColorWhite,
			})
	} else {
		return nil
	}
}
