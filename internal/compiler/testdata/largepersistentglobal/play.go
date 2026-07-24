//go:build play

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

const noteCapacity = 4096

type noteSimulateResult struct {
	NoteID          int
	Note            int
	State           int
	JudgementResult *noteSimulateJudgementResult
	Progress        float64
	Offset          int
	Override        int
}

type noteSimulateJudgementResult struct {
	Note                int
	Type                int
	Origin              int
	Converted           int
	Timing              int
	DirectionMismatch   bool
	Time                int
	Difference          int
	ConvertedDifference int
	EasyFlick           bool
}

type SimulatorMemory struct {
	sonolus.LevelMemoryResource
	Results    [noteCapacity]noteSimulateResult
	Judgements [noteCapacity]noteSimulateJudgementResult
}

var Memory = SimulatorMemory{}
