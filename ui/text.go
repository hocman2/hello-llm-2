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
	linesBuilt bool
}

type TextParams struct {
	HeightMode int
	Color tcell.Color
	ColorForeground tcell.Color
}

func NewText(content string, params TextParams) *Text {
	return &Text {
		buffer: content,
		params: params,
		lines: make([]string, 0, 2),
		linesBuilt: false,
	}
}

func (text *Text) BuildLines(screen tcell.Screen) {
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

	text.lines = lines
	text.linesBuilt = true
}

func (text *Text) ComputeHeight(screen tcell.Screen, availableVoidSpace int) int {
	if !text.linesBuilt {
		text.BuildLines(screen)
	}
	switch text.params.HeightMode {
	case HeightFit:
		return len(text.lines)
	case HeightFillOrFit:
		return max(len(text.lines), availableVoidSpace)
	default:
		return len(text.lines)
	}
}

func (text *Text) HeightMode() int {
	return text.params.HeightMode
}

func (text *Text) Draw(screen tcell.Screen, y int) {
	screenW, _ := screen.Size()

	if !text.linesBuilt {
		text.BuildLines(screen)
	}

	style := tcell.StyleDefault.Background(text.params.Color) 
	style = style.Foreground(text.params.ColorForeground)
	lines := text.lines
	for i, line := range lines {
		lineY := y + i
		if lineY < 0 { 
			continue 
		}

		if len(line) < screenW {
			line = line + strings.Repeat(" ", screenW - len(line))
		}
		screen.PutStrStyled(0, lineY, line, style)
	}
}
