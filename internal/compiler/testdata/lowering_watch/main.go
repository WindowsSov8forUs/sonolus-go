package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/watch"
)

type Note struct {
	watch.Archetype `archetype:"name=Note"`
}

type EffectsData struct {
	sonolus.EffectResource
	Hit sonolus.Clip
}

var Effects = &EffectsData{Hit: sonolus.EffectClip("hit")}

func (*Note) Preprocess() {
	result := watch.Entity.Result()
	result.TargetTime = watch.Time.Now()
	watch.Entity.SetResult(result)
	value := watch.Time.Delta() + watch.Time.Scaled() + watch.Time.Previous() + watch.Time.BeatToTime(1)
	_ = watch.Time.Skip()
	value += watch.Screen.Rect().Width() + watch.SafeArea.Rect().Width() + watch.Audio.Offset()
	watch.Audio.Play(Effects.Hit, 0)
	watch.Audio.PlayScheduled(Effects.Hit, 1, 0)
	background := watch.Background.Get()
	watch.Background.Set(background)
	_, _ = watch.Entity.Info(), watch.Entity.InfoAt(0)
	archetypeScore := watch.Score.Archetype(0)
	watch.Score.SetArchetype(0, archetypeScore)
	watch.Score.SetBase(watch.Score.Base())
	watch.Score.SetConsecutive(sonolus.JudgmentPerfect, watch.Score.Consecutive(sonolus.JudgmentPerfect))
	watch.Life.SetInitial(watch.Life.Initial())
	watch.Life.SetMax(watch.Life.Max())
	watch.Life.SetArchetype(0, watch.Life.Archetype(0))
	watch.Life.SetConsecutive(sonolus.JudgmentPerfect, watch.Life.Consecutive(sonolus.JudgmentPerfect))
	watch.Life.AddScheduled(1, 2)
	watch.UI.SetMenu(watch.UI.Menu())
	watch.UI.SetJudgment(watch.UI.Judgment())
	watch.UI.SetComboValue(watch.UI.ComboValue())
	watch.UI.SetComboText(watch.UI.ComboText())
	watch.UI.SetPrimaryMetricBar(watch.UI.PrimaryMetricBar())
	watch.UI.SetPrimaryMetricValue(watch.UI.PrimaryMetricValue())
	watch.UI.SetSecondaryMetricBar(watch.UI.SecondaryMetricBar())
	watch.UI.SetSecondaryMetricValue(watch.UI.SecondaryMetricValue())
	watch.UI.SetProgress(watch.UI.Progress())
	watch.UI.SetProgressGraph(watch.UI.ProgressGraph())
	value += watch.UI.MenuConfiguration().Scale + watch.UI.JudgmentConfiguration().Scale
	value += watch.UI.ComboConfiguration().Scale + watch.UI.PrimaryMetricConfiguration().Scale
	value += watch.UI.SecondaryMetricConfiguration().Scale + watch.UI.ProgressConfiguration().Scale
	_, _, _, _ = watch.Streams.Has(0, 1), watch.Streams.PreviousKey(0, 1), watch.Streams.NextKey(0, 1), watch.Streams.Value(0, 1)
	_, _ = watch.Input.Offset(), watch.Input.Judge(1, 1, sonolus.JudgmentWindows{})
	Effects.Hit.Play(0)
	_, _, _ = watch.Environment.Debug(), watch.Environment.AspectRatio(), watch.Environment.Replay()
	_ = watch.Replay.IsReplay()
	watch.Debug.Log(value)
	watch.Debug.Pause()
}

func main() {}
