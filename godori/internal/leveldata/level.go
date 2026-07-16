package leveldata

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	level "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/level"
)

type stageData struct{}

type bpmChangeData struct {
	Beat float64 `level:"#BEAT"`
	BPM  float64 `level:"#BPM"`
}

type timescaleChangeData struct {
	Beat      float64 `level:"#BEAT"`
	Timescale float64 `level:"#TIMESCALE"`
}

type noteData struct {
	Beat      float64             `level:"#BEAT"`
	Lane      float64             `level:"lane"`
	Direction float64             `level:"direction"`
	Previous  level.Ref[noteData] `level:"prev,omitempty"`
	Next      level.Ref[noteData] `level:"next,omitempty"`
}

type connectorData struct {
	First  level.Ref[noteData] `level:"first"`
	Second level.Ref[noteData] `level:"second"`
}

var (
	stage                = level.MustDefine[stageData]("Stage")
	bpmChange            = level.MustDefine[bpmChangeData]("#BPM_CHANGE")
	timescaleChange      = level.MustDefine[timescaleChangeData]("#TIMESCALE_CHANGE")
	tapNote              = level.MustDefine[noteData]("TapNote")
	flickNote            = level.MustDefine[noteData]("FlickNote")
	directionalFlickNote = level.MustDefine[noteData]("DirectionalFlickNote")
	holdHeadNote         = level.MustDefine[noteData]("HoldHeadNote")
	holdAnchorNote       = level.MustDefine[noteData]("HoldAnchorNote")
	holdEndNote          = level.MustDefine[noteData]("HoldEndNote")
	holdFlickNote        = level.MustDefine[noteData]("HoldFlickNote")
	holdTickNote         = level.MustDefine[noteData]("HoldTickNote")
	holdConnector        = level.MustDefine[connectorData]("HoldConnector")
	simLine              = level.MustDefine[connectorData]("SimLine")
)

type chartNote struct {
	entity      *level.Entity[noteData]
	simEligible bool
}

func note(archetype level.Archetype[noteData], name string, beat, lane, direction float64, simEligible bool) chartNote {
	return chartNote{
		entity:      archetype.New(noteData{Beat: beat, Lane: lane, Direction: direction}).Named(name),
		simEligible: simEligible,
	}
}

func hold(name string, notes ...chartNote) []level.Item {
	sort.SliceStable(notes, func(i, j int) bool { return notes[i].entity.Data.Beat < notes[j].entity.Data.Beat })
	items := make([]level.Item, 0, len(notes)*2-1)
	for _, note := range notes {
		items = append(items, note.entity)
	}
	for index := 0; index+1 < len(notes); index++ {
		first, second := notes[index].entity, notes[index+1].entity
		first.Data.Next = second.Ref()
		second.Data.Previous = first.Ref()
		items = append(items, holdConnector.New(connectorData{First: first.Ref(), Second: second.Ref()}).Named(fmt.Sprintf("%s-connector-%c", name, 'a'+index)))
	}
	return items
}

func createSimLines(notes []chartNote) []level.Item {
	filtered := make([]chartNote, 0, len(notes))
	for _, note := range notes {
		if note.simEligible {
			filtered = append(filtered, note)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].entity.Data.Beat != filtered[j].entity.Data.Beat {
			return filtered[i].entity.Data.Beat < filtered[j].entity.Data.Beat
		}
		return filtered[i].entity.Data.Lane < filtered[j].entity.Data.Lane
	})
	items := []level.Item{}
	for index := 0; index+1 < len(filtered); index++ {
		first, second := filtered[index].entity, filtered[index+1].entity
		if first.Data.Beat != second.Data.Beat {
			continue
		}
		items = append(items, simLine.New(connectorData{First: first.Ref(), Second: second.Ref()}).Named(simLineName(len(items))))
	}
	return items
}

func simLineName(index int) string {
	return "sim-" + strconv.Itoa(index+1)
}

// Build constructs the checked-in Godori development LevelData.
func Build() (*resource.LevelData, error) {
	notes := []chartNote{
		note(tapNote, "tap-1", 2, -2, 0, true),
		note(tapNote, "tap-2", 4, 0, 0, true),
		note(tapNote, "tap-3", 6, 2, 0, true),
		note(tapNote, "tap-3-chord", 6, -2, 0, true),
		note(tapNote, "tap-3-center", 6, 0, 0, true),
		note(tapNote, "tap-4", 8, -1, 0, true),
		note(tapNote, "tap-5", 10, 1, 0, true),
		note(flickNote, "flick-1", 12, 0, 0, true),
		note(directionalFlickNote, "directional-flick-left", 14, -1, -2, true),
		note(directionalFlickNote, "directional-flick-right", 14, 1, 2, true),
	}
	firstHold := []chartNote{
		note(holdHeadNote, "hold-head-1", 16, -2, 0, true),
		note(holdTickNote, "hold-tick-early-1", 16.5, 0, 0, false),
		note(holdTickNote, "hold-tick-1", 17, 0, 0, false),
		note(holdTickNote, "hold-tick-middle-1", 17.5, 0, 0, false),
		note(holdAnchorNote, "hold-anchor-1", 18, 1, 0, false),
		note(holdAnchorNote, "hold-anchor-1b", 19, -1, 0, false),
		note(holdTickNote, "hold-tick-2", 19, 0, 0, false),
		note(holdTickNote, "hold-tick-late-1", 19.5, 0, 0, false),
		note(holdEndNote, "hold-end-1", 20, 2, 0, true),
	}
	secondHold := []chartNote{
		note(holdHeadNote, "hold-head-2", 22, 3, 0, true),
		note(holdAnchorNote, "hold-anchor-2", 23, -1, 0, false),
		note(holdTickNote, "hold-tick-3", 23.5, 0, 0, false),
		note(holdFlickNote, "hold-flick-2", 24, -3, 0, true),
	}
	allNotes := append(append(append([]chartNote{}, notes...), firstHold...), secondHold...)
	items := []level.Item{
		stage.New(stageData{}).Named("stage"),
		bpmChange.New(bpmChangeData{Beat: 0, BPM: 120}).Named("bpm"),
		timescaleChange.New(timescaleChangeData{Beat: 8, Timescale: 0.75}).Named("timescale"),
		bpmChange.New(bpmChangeData{Beat: 12, BPM: 180}).Named("bpm-fast"),
		timescaleChange.New(timescaleChangeData{Beat: 21, Timescale: 1.25}).Named("timescale-fast"),
	}
	for _, note := range notes {
		items = append(items, note.entity)
	}
	items = append(items, hold("hold-1", firstHold...)...)
	items = append(items, hold("hold-2", secondHold...)...)
	items = append(items, createSimLines(allNotes)...)
	return level.NewBuilder().Add(items...).Build()
}

// Marshal constructs and serializes the checked-in Godori development LevelData.
func Marshal() ([]byte, error) {
	data, err := Build()
	if err != nil {
		return nil, err
	}
	return level.Marshal(data)
}
