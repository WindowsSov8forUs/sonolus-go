package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

//sonolus:resource skin
type SkinData struct {
	Note     sonolus.Sprite
	Fallback sonolus.Sprite
}

//sonolus:resource skin
var Skin = &SkinData{
	Note:     sonolus.SkinSprite("note"),
	Fallback: sonolus.SkinSprite("fallback"),
}

//sonolus:resource buckets
type BucketData struct {
	Tap sonolus.Bucket
}

//sonolus:resource buckets
var Buckets = &BucketData{
	Tap: sonolus.JudgmentBucket(
		"#MILLISECONDS",
		sonolus.JudgmentBucketSprite(Skin.Note, 0, 0, 2, 2, 0),
		sonolus.JudgmentBucketSpriteWithFallback(Skin.Note, Skin.Fallback, 1, 2, 3, 4, 5),
	),
}

func main() {}
