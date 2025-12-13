// A very very simple full screen vertical stack, only works for our use case, probably ... 

package ui

import (
	"reflect"
	"github.com/gdamore/tcell/v2"
)

const (
	HeightFit int = iota
	HeightFillOrFit
)

type VerticalStack struct {
	Elements []StackElement

	params VerticalStackParams
	heightComputed bool
	elementsHeights []int
	spacePerFiller int
}

type VerticalStackParams struct {
	HeightMode int
}

func NewVerticalStack(elems []StackElement, params VerticalStackParams) *VerticalStack {
	filteredElems := make([]StackElement, 0, len(elems))
	for _, el := range elems {
		if !reflect.ValueOf(el).IsNil() {
			filteredElems = append(filteredElems, el)
		}
	}

	return &VerticalStack{
		Elements: filteredElems,
		params: params,
	}
}

type StackElement interface {
	ComputeHeight(screen tcell.Screen, availableVoidSpace int) int
	HeightMode() int
	Draw(screen tcell.Screen, y int)
}

func (stack *VerticalStack) ComputeHeight(screen tcell.Screen, availableVoidSpace int) int {
	stack.heightComputed = true

	stack.elementsHeights = make([]int, len(stack.Elements));
	for i := range stack.elementsHeights {
		stack.elementsHeights[i] = -1
	}

	// First pass compute fixed height elements
	heightSumFixed := 0
	numFixedEl := 0
	for i, el := range stack.Elements {
		if el.HeightMode() != HeightFit {
			continue
		}
		
		h := el.ComputeHeight(screen, 0)
		stack.elementsHeights[i] = h
		heightSumFixed += h
		numFixedEl += 1
	}

	remainingVoidSpace := max(0, availableVoidSpace - heightSumFixed)
	numFillers := len(stack.Elements) - numFixedEl
	var spacePerFillerEl int
	if numFillers <= 0 {
		spacePerFillerEl = 0 // doesn't matter
	} else {
		spacePerFillerEl = remainingVoidSpace / numFillers
	}

	// Second pass compute filler heights
	heightSumFillers := 0
	for i, el := range stack.Elements {
		if el.HeightMode() != HeightFillOrFit {
			continue
		}

		h := el.ComputeHeight(screen, spacePerFillerEl)
		heightSumFillers += h
		stack.elementsHeights[i] = h
	}
	
	switch stack.params.HeightMode {
	case HeightFit:
		return heightSumFillers + heightSumFixed
	case HeightFillOrFit:
		return max(heightSumFillers + heightSumFixed, availableVoidSpace)
	default:
		return heightSumFillers + heightSumFixed
	}
}

func (stack *VerticalStack) HeightMode() int {
	return stack.params.HeightMode
}

func (stack *VerticalStack) Draw(screen tcell.Screen, y int) {
	if !stack.heightComputed {
		panic("Stack height must be computed before calling draw")
	}

	heightCursor := y
	for i, el := range stack.Elements {
		elY := heightCursor

		el.Draw(screen, elY) 

		elH := stack.elementsHeights[i]
		heightCursor += elH 
		if el.HeightMode() == HeightFillOrFit {
			heightCursor += stack.spacePerFiller
		}
	}
}
