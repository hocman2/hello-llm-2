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

	computed bool
	atBottom bool
}

// Computes some internal values for drawing correctly 
// The reason why it's a two step process is that you might want
// to check for new yOffset or if view is at bottom after computing
// Doing it that way is more explicit.
// Technically if you don't call Compute() it will be done before drawing anyway ...
func (view *View) Compute(screen tcell.Screen) {
	view.computed = true

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
}

func (view *View) Draw(screen tcell.Screen) {
	if !view.computed {
		view.Compute(screen)
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
