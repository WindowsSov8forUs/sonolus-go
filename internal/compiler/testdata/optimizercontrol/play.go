//go:build play

package main

import (
	"math"

	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/native"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Gauge struct{ play.Archetype }

type drawList struct {
	Count int
	Data  [448]float64
}

func (d *drawList) push(sprite, left, right, bottom, top float64) {
	i := d.Count * 7
	d.Data[i] = sprite
	d.Data[i+1] = left
	d.Data[i+2] = right
	d.Data[i+3] = bottom
	d.Data[i+4] = top
	d.Data[i+5] = 1
	d.Data[i+6] = 0
	d.Count++
}

func (*Gauge) Preprocess() {
	scale := native.Get(4001, 0)
	var list drawList
	for layer := 0; layer < 2; layer++ {
		width, height, border := 506.0, 28.0, 14.0
		if layer == 1 {
			width, height, border = 502, 24, 14/1.1666666269302368
		}
		x, y := width/2, height/2
		capX, capY := math.Min(border, x), math.Min(border, y)
		for row := 0; row < 3; row++ {
			bottom, top := -y, -y+capY
			if row == 1 {
				bottom, top = -y+capY, y-capY
			} else if row == 2 {
				bottom, top = y-capY, y
			}
			for column := 0; column < 3; column++ {
				left, right := -x, -x+capX
				if column == 1 {
					left, right = -x+capX, x-capX
				} else if column == 2 {
					left, right = x-capX, x
				}
				if right > left && top > bottom {
					list.push(float64(5556+layer*9+row*3+column), left*scale, right*scale, bottom*scale, top*scale)
				}
			}
		}
	}
	native.DebugLog(float64(list.Count))
	for i := 0; i < list.Count*7; i++ {
		native.DebugLog(list.Data[i])
	}
}

type Split struct{ play.Archetype }

func split(b float64) (float64, float64) {
	c := 4097 * b
	high := c - (c - b)
	return high, b - high
}

func (*Split) Preprocess() {
	constantHigh, constantLow := split(1000000)
	dynamicHigh, dynamicLow := split(native.Get(4001, 0))
	native.DebugLog(constantHigh)
	native.DebugLog(constantLow)
	native.DebugLog(dynamicHigh)
	native.DebugLog(dynamicLow)
}
