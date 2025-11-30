// Defines a text meant to be rendered to terminal with wrapping and stuff like that

package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"

	"strings"
)

type Text struct {
	buffer string
	params TextParams
}

type TextParams struct {
	OffsetsUpwards bool
}

func NewText(content string, params TextParams) Text {
	return Text {
		buffer: content,
		params: params,
	}
}

func (text Text) Draw(screen tcell.Screen, y int) (int, int) {
	screenWidth, _ := screen.Size()
	cursorX, cursorY := 0, y
	var lines []string
	var currentLine strings.Builder
	currentWidth := 0
	
	for _, r := range text.buffer {
		runeWidth := runewidth.RuneWidth(r)

		// This word is going on the next line
		if currentWidth + runeWidth > screenWidth {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentWidth = 0
		}

		currentLine.WriteRune(r)
		currentWidth += runeWidth

		// This will be the last line's width at the end
		cursorX = currentWidth
	}

	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	if text.params.OffsetsUpwards {
		startY := y - len(lines) + 1
		for i, line := range lines {
			screen.PutStr(0, startY + i, line)
		}
	} else {
		startLine := max(0, len(lines)-screenHeight)
		for i, line := range lines[startLine:] {
			screen.PutStr(0, y + i, line)
		}
	}

	return cursorX, cursorY
}
