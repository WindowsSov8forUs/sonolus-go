package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type ConfigData struct {
	sonolus.Configuration
	UI sonolus.UIConfig
}

// The "time" metric (Sonolus app 1.1.3) is accepted for both metric slots.
var Config = ConfigData{
	UI: sonolus.UIConfig{PrimaryMetric: sonolus.UIMetricTime, SecondaryMetric: sonolus.UIMetricTime},
}

func main() {}
