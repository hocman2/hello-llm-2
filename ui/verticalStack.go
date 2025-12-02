// A very very simple full screen vertical stack, only works for our use case, probably ... 

package ui

import (
	"github.com/gdamore/tcell/v2"
)

const (
	HeightFit int = iota
	HeightFillOrFit
)

type VerticalStack struct {
	Elements []StackElement
}

type StackElement interface {
	ComputeHeight(screen tcell.Screen) int
	HeightMode() int
	Draw(screen tcell.Screen, y int) (int, int)
}

func (stack VerticalStack) Draw(screen tcell.Screen) {
	_, screenHeight := screen.Size()

	elementsHeights := make([]int, 0, len(stack.Elements));
	numFillers := 0
	for _, el := range stack.Elements {
		elementsHeights = append(elementsHeights, el.ComputeHeight(screen))
		if el.HeightMode() == HeightFillOrFit {
			numFillers += 1
		}
	}

	requiresConflictResolve := false
	heightSum := 0
	for _, elH := range elementsHeights {
		heightSum += elH
		if heightSum > screenHeight {
			requiresConflictResolve = true
			break
		}
	}

	if !requiresConflictResolve {
		voidSpace := screenHeight - heightSum - 1

		var spacePerFiller int 
		if numFillers == 0 {
			spacePerFiller = 0 // doesn't matter
		} else {
			spacePerFiller = voidSpace / numFillers
		}

		heightCursor := 0
		for i, el := range stack.Elements {
			elH := elementsHeights[i]
			y := heightCursor

			el.Draw(screen, y) 

			heightCursor += elH 
			if el.HeightMode() == HeightFillOrFit {
				heightCursor += spacePerFiller + 1
			}
		}
	} else {
		heightCursor := screenHeight
		for i := len(stack.Elements) - 1; i >= 0; i-- {
			el := stack.Elements[i]
			elH := elementsHeights[i]
			
			y := heightCursor - elH 

			el.Draw(screen, y)

			heightCursor -= elH
			if heightCursor <= 0 {
				break
			}
		}
	}
}
