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

	heightComputed bool
	elementsHeights []int
	spacePerFiller int
}

type StackElement interface {
	ComputeHeight(screen tcell.Screen) int
	HeightMode() int
	Draw(screen tcell.Screen, y int) (int, int)
}

func (stack *VerticalStack) ComputeHeight(screen tcell.Screen) int {
	stack.heightComputed = true

	stack.elementsHeights = make([]int, 0, len(stack.Elements));
	numFillers := 0
	for _, el := range stack.Elements {
		stack.elementsHeights = append(
			stack.elementsHeights,
			el.ComputeHeight(screen),
			)

		if el.HeightMode() == HeightFillOrFit {
			numFillers += 1
		}
	}

	heightSum := 0
	for _, elH := range stack.elementsHeights {
		heightSum += elH
	}

	_, screenHeight := screen.Size()
	voidSpace := screenHeight - heightSum
	if voidSpace > 0 {
		if numFillers == 0 {
			stack.spacePerFiller = 0 // doesn't matter
		} else {
			stack.spacePerFiller = voidSpace / numFillers
		}

		return heightSum + stack.spacePerFiller * numFillers
	} else {
		stack.spacePerFiller = 0
		return heightSum
	}
}

func (stack *VerticalStack) Draw(screen tcell.Screen, yOffset int) {
	if !stack.heightComputed {
		stack.ComputeHeight(screen)
	}

	heightCursor := -yOffset
	for i, el := range stack.Elements {
		y := heightCursor

		el.Draw(screen, y) 

		elH := stack.elementsHeights[i]
		heightCursor += elH 
		if el.HeightMode() == HeightFillOrFit {
			heightCursor += stack.spacePerFiller
		}
	}
}
