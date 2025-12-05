package ui

import (
	"github.com/gdamore/tcell/v2"
)

type ViewMode int

const (
	ViewModeAutoCompute ViewMode = iota
	ViewModeFree
)

type View struct {
	Element ViewElement
	Yoffset int
	Mode ViewMode

	atBottom bool
}

func (view *View) Draw(screen tcell.Screen) {
	contentHeight := view.Element.ComputeHeight(screen)
	_, screenHeight := screen.Size()
	if view.Mode == ViewModeAutoCompute {
		if contentHeight < screenHeight {
			view.Yoffset = 0
		} else {
			view.Yoffset = contentHeight - screenHeight
		}
	}

	if view.Yoffset >= contentHeight - screenHeight {
		view.atBottom = true
	}

	view.Element.Draw(screen, view.Yoffset)
}

func (view *View) AtBottom() bool {
	return view.atBottom
}

type ViewElement interface {
	ComputeHeight(screen tcell.Screen) int
	Draw(screen tcell.Screen, yOffset int)
}
