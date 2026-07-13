package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

const (
	stageLeft      = -3.5
	stageRight     = 3.5
	judgmentLineY  = -0.75
	noteTravelTime = 2.0
)

func laneX(lane float64) float64 {
	return lane * Config.LaneWidth
}

func noteY(targetTime, now float64) float64 {
	return judgmentLineY + (targetTime-now)/noteTravelTime*1.6
}

func noteRect(lane, y float64) sonolus.Rect {
	halfWidth := 0.42 * Config.NoteSize * Config.LaneWidth
	halfHeight := 0.12 * Config.NoteSize
	center := laneX(lane)
	return sonolus.Rect{T: y + halfHeight, R: center + halfWidth, B: y - halfHeight, L: center - halfWidth}
}

func hitbox(lane float64) sonolus.Rect {
	center := laneX(lane)
	halfWidth := 0.5 * Config.LaneWidth
	return sonolus.Rect{T: judgmentLineY + 0.35, R: center + halfWidth, B: judgmentLineY - 0.35, L: center - halfWidth}
}

func stageRect() sonolus.Rect {
	return sonolus.Rect{T: 1, R: stageRight * Config.LaneWidth, B: -1, L: stageLeft * Config.LaneWidth}
}

func judgmentRect() sonolus.Rect {
	return sonolus.Rect{T: judgmentLineY + 0.025, R: stageRight * Config.LaneWidth, B: judgmentLineY - 0.025, L: stageLeft * Config.LaneWidth}
}
