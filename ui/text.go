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
	lines []string
	linesCached bool
}

type TextParams struct {
	Anchor int
	HeightMode int
}

func NewText(content string, params TextParams) *Text {
	return &Text {
		buffer: content,
		params: params,
		lines: make([]string, 0, 2),
		linesCached: false,
	}
}

func (text *Text) ComputeHeight(screen tcell.Screen) int {
	screenWidth, _ := screen.Size()
	var lines []string
	var currentLine strings.Builder
	currentWidth := 0
	
	for _, r := range text.buffer {
		if r == '\n' {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentWidth = 0
			continue
		}

		runeWidth := runewidth.RuneWidth(r)

		// This word is going on the next line
		if currentWidth + runeWidth > screenWidth {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentWidth = 0
		}

		currentLine.WriteRune(r)
		currentWidth += runeWidth
	}

	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	// Cached for drawing later
	text.lines = lines
	text.linesCached = true

	return len(lines)
}

func (text *Text) HeightMode() int {
	return text.params.HeightMode
}

func (text *Text) Anchor() int {
	return text.params.Anchor
}

func (text *Text) Draw(screen tcell.Screen, y int) (int, int) {
	if !text.linesCached {
		text.ComputeHeight(screen)
	}

	lines := text.lines
	if text.params.Anchor == AnchorBottom {
		startY := y + len(lines)
		for i, line := range lines {
			y := startY + i
			if y < 0 { 
				continue 
			}
			screen.PutStr(0, y, line)
		}
	} else if text.params.Anchor == AnchorUp {
		for i, line := range lines {
			y := y + i
			if y < 0 {
				continue
			}
			screen.PutStr(0, y, line)
		}
	}

	return 0, 0
}
