package backend

import (
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
)

func TestOmitConstantCallback(t *testing.T) {
	tests := []struct {
		name     string
		mode     mode.Mode
		callback string
		value    valueNode
		want     bool
	}{
		{name: "play spawn order default", mode: mode.ModePlay, callback: "spawnOrder", value: 0, want: true},
		{name: "play spawn order nondefault", mode: mode.ModePlay, callback: "spawnOrder", value: 1, want: false},
		{name: "play should spawn default", mode: mode.ModePlay, callback: "shouldSpawn", value: 1, want: true},
		{name: "play should spawn false", mode: mode.ModePlay, callback: "shouldSpawn", value: 0, want: false},
		{name: "watch spawn time default", mode: mode.ModeWatch, callback: "spawnTime", value: 0, want: true},
		{name: "watch spawn time nondefault", mode: mode.ModeWatch, callback: "spawnTime", value: 1, want: false},
		{name: "watch despawn time default", mode: mode.ModeWatch, callback: "despawnTime", value: 0, want: true},
		{name: "watch despawn time nondefault", mode: mode.ModeWatch, callback: "despawnTime", value: -1, want: false},
		{name: "watch update spawn default", mode: mode.ModeWatch, callback: "updateSpawn", value: 0, want: true},
		{name: "watch update spawn nondefault", mode: mode.ModeWatch, callback: "updateSpawn", value: 3, want: false},
		{name: "ordinary callback", mode: mode.ModeWatch, callback: "initialize", value: 1, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := omitConstantCallback(test.mode, test.callback, test.value); got != test.want {
				t.Fatalf("omitConstantCallback(%s, %q, %v) = %v, want %v", test.mode, test.callback, test.value, got, test.want)
			}
		})
	}
}
