package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

type Buckets struct {
	sonolus.BucketsResource
	Note sonolus.Bucket
}

var BucketData = Buckets{Note: sonolus.JudgmentBucket("#MILLISECONDS")}

func (*Note) UpdateParallel() {
	play.UI.SetMenu(sonolus.RuntimeUILayout{})
	BucketData.Note.SetWindow(sonolus.JudgmentWindows{})
}

func main() {}
