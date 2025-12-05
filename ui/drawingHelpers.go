package ui

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
)

func BuildFifoFileUiElement(pipedContent string, pipePath string, pipeFailure bool) *Text {
	if pipeFailure {
		return NewText(
			"Not listening to FIFO file... It is not possible to add context to this conversation. I'll implement error message another day 😴",
			TextParams{
				Color: tcell.ColorDarkOrange,
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
			})
	}
}

func BuildUserErrorUiElement(userError string) *Text {
	if userError != "" {
		return NewText(
			userError,
			TextParams{
				Color: tcell.ColorDarkRed,
			})
	} else {
		return nil
	}
}

