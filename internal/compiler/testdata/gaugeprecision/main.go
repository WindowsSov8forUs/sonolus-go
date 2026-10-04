package main

import (
	"math"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/native"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type drawList struct {
	Count int
	Data  [448]float64
}
type state struct {
	sonolus.LevelMemoryResource
	List drawList
}

var memory = state{}

func (list *drawList) push(sprite int, q sonolus.Quad, order, alpha float64) {
	i := list.Count * 7
	list.Data[i] = float64(sprite)
	list.Data[i+1] = q.BL.X
	list.Data[i+2] = q.TR.X
	list.Data[i+3] = q.BL.Y
	list.Data[i+4] = q.TR.Y
	list.Data[i+5] = order
	list.Data[i+6] = alpha
	list.Count++
}

func gauge(list *drawList, x, y, scale float64) {
	list.Count = 0
	for layer := 0; layer < 2; layer++ {
		width, height, border, alpha := 506., 28., 14., 1.
		if layer == 1 {
			width, height, border, alpha = 502, 24, 14/1.1666666269302368, 178./255
		}
		capX, capY := math.Min(border, width/2), math.Min(border, height/2)
		for row := 0; row < 3; row++ {
			bottom, top := 0., capY
			if row == 1 {
				bottom, top = capY, height-capY
			} else if row == 2 {
				bottom, top = height-capY, height
			}
			for column := 0; column < 3; column++ {
				left, right := 0., capX
				if column == 1 {
					left, right = capX, width-capX
				} else if column == 2 {
					left, right = width-capX, width
				}
				if right > left && top > bottom {
					q := sonolus.RectFromCenter(sonolus.NewVec2(x+((left+right-width)/2)*scale, y+((bottom+top-height)/2)*scale), sonolus.NewVec2((right-left)*scale, (top-bottom)*scale)).ToQuad()
					list.push(5556+layer*9+row*3+column, q, 6200+float64(layer)*.5, alpha)
				}
			}
		}
	}
}

type Gauge struct{ play.Archetype }

func (*Gauge) Preprocess() {
	gauge(&memory.List, 0, 0, native.Get(4001, 0))
	native.DebugLog(float64(memory.List.Count))
	for i := 0; i < memory.List.Count; i++ {
		native.DebugLog(memory.List.Data[i*7])
	}
}

func productPair(a, b float64) (float64, float64) {
	product := a * b
	c := 4097 * a
	aHigh := c - (c - a)
	aLow := a - aHigh
	c = 4097 * b
	bHigh := c - (c - b)
	bLow := b - bHigh
	err1 := product - aHigh*bHigh
	err2 := err1 - aLow*bHigh
	err3 := err2 - aHigh*bLow
	return product, aLow*bLow - err3
}

type Product struct{ play.Archetype }

func (*Product) Preprocess() {
	a := native.Get(4001, 0) * 1.9092559814453125
	b := native.Get(4001, 0) * 1.9073486328125
	k := native.Get(4001, 0) * 500500
	ah, al := productPair(a, 1000000)
	bh, bl := productPair(b, k)
	native.DebugLog(ah * .5)
	native.DebugLog(al * .5)
	native.DebugLog(bh)
	native.DebugLog(bl)
}
func main() {}
