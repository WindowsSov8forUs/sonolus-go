package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type SkinData struct {
	sonolus.SkinResource

	Note     sonolus.Sprite
	Fallback sonolus.Sprite
}

var Skin = &SkinData{
	Note:     sonolus.SkinSprite("note"),
	Fallback: sonolus.SkinSprite("fallback"),
}

type BucketData struct {
	sonolus.BucketsResource

	Tap sonolus.Bucket
}

var Buckets = &BucketData{
	Tap: sonolus.JudgmentBucket(
		"#MILLISECONDS",
		sonolus.JudgmentBucketSprite(Skin.Note, 0, 0, 2, 2, 0),
		sonolus.JudgmentBucketSpriteWithFallback(Skin.Note, Skin.Fallback, 1, 2, 3, 4, 5),
	),
}

func main() {}
