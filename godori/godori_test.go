package main

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/godori/internal/leveldata"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/optimize"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/level"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
)

var (
	standardArtifactsOnce sync.Once
	standardArtifacts     *compiler.Artifacts
	standardArtifactsErr  error
)

func TestGodoriCompilesAtEveryOptimizationLevel(t *testing.T) {
	nodeCounts := map[optimize.Level]int{}
	for _, optimization := range []optimize.Level{optimize.LevelMinimal, optimize.LevelFast, optimize.LevelStandard} {
		t.Run(optimization.String(), func(t *testing.T) {
			var artifacts *compiler.Artifacts
			if optimization == optimize.LevelStandard {
				artifacts = compileStandard(t)
			} else {
				var err error
				artifacts, err = compiler.NewCompiler(compiler.Options{Optimization: optimization}, ".").CompileAll()
				if err != nil {
					t.Fatal(err)
				}
			}
			assertArtifacts(t, artifacts)
			assertRuntimeContract(t, artifacts, optimization)
			nodeCounts[optimization] = len(artifacts.Play.Nodes) + len(artifacts.Watch.Nodes) + len(artifacts.Preview.Nodes) + len(artifacts.Tutorial.Nodes)
		})
	}
	if len(nodeCounts) == 3 && (nodeCounts[optimize.LevelStandard] >= nodeCounts[optimize.LevelMinimal] || nodeCounts[optimize.LevelStandard] >= nodeCounts[optimize.LevelFast]) {
		t.Fatalf("Standard node count = %d, want less than Minimal %d and Fast %d", nodeCounts[optimize.LevelStandard], nodeCounts[optimize.LevelMinimal], nodeCounts[optimize.LevelFast])
	}
}

func BenchmarkCompileAll(b *testing.B) {
	for _, optimization := range []optimize.Level{optimize.LevelMinimal, optimize.LevelFast, optimize.LevelStandard} {
		b.Run(optimization.String(), func(b *testing.B) {
			var nodes int
			for range b.N {
				artifacts, err := compiler.NewCompiler(compiler.Options{Optimization: optimization}, ".").CompileAll()
				if err != nil {
					b.Fatal(err)
				}
				nodes = len(artifacts.Play.Nodes) + len(artifacts.Watch.Nodes) + len(artifacts.Preview.Nodes) + len(artifacts.Tutorial.Nodes)
			}
			b.ReportMetric(float64(nodes), "nodes/op")
		})
	}
}

func TestGodoriCompilationIsDeterministic(t *testing.T) {
	first := compileStandard(t)
	second, err := compiler.NewCompiler(compiler.Options{}, ".").CompileAll()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("compilation is not deterministic")
	}
}

func TestGodoriSchema(t *testing.T) {
	schema, err := compiler.NewCompiler(compiler.Options{}, ".").Schema()
	if err != nil {
		t.Fatal(err)
	}
	want := &compiler.ProjectSchema{Archetypes: []compiler.ArchetypeSchema{
		{Name: "#BPM_CHANGE", Fields: []string{"#BEAT", "#BPM"}},
		{Name: "#TIMESCALE_CHANGE", Fields: []string{"#BEAT", "#TIMESCALE"}},
		{Name: "AccentTapNote", Fields: []string{"end_time", "#BEAT", "lane", "direction", "prev", "next"}},
		{Name: "DirectionalFlickNote", Fields: []string{"end_time", "#BEAT", "lane", "direction", "prev", "next"}},
		{Name: "FlickNote", Fields: []string{"end_time", "#BEAT", "lane", "direction", "prev", "next"}},
		{Name: "HoldAnchorNote", Fields: []string{"end_time", "#BEAT", "lane", "direction", "prev", "next"}},
		{Name: "HoldConnector", Fields: []string{"first", "second"}},
		{Name: "HoldEndNote", Fields: []string{"end_time", "#BEAT", "lane", "direction", "prev", "next"}},
		{Name: "HoldFlickNote", Fields: []string{"end_time", "#BEAT", "lane", "direction", "prev", "next"}},
		{Name: "HoldHeadNote", Fields: []string{"end_time", "#BEAT", "lane", "direction", "prev", "next"}},
		{Name: "HoldManager", Fields: []string{}},
		{Name: "HoldTickNote", Fields: []string{"end_time", "#BEAT", "lane", "direction", "prev", "next"}},
		{Name: "ScheduledLaneEffect", Fields: []string{}},
		{Name: "SimLine", Fields: []string{"first", "second"}},
		{Name: "Stage", Fields: []string{}},
		{Name: "TapNote", Fields: []string{"end_time", "#BEAT", "lane", "direction", "prev", "next"}},
	}}
	if !reflect.DeepEqual(schema, want) {
		t.Fatalf("schema = %#v, want %#v", schema, want)
	}
}

func TestGodoriDevelopmentLevel(t *testing.T) {
	artifacts := compileStandard(t)
	development, err := level.LoadDevelopment(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(development.Levels) != 1 {
		t.Fatalf("development levels = %d, want 1", len(development.Levels))
	}
	developmentLevel := development.Levels[0]
	if developmentLevel.Name != "dev" || developmentLevel.Title != "Dev Level" || developmentLevel.File == "" || len(developmentLevel.Data.Entities) != 42 {
		t.Fatalf("development level = %#v", development)
	}
	if err := level.Validate(developmentLevel.Data, artifacts); err != nil {
		t.Fatal(err)
	}
	assertDevelopmentChart(t, developmentLevel.Data)
	assertDevelopmentFlick(t, developmentLevel.Data)
	assertDevelopmentDirectionalFlick(t, developmentLevel.Data)
	assertDevelopmentHold(t, developmentLevel.Data)
	assertDevelopmentHoldConnector(t, developmentLevel.Data)
	assertDevelopmentSimLine(t, developmentLevel.Data)
	assertDevelopmentHoldTick(t, developmentLevel.Data)
	assertDevelopmentBPM(t, developmentLevel.Data)
	assertDevelopmentTimescale(t, developmentLevel.Data)
}

func TestGodoriDevelopmentLevelIsGeneratedDeterministically(t *testing.T) {
	generated, err := leveldata.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	checkedIn, err := os.ReadFile("dev-level.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, checkedIn) {
		t.Fatal("dev-level.json is stale; run go generate")
	}
}

func TestGodoriPreviewColumnGeometry(t *testing.T) {
	tests := []struct {
		time   float64
		column int
		y      float64
	}{
		{0, 0, previewYMin},
		{1, 0, 0},
		{2, 1, previewYMin},
		{7.999, 3, previewYMin + 1.999/2*(previewYMax-previewYMin)},
		{-0.5, 0, previewYMin + 1.5/2*(previewYMax-previewYMin)},
	}
	for _, test := range tests {
		if got := previewColumn(test.time); got != test.column {
			t.Fatalf("previewColumn(%v) = %d, want %d", test.time, got, test.column)
		}
		if got := previewTimeY(test.time); math.Abs(got-test.y) > 1e-9 {
			t.Fatalf("previewTimeY(%v) = %v, want %v", test.time, got, test.y)
		}
	}
	if got := previewTimeYInColumn(2, 0); math.Abs(got-previewYMax) > 1e-9 {
		t.Fatalf("column end y = %v, want %v", got, previewYMax)
	}
	if got := previewTimeYInColumn(2, 1); math.Abs(got-previewYMin) > 1e-9 {
		t.Fatalf("next column start y = %v, want %v", got, previewYMin)
	}
	if got := previewColumnsForDuration(10); got != 6 {
		t.Fatalf("previewColumnsForDuration(10) = %v, want 6", got)
	}
	if got := previewColumnWidth * float64(previewColumnsForDuration(10)); math.Abs(got-6.024) > 1e-9 {
		t.Fatalf("preview canvas width = %v, want 6.024", got)
	}
}

func TestGodoriNoteAlphaFadesAtLaneBounds(t *testing.T) {
	if noteFadeProgress(-1) != 0 || noteFadeProgress(laneTopY()) != 0 {
		t.Fatalf("note fade bounds = (%v, %v)", noteFadeProgress(-1), noteFadeProgress(laneTopY()))
	}
	if noteFadeProgress(-1+noteFadeLength) != 1 || noteFadeProgress(laneTopY()-noteFadeLength) != 1 {
		t.Fatalf("note fade interior = (%v, %v)", noteFadeProgress(-1+noteFadeLength), noteFadeProgress(laneTopY()-noteFadeLength))
	}
	if progress := noteFadeProgress(-1 + noteFadeLength/2); progress <= 0 || progress >= 1 {
		t.Fatalf("note fade progress = %v", progress)
	}
}

func TestGodoriLayerAndProjectionGeometry(t *testing.T) {
	if got := layerZ(layerNote, -2, 3.5); got != 309803.5 {
		t.Fatalf("note z = %v, want 309803.5", got)
	}
	if got := layerZ(layerArrow, 1, -0.25); got != 320099.75 {
		t.Fatalf("arrow z = %v, want 320099.75", got)
	}
	y, scale := stageProjectY(judgmentLineY)
	if math.Abs(y-judgmentLineScreenY) > 1e-9 || math.Abs(scale-1) > 1e-9 {
		t.Fatalf("judgment projection = (%v, %v)", y, scale)
	}
	farY, farScale := stageProjectY(laneTopY())
	if farY <= judgmentLineScreenY || farY >= vanishingPointY || farScale <= 0 || farScale >= 1 {
		t.Fatalf("far projection = (%v, %v)", farY, farScale)
	}
}

func TestGodoriSimultaneousHitboxGeometry(t *testing.T) {
	base := noteHitbox(0, 0)
	if math.Abs(base.L+1.25*laneWidth()) > 1e-9 || math.Abs(base.R-1.25*laneWidth()) > 1e-9 {
		t.Fatalf("base hitbox = %#v", base)
	}
	right := noteHitbox(1, 0)
	leftOverlap, rightOverlap := hitboxOverlap(0, 1, base, right)
	if math.Abs(leftOverlap) > 1e-9 || math.Abs(rightOverlap-1.5*laneWidth()) > 1e-9 {
		t.Fatalf("right overlap = (%v, %v)", leftOverlap, rightOverlap)
	}
	left := noteHitbox(-1, 0)
	leftOverlap, rightOverlap = hitboxOverlap(0, -1, base, left)
	if math.Abs(leftOverlap-1.5*laneWidth()) > 1e-9 || math.Abs(rightOverlap) > 1e-9 {
		t.Fatalf("left overlap = (%v, %v)", leftOverlap, rightOverlap)
	}
	directional := noteHitbox(0, 2)
	if math.Abs(directional.L-base.L) > 1e-9 || math.Abs(directional.R-(base.R+2*laneWidth())) > 1e-9 {
		t.Fatalf("directional hitbox = %#v", directional)
	}
}

func TestGodoriHoldTickResolution(t *testing.T) {
	tests := []struct {
		name              string
		best, target, now float64
		wantTime          float64
		wantReady         bool
	}{
		{name: "untouched before deadline", best: unsetJudgmentTime, target: 10, now: 10.15, wantTime: unsetJudgmentTime},
		{name: "untouched after deadline", best: unsetJudgmentTime, target: 10, now: 10.151, wantTime: unsetJudgmentTime, wantReady: true},
		{name: "early good can improve", best: 9.88, target: 10, now: 10.1, wantTime: 9.88},
		{name: "early good resolved", best: 9.88, target: 10, now: 10.121, wantTime: 9.88, wantReady: true},
		{name: "continuous hold grace", best: 9.98, target: 10, now: 10.021, wantTime: 10, wantReady: true},
		{name: "late hit resolves", best: 10.04, target: 10, now: 10.04, wantTime: 10.04, wantReady: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotTime, gotReady := holdTickResolution(test.best, test.target, test.now)
			if gotTime != test.wantTime || gotReady != test.wantReady {
				t.Fatalf("holdTickResolution(%v, %v, %v) = (%v, %v), want (%v, %v)", test.best, test.target, test.now, gotTime, gotReady, test.wantTime, test.wantReady)
			}
		})
	}
}

func TestGodoriTutorialPhaseTiming(t *testing.T) {
	want := []tutorialPhaseTimeline{
		{IntroEnd: 1.5, FallEnd: 3, FrozenEnd: 7, Hit: 7, End: 8.5},
		{IntroEnd: 1.5, FallEnd: 3, FrozenEnd: 9, Hit: 8.5, End: 10.5},
		{IntroEnd: 1.5, FallEnd: 3, FrozenEnd: 9, Hit: 8.5, End: 10.5},
		{IntroEnd: 1.5, FallEnd: 3, FrozenEnd: 7, Hit: 7, End: 8.5},
		{IntroEnd: 1.5, FallEnd: 7, FrozenEnd: 5.5, Hit: 5.5, End: 8.5},
		{IntroEnd: 1.5, FallEnd: 3, FrozenEnd: 7, Hit: 7, End: 8.5},
		{IntroEnd: 1.5, FallEnd: 3, FrozenEnd: 9, Hit: 8.5, End: 10.5},
	}
	for phase, expected := range want {
		if got := tutorialTimelineFor(phase); got != expected {
			t.Fatalf("phase %d timeline = %#v, want %#v", phase, got, expected)
		}
	}
	instant := 7.0
	if tutorialCrossed(instant, instant+0.1, instant) || tutorialCrossed(instant+0.1, instant+0.2, instant) {
		t.Fatal("tutorial instant was triggered more than once")
	}
	if !tutorialCrossed(instant-0.1, instant, instant) || !tutorialCrossed(instant-0.1, instant+0.1, instant) {
		t.Fatal("tutorial instant was not triggered when crossed")
	}
}

func compileStandard(t *testing.T) *compiler.Artifacts {
	t.Helper()
	standardArtifactsOnce.Do(func() {
		standardArtifacts, standardArtifactsErr = compiler.NewCompiler(compiler.Options{}, ".").CompileAll()
	})
	if standardArtifactsErr != nil {
		t.Fatal(standardArtifactsErr)
	}
	return standardArtifacts
}

func assertArtifacts(t *testing.T, artifacts *compiler.Artifacts) {
	t.Helper()
	if artifacts.Configuration == nil || len(artifacts.Configuration.Options) != 13 || artifacts.ROM != nil {
		t.Fatalf("shared artifacts are incomplete: configuration=%#v rom=%d", artifacts.Configuration, len(artifacts.ROM))
	}
	assertStaticResources(t, artifacts)
	assertArchetypeInventory(t, artifacts)
	assertDeclarationContract(t, artifacts)
	assertNodes(t, "play", artifacts.Play.Nodes)
	assertNodes(t, "watch", artifacts.Watch.Nodes)
	assertNodes(t, "preview", artifacts.Preview.Nodes)
	assertNodes(t, "tutorial", artifacts.Tutorial.Nodes)

	playStage, playNote := findPlay(artifacts.Play, "Stage"), findPlay(artifacts.Play, "TapNote")
	watchStage, watchNote := findWatch(artifacts.Watch, "Stage"), findWatch(artifacts.Watch, "TapNote")
	previewStage, previewNote := findPreview(artifacts.Preview, "Stage"), findPreview(artifacts.Preview, "TapNote")
	if playStage == nil || playNote == nil || watchStage == nil || watchNote == nil || previewStage == nil || previewNote == nil {
		t.Fatal("Stage and TapNote must exist in Play, Watch, and Preview")
	}
	if playStage.Preprocess == nil || playStage.UpdateSequential == nil || playStage.Touch == nil || playNote.Touch == nil ||
		watchStage.Preprocess == nil || watchNote.Terminate == nil || previewNote.Render == nil {
		t.Fatal("gameplay callbacks were omitted")
	}
	assertIndex(t, "play Stage preprocess", playStage.Preprocess.Index, len(artifacts.Play.Nodes))
	assertIndex(t, "play TapNote touch", playNote.Touch.Index, len(artifacts.Play.Nodes))
	assertIndex(t, "watch TapNote replay", watchNote.Terminate.Index, len(artifacts.Watch.Nodes))
	assertIndex(t, "watch updateSpawn", artifacts.Watch.UpdateSpawn, len(artifacts.Watch.Nodes))
	assertIndex(t, "preview TapNote render", previewNote.Render.Index, len(artifacts.Preview.Nodes))
	assertIndex(t, "tutorial preprocess", artifacts.Tutorial.Preprocess, len(artifacts.Tutorial.Nodes))
	assertIndex(t, "tutorial navigate", artifacts.Tutorial.Navigate, len(artifacts.Tutorial.Nodes))
	assertIndex(t, "tutorial update", artifacts.Tutorial.Update, len(artifacts.Tutorial.Nodes))
}

func assertArchetypeInventory(t *testing.T, artifacts *compiler.Artifacts) {
	t.Helper()
	playNames := make([]string, len(artifacts.Play.Archetypes))
	for i, archetype := range artifacts.Play.Archetypes {
		playNames[i] = string(archetype.Name)
	}
	watchNames := make([]string, len(artifacts.Watch.Archetypes))
	for i, archetype := range artifacts.Watch.Archetypes {
		watchNames[i] = string(archetype.Name)
	}
	previewNames := make([]string, len(artifacts.Preview.Archetypes))
	for i, archetype := range artifacts.Preview.Archetypes {
		previewNames[i] = string(archetype.Name)
	}
	slices.Sort(playNames)
	slices.Sort(watchNames)
	slices.Sort(previewNames)

	playAndPreview := []string{
		"#BPM_CHANGE", "#TIMESCALE_CHANGE", "AccentTapNote", "DirectionalFlickNote", "FlickNote", "HoldAnchorNote",
		"HoldConnector", "HoldEndNote", "HoldFlickNote", "HoldHeadNote", "HoldTickNote", "SimLine", "Stage", "TapNote",
	}
	wantPlay := append(slices.Clone(playAndPreview), "HoldManager")
	wantWatch := append(slices.Clone(playAndPreview), "HoldManager", "ScheduledLaneEffect")
	slices.Sort(wantPlay)
	slices.Sort(wantWatch)
	if !reflect.DeepEqual(playNames, wantPlay) {
		t.Fatalf("Play archetypes = %v, want %v", playNames, wantPlay)
	}
	if !reflect.DeepEqual(watchNames, wantWatch) {
		t.Fatalf("Watch archetypes = %v, want %v", watchNames, wantWatch)
	}
	if !reflect.DeepEqual(previewNames, playAndPreview) {
		t.Fatalf("Preview archetypes = %v, want %v", previewNames, playAndPreview)
	}
}

func assertStaticResources(t *testing.T, artifacts *compiler.Artifacts) {
	t.Helper()
	if artifacts.Configuration.UI.PrimaryMetric != "arcade" || artifacts.Configuration.UI.SecondaryMetric != "life" {
		t.Fatalf("unexpected UI metrics: %#v", artifacts.Configuration.UI)
	}
	if artifacts.Configuration.UI.JudgmentErrorStyle != "late" || artifacts.Configuration.UI.JudgmentErrorPlacement != "top" || artifacts.Configuration.UI.JudgmentErrorMin != 20 {
		t.Fatalf("unexpected judgment error UI: %#v", artifacts.Configuration.UI)
	}
	if artifacts.Configuration.UI.MenuVisibility.Scale != 1 || artifacts.Configuration.UI.MenuVisibility.Alpha != 1 || artifacts.Configuration.UI.JudgmentAnimation.Scale.Ease != resource.EngineConfigurationAnimationTweenEaseOutCubic || artifacts.Configuration.UI.ComboAnimation.Scale.Ease != resource.EngineConfigurationAnimationTweenEaseInCubic {
		t.Fatalf("unexpected default UI configuration: %#v", artifacts.Configuration.UI)
	}
	encodedConfiguration, err := json.Marshal(artifacts.Configuration)
	if err != nil {
		t.Fatalf("marshal configuration: %v", err)
	}
	var decodedConfiguration resource.EngineConfiguration
	if err := json.Unmarshal(encodedConfiguration, &decodedConfiguration); err != nil {
		t.Fatalf("configuration does not satisfy core wire schema: %v", err)
	}
	if len(artifacts.Play.Skin.Sprites) != 19 || len(artifacts.Play.Effect.Clips) != 8 || len(artifacts.Play.Particle.Effects) != 12 || len(artifacts.Play.Buckets) != 6 {
		t.Fatalf("unexpected Play resources: skin=%#v effect=%#v particle=%#v buckets=%#v", artifacts.Play.Skin, artifacts.Play.Effect, artifacts.Play.Particle, artifacts.Play.Buckets)
	}
	if artifacts.Play.Skin.RenderMode != resource.EngineRenderModeLightweight || artifacts.Play.Skin.Sprites[2].Name != sonolus.StandardSpriteNoteHeadCyan {
		t.Fatalf("unexpected Play skin: %#v", artifacts.Play.Skin)
	}
	if artifacts.Play.Skin.Sprites[3].Name != sonolus.StandardSpriteNoteHeadRed || artifacts.Play.Skin.Sprites[4].Name != sonolus.StandardSpriteDirectionalMarkerRed {
		t.Fatalf("unexpected Flick sprites: %#v", artifacts.Play.Skin.Sprites)
	}
	if got := clipNames(artifacts.Play.Effect.Clips); !reflect.DeepEqual(got, []string{
		sonolus.StandardClipStage, sonolus.StandardClipPerfect, sonolus.StandardClipGreat, sonolus.StandardClipGood,
		sonolus.StandardClipPerfectAlternative, sonolus.StandardClipGreatAlternative, sonolus.StandardClipGoodAlternative,
		sonolus.StandardClipHold,
	}) {
		t.Fatalf("unexpected Play effect order: %v", got)
	}
	if artifacts.Play.Effect.Clips[1].Name != sonolus.StandardClipPerfect ||
		artifacts.Play.Particle.Effects[0].Name != sonolus.StandardEffectNoteCircularTapCyan ||
		artifacts.Play.Particle.Effects[1].Name != sonolus.StandardEffectNoteCircularAlternativeRed {
		t.Fatalf("unexpected judgment resources: effect=%#v particle=%#v", artifacts.Play.Effect, artifacts.Play.Particle)
	}
	if artifacts.Play.Effect.Clips[7].Name != sonolus.StandardClipHold || len(artifacts.Watch.Effect.Clips) != 8 ||
		artifacts.Watch.Effect.Clips[7].Name != sonolus.StandardClipHold ||
		!reflect.DeepEqual(clipNames(artifacts.Play.Effect.Clips), clipNames(artifacts.Watch.Effect.Clips)) {
		t.Fatalf("unexpected Hold clips: play=%#v watch=%#v", artifacts.Play.Effect.Clips, artifacts.Watch.Effect.Clips)
	}
	if len(artifacts.Watch.Skin.Sprites) != 19 || len(artifacts.Watch.Particle.Effects) != 12 || len(artifacts.Watch.Buckets) != 6 ||
		len(artifacts.Preview.Skin.Sprites) != 22 || len(artifacts.Tutorial.Skin.Sprites) != 18 || len(artifacts.Tutorial.Particle.Effects) != 12 {
		t.Fatalf("Flick resources are incomplete across modes")
	}
	if artifacts.Preview.Skin.Sprites[10].Name != sonolus.StandardSpriteGridNeutral {
		t.Fatalf("unexpected Preview measure sprite: %#v", artifacts.Preview.Skin.Sprites[10])
	}
	if artifacts.Preview.Skin.Sprites[11].Name != sonolus.StandardSpriteGridCyan {
		t.Fatalf("unexpected Preview time sprite: %#v", artifacts.Preview.Skin.Sprites[11])
	}
	if len(artifacts.Tutorial.Effect.Clips) != 8 || len(artifacts.Tutorial.Instruction.Texts) != 6 || len(artifacts.Tutorial.Instruction.Icons) != 1 {
		t.Fatalf("unexpected Tutorial instructions: %#v", artifacts.Tutorial.Instruction)
	}
	if !reflect.DeepEqual(clipNames(artifacts.Tutorial.Effect.Clips), clipNames(artifacts.Play.Effect.Clips)) ||
		artifacts.Tutorial.Effect.Clips[7].Name != sonolus.StandardClipHold {
		t.Fatalf("unexpected Tutorial hold clip: %#v", artifacts.Tutorial.Effect.Clips)
	}
	if got := particleNames(artifacts.Tutorial.Particle.Effects); !reflect.DeepEqual(got, []string{
		sonolus.StandardEffectLaneLinear,
		sonolus.StandardEffectNoteLinearTapCyan, sonolus.StandardEffectNoteCircularTapCyan,
		sonolus.StandardEffectNoteLinearTapGreen, sonolus.StandardEffectNoteCircularTapGreen,
		sonolus.StandardEffectNoteCircularHoldGreen,
		sonolus.StandardEffectNoteLinearAlternativeRed, sonolus.StandardEffectNoteCircularAlternativeRed,
		sonolus.StandardEffectNoteLinearAlternativeYellow, sonolus.StandardEffectNoteCircularAlternativeYellow,
		sonolus.StandardEffectNoteLinearAlternativePurple, sonolus.StandardEffectNoteCircularAlternativePurple,
	}) {
		t.Fatalf("unexpected Tutorial particle order: %v", got)
	}
	tutorialTexts := make([]string, len(artifacts.Tutorial.Instruction.Texts))
	for i, instruction := range artifacts.Tutorial.Instruction.Texts {
		tutorialTexts[i] = string(instruction.Name)
		if instruction.ID != i {
			t.Fatalf("Tutorial instruction %q has ID %d, want %d", instruction.Name, instruction.ID, i)
		}
	}
	if !reflect.DeepEqual(tutorialTexts, []string{"#TAP", "#TAP_FLICK", "#TAP_HOLD", "#HOLD_FOLLOW", "#RELEASE", "#FLICK"}) {
		t.Fatalf("unexpected Tutorial instruction texts: %v", tutorialTexts)
	}
	wantBuckets := [][]resource.EngineDataBucketSprite{
		{{ID: 2, X: 0, Y: 0, W: 2, H: 2, Rotation: -90}},
		{{ID: 11, X: 0.5, Y: 0, W: 2, H: 5, Rotation: -90}, {ID: 9, X: -2, Y: 0, W: 2, H: 2, Rotation: -90}},
		{{ID: 11, X: -0.5, Y: 0, W: 2, H: 5, Rotation: -90}, {ID: 10, X: 2, Y: 0, W: 2, H: 2, Rotation: -90}},
		{{ID: 11, X: 0, Y: 0, W: 2, H: 5.5, Rotation: -90}, {ID: 13, X: 0, Y: 0, W: 2, H: 2, Rotation: -90}},
		{{ID: 3, X: 0, Y: 0, W: 2, H: 2, Rotation: -90}, {ID: 4, X: 1, Y: 0, W: 2, H: 2, Rotation: -90}},
		{{ID: 5, X: 2, Y: 0, W: 2, H: 2, Rotation: -90}, {ID: 7, X: -2, Y: 0, W: 2, H: 2, Rotation: 90}, {ID: 6, X: 3, Y: 0, W: 2, H: 2, Rotation: -90}, {ID: 8, X: -3, Y: 0, W: 2, H: 2, Rotation: 90}},
	}
	for i, want := range wantBuckets {
		if artifacts.Play.Buckets[i].Unit != "#MILLISECONDS" || !reflect.DeepEqual(artifacts.Play.Buckets[i].Sprites, want) {
			t.Fatalf("Play bucket %d = %#v, want sprites %#v", i, artifacts.Play.Buckets[i], want)
		}
		if !reflect.DeepEqual(artifacts.Watch.Buckets[i], artifacts.Play.Buckets[i]) {
			t.Fatalf("Watch bucket %d = %#v, want Play bucket %#v", i, artifacts.Watch.Buckets[i], artifacts.Play.Buckets[i])
		}
	}
	if artifacts.Play.Skin.Sprites[12].Name != sonolus.StandardSpriteSimultaneousConnectionNeutralSeamless {
		t.Fatalf("unexpected SimLine sprite: %#v", artifacts.Play.Skin.Sprites[12])
	}
	if artifacts.Play.Skin.Sprites[14].Name != sonolus.StandardSpriteStageMiddle ||
		artifacts.Play.Skin.Sprites[17].Name != sonolus.StandardSpriteNoteSlot ||
		artifacts.Play.Skin.Sprites[18].Name != sonolus.StandardSpriteStageCover ||
		artifacts.Play.Particle.Effects[5].Name != sonolus.StandardEffectLaneLinear {
		t.Fatalf("unexpected Stage resources: skin=%#v particle=%#v", artifacts.Play.Skin, artifacts.Play.Particle)
	}
	if artifacts.Play.Particle.Effects[4].Name != sonolus.StandardEffectNoteCircularTapGreen ||
		artifacts.Play.Particle.Effects[6].Name != sonolus.StandardEffectNoteLinearTapCyan ||
		artifacts.Play.Particle.Effects[10].Name != sonolus.StandardEffectNoteLinearTapGreen ||
		artifacts.Play.Particle.Effects[11].Name != sonolus.StandardEffectNoteCircularHoldGreen {
		t.Fatalf("unexpected dual note particles: %#v", artifacts.Play.Particle.Effects)
	}

	encoded, err := json.Marshal(artifacts.Configuration.Options)
	if err != nil {
		t.Fatal(err)
	}
	var options []struct {
		Name  string  `json:"name"`
		Type  string  `json:"type"`
		Def   float64 `json:"def"`
		Min   float64 `json:"min"`
		Max   float64 `json:"max"`
		Step  float64 `json:"step"`
		Unit  string  `json:"unit"`
		Scope string  `json:"scope"`
	}
	if err := json.Unmarshal(encoded, &options); err != nil {
		t.Fatal(err)
	}
	want := []struct {
		Name  string  `json:"name"`
		Type  string  `json:"type"`
		Def   float64 `json:"def"`
		Min   float64 `json:"min"`
		Max   float64 `json:"max"`
		Step  float64 `json:"step"`
		Unit  string  `json:"unit"`
		Scope string  `json:"scope"`
	}{
		{Name: "#SPEED", Type: "slider", Def: 1, Min: 0.5, Max: 2, Step: 0.05, Unit: "#PERCENTAGE"},
		{Name: "#NOTE_SPEED", Type: "slider", Def: 10, Min: 1, Max: 20, Step: 0.05, Scope: "godori"},
		{Name: "#NOTE_SIZE", Type: "slider", Def: 1, Min: 0.1, Max: 2, Step: 0.05, Unit: "#PERCENTAGE", Scope: "godori"},
		{Name: "#LANE_SIZE", Type: "slider", Def: 1, Min: 0.1, Max: 1.5, Step: 0.05, Unit: "#PERCENTAGE", Scope: "godori"},
		{Name: "Lane Length", Type: "slider", Def: 0.8, Min: 0.1, Max: 1, Step: 0.05, Unit: "#PERCENTAGE", Scope: "godori"},
		{Name: "#CONNECTOR_ALPHA", Type: "slider", Def: 0.8, Min: 0.1, Max: 1, Step: 0.05, Unit: "#PERCENTAGE", Scope: "godori"},
		{Name: "#NOTE_EFFECT", Type: "toggle", Def: 1, Scope: "godori"},
		{Name: "#LANE_EFFECT", Type: "toggle", Def: 1, Scope: "godori"},
		{Name: "#SIMLINE", Type: "toggle", Def: 1, Scope: "godori"},
		{Name: "#SIMLINE_ALPHA", Type: "slider", Def: 0.5, Min: 0.1, Max: 1, Step: 0.05, Unit: "#PERCENTAGE", Scope: "godori"},
		{Name: "#EFFECT", Type: "toggle", Def: 1, Scope: "godori"},
		{Name: "#EFFECT_AUTO", Type: "toggle", Scope: "godori"},
		{Name: "#MIRROR", Type: "toggle", Scope: "godori"},
	}
	if !reflect.DeepEqual(options, want) {
		t.Fatalf("configuration options = %#v, want %#v", options, want)
	}
}

func assertDeclarationContract(t *testing.T, artifacts *compiler.Artifacts) {
	t.Helper()
	playStage, playNote := findPlay(artifacts.Play, "Stage"), findPlay(artifacts.Play, "TapNote")
	watchNote := findWatch(artifacts.Watch, "TapNote")
	previewStage := findPreview(artifacts.Preview, "Stage")
	previewNote := findPreview(artifacts.Preview, "TapNote")
	playFlick := findPlay(artifacts.Play, "FlickNote")
	watchFlick := findWatch(artifacts.Watch, "FlickNote")
	previewFlick := findPreview(artifacts.Preview, "FlickNote")
	playDirectional := findPlay(artifacts.Play, "DirectionalFlickNote")
	watchDirectional := findWatch(artifacts.Watch, "DirectionalFlickNote")
	previewDirectional := findPreview(artifacts.Preview, "DirectionalFlickNote")
	playHold := findPlay(artifacts.Play, "HoldHeadNote")
	watchHold := findWatch(artifacts.Watch, "HoldHeadNote")
	previewHold := findPreview(artifacts.Preview, "HoldHeadNote")
	playHoldFlick := findPlay(artifacts.Play, "HoldFlickNote")
	watchHoldFlick := findWatch(artifacts.Watch, "HoldFlickNote")
	previewHoldFlick := findPreview(artifacts.Preview, "HoldFlickNote")
	playAnchor, watchAnchor, previewAnchor := findPlay(artifacts.Play, "HoldAnchorNote"), findWatch(artifacts.Watch, "HoldAnchorNote"), findPreview(artifacts.Preview, "HoldAnchorNote")
	playEnd, watchEnd, previewEnd := findPlay(artifacts.Play, "HoldEndNote"), findWatch(artifacts.Watch, "HoldEndNote"), findPreview(artifacts.Preview, "HoldEndNote")
	playManager := findPlay(artifacts.Play, "HoldManager")
	watchManager := findWatch(artifacts.Watch, "HoldManager")
	watchLaneEffect := findWatch(artifacts.Watch, "ScheduledLaneEffect")
	playConnector := findPlay(artifacts.Play, "HoldConnector")
	watchConnector := findWatch(artifacts.Watch, "HoldConnector")
	previewConnector := findPreview(artifacts.Preview, "HoldConnector")
	playSim := findPlay(artifacts.Play, "SimLine")
	watchSim := findWatch(artifacts.Watch, "SimLine")
	previewSim := findPreview(artifacts.Preview, "SimLine")
	playTick := findPlay(artifacts.Play, "HoldTickNote")
	watchTick := findWatch(artifacts.Watch, "HoldTickNote")
	previewTick := findPreview(artifacts.Preview, "HoldTickNote")
	if playNote == nil || watchNote == nil || previewNote == nil || playFlick == nil || watchFlick == nil || previewFlick == nil ||
		playDirectional == nil || watchDirectional == nil || previewDirectional == nil || playHold == nil || watchHold == nil || previewHold == nil ||
		playHoldFlick == nil || watchHoldFlick == nil || previewHoldFlick == nil ||
		playConnector == nil || watchConnector == nil || previewConnector == nil ||
		playAnchor == nil || watchAnchor == nil || previewAnchor == nil || playEnd == nil || watchEnd == nil || previewEnd == nil || playManager == nil || watchManager == nil ||
		playSim == nil || watchSim == nil || previewSim == nil || playTick == nil || watchTick == nil || previewTick == nil ||
		playStage == nil || previewStage == nil || watchLaneEffect == nil {
		t.Fatal("TapNote declaration is missing from one or more modes")
	}
	if !playNote.HasInput || !playFlick.HasInput || playStage.HasInput || playNote.Preprocess.Order != 0 ||
		playStage.UpdateSequential.Order != -1 || playStage.Touch.Order != 2 {
		t.Fatalf("unexpected Play archetype metadata: stage=%#v note=%#v", playStage, playNote)
	}
	playBasicImports := []resource.EngineArchetypeDataName{"#BEAT", "lane", "direction", "prev", "next"}
	watchBasicImports := []resource.EngineArchetypeDataName{"#BEAT", "lane", "direction", "prev", "next", "#JUDGMENT", "#ACCURACY", "end_time"}
	if got := importNames(playNote.Imports); !reflect.DeepEqual(got, playBasicImports) {
		t.Fatalf("Play TapNote imports = %v", got)
	}
	if !reflect.DeepEqual(playNote.Exports, []resource.EngineArchetypeDataName{"end_time"}) {
		t.Fatalf("Play TapNote exports = %v", playNote.Exports)
	}
	if got := importNames(watchNote.Imports); !reflect.DeepEqual(got, watchBasicImports) {
		t.Fatalf("Watch TapNote imports = %v", got)
	}
	if got := importNames(previewNote.Imports); !reflect.DeepEqual(got, playBasicImports) {
		t.Fatalf("Preview TapNote imports = %v", got)
	}
	if got := importNames(playFlick.Imports); !reflect.DeepEqual(got, playBasicImports) {
		t.Fatalf("Play FlickNote imports = %v", got)
	}
	if got := importNames(watchFlick.Imports); !reflect.DeepEqual(got, watchBasicImports) {
		t.Fatalf("Watch FlickNote imports = %v", got)
	}
	if got := importNames(previewFlick.Imports); !reflect.DeepEqual(got, playBasicImports) {
		t.Fatalf("Preview FlickNote imports = %v", got)
	}
	if !playDirectional.HasInput {
		t.Fatal("Play DirectionalFlickNote must have input")
	}
	if got := importNames(playDirectional.Imports); !reflect.DeepEqual(got, playBasicImports) {
		t.Fatalf("Play DirectionalFlickNote imports = %v", got)
	}
	if got := importNames(watchDirectional.Imports); !reflect.DeepEqual(got, watchBasicImports) {
		t.Fatalf("Watch DirectionalFlickNote imports = %v", got)
	}
	if got := importNames(previewDirectional.Imports); !reflect.DeepEqual(got, playBasicImports) {
		t.Fatalf("Preview DirectionalFlickNote imports = %v", got)
	}
	if !playHold.HasInput {
		t.Fatal("Play HoldNote must have input")
	}
	if got := importNames(playHold.Imports); !reflect.DeepEqual(got, playBasicImports) {
		t.Fatalf("Play HoldHeadNote imports = %v", got)
	}
	if got := importNames(watchHold.Imports); !reflect.DeepEqual(got, watchBasicImports) {
		t.Fatalf("Watch HoldHeadNote imports = %v", got)
	}
	if got := importNames(previewHold.Imports); !reflect.DeepEqual(got, playBasicImports) {
		t.Fatalf("Preview HoldHeadNote imports = %v", got)
	}
	if !playHoldFlick.HasInput {
		t.Fatal("Play HoldFlickNote must have input")
	}
	if got := importNames(playHoldFlick.Imports); !reflect.DeepEqual(got, playBasicImports) {
		t.Fatalf("Play HoldFlickNote imports = %v", got)
	}
	if got := importNames(watchHoldFlick.Imports); !reflect.DeepEqual(got, watchBasicImports) {
		t.Fatalf("Watch HoldFlickNote imports = %v", got)
	}
	if got := importNames(previewHoldFlick.Imports); !reflect.DeepEqual(got, playBasicImports) {
		t.Fatalf("Preview HoldFlickNote imports = %v", got)
	}
	for _, check := range []struct {
		name    string
		imports []resource.EngineDataArchetypeImport
		want    []resource.EngineArchetypeDataName
	}{
		{"Play HoldAnchorNote", playAnchor.Imports, playBasicImports},
		{"Watch HoldAnchorNote", watchAnchor.Imports, watchBasicImports},
		{"Preview HoldAnchorNote", previewAnchor.Imports, playBasicImports},
	} {
		if got := importNames(check.imports); !reflect.DeepEqual(got, check.want) {
			t.Fatalf("%s imports = %v", check.name, got)
		}
	}
	if !playEnd.HasInput {
		t.Fatal("Play HoldEndNote must have input")
	}
	if got := importNames(playEnd.Imports); !reflect.DeepEqual(got, playBasicImports) {
		t.Fatalf("Play HoldEndNote imports = %v", got)
	}
	if got := importNames(watchEnd.Imports); !reflect.DeepEqual(got, watchBasicImports) {
		t.Fatalf("Watch HoldEndNote imports = %v", got)
	}
	if got := importNames(previewEnd.Imports); !reflect.DeepEqual(got, playBasicImports) {
		t.Fatalf("Preview HoldEndNote imports = %v", got)
	}
	connectorImports := []resource.EngineArchetypeDataName{"first", "second"}
	if got := importNames(playConnector.Imports); !reflect.DeepEqual(got, connectorImports) {
		t.Fatalf("Play HoldConnector imports = %v", got)
	}
	if got := importNames(watchConnector.Imports); !reflect.DeepEqual(got, connectorImports) {
		t.Fatalf("Watch HoldConnector imports = %v", got)
	}
	if got := importNames(previewConnector.Imports); !reflect.DeepEqual(got, connectorImports) {
		t.Fatalf("Preview HoldConnector imports = %v", got)
	}
	if len(playManager.Imports) != 0 {
		t.Fatalf("Play HoldManager imports = %v", importNames(playManager.Imports))
	}
	if !playManager.HasInput || playManager.Touch == nil || playManager.Touch.Order != 1 {
		t.Fatalf("unexpected Play HoldManager input metadata: %#v", playManager)
	}
	if len(watchManager.Imports) != 0 {
		t.Fatalf("Watch HoldManager imports = %v", importNames(watchManager.Imports))
	}
	if len(watchLaneEffect.Imports) != 0 {
		t.Fatalf("Watch ScheduledLaneEffect imports = %v", importNames(watchLaneEffect.Imports))
	}
	if previewStage.Preprocess.Order != 1 {
		t.Fatalf("Preview Stage preprocess order = %d, want 1", previewStage.Preprocess.Order)
	}
	simImports := []resource.EngineArchetypeDataName{"first", "second"}
	if got := importNames(playSim.Imports); !reflect.DeepEqual(got, simImports) {
		t.Fatalf("Play SimLine imports = %v", got)
	}
	if got := importNames(watchSim.Imports); !reflect.DeepEqual(got, simImports) {
		t.Fatalf("Watch SimLine imports = %v", got)
	}
	if got := importNames(previewSim.Imports); !reflect.DeepEqual(got, simImports) {
		t.Fatalf("Preview SimLine imports = %v", got)
	}
	tickImports := playBasicImports
	if got := importNames(playTick.Imports); !reflect.DeepEqual(got, tickImports) {
		t.Fatalf("Play HoldTickNote imports = %v", got)
	}
	if !playTick.HasInput || playTick.Touch == nil {
		t.Fatalf("unexpected Play HoldTickNote input metadata: %#v", playTick)
	}
	if got := importNames(watchTick.Imports); !reflect.DeepEqual(got, watchBasicImports) {
		t.Fatalf("Watch HoldTickNote imports = %v", got)
	}
	if got := importNames(previewTick.Imports); !reflect.DeepEqual(got, tickImports) {
		t.Fatalf("Preview HoldTickNote imports = %v", got)
	}
}

func assertRuntimeContract(t *testing.T, artifacts *compiler.Artifacts, optimization optimize.Level) {
	t.Helper()
	playStage, playNote := findPlay(artifacts.Play, "Stage"), findPlay(artifacts.Play, "TapNote")
	watchStage, watchNote := findWatch(artifacts.Watch, "Stage"), findWatch(artifacts.Watch, "TapNote")
	previewBPM, previewNote := findPreview(artifacts.Preview, "#BPM_CHANGE"), findPreview(artifacts.Preview, "TapNote")
	previewTimescale := findPreview(artifacts.Preview, "#TIMESCALE_CHANGE")
	previewStage := findPreview(artifacts.Preview, "Stage")
	playFlick := findPlay(artifacts.Play, "FlickNote")
	watchFlick := findWatch(artifacts.Watch, "FlickNote")
	previewFlick := findPreview(artifacts.Preview, "FlickNote")
	playDirectional := findPlay(artifacts.Play, "DirectionalFlickNote")
	watchDirectional := findWatch(artifacts.Watch, "DirectionalFlickNote")
	previewDirectional := findPreview(artifacts.Preview, "DirectionalFlickNote")
	playHold := findPlay(artifacts.Play, "HoldHeadNote")
	watchHold := findWatch(artifacts.Watch, "HoldHeadNote")
	previewHold := findPreview(artifacts.Preview, "HoldHeadNote")
	playHoldFlick := findPlay(artifacts.Play, "HoldFlickNote")
	watchHoldFlick := findWatch(artifacts.Watch, "HoldFlickNote")
	previewHoldFlick := findPreview(artifacts.Preview, "HoldFlickNote")
	playEnd := findPlay(artifacts.Play, "HoldEndNote")
	watchEnd := findWatch(artifacts.Watch, "HoldEndNote")
	previewEnd := findPreview(artifacts.Preview, "HoldEndNote")
	playManager := findPlay(artifacts.Play, "HoldManager")
	watchManager := findWatch(artifacts.Watch, "HoldManager")
	watchLaneEffect := findWatch(artifacts.Watch, "ScheduledLaneEffect")
	playConnector := findPlay(artifacts.Play, "HoldConnector")
	watchConnector := findWatch(artifacts.Watch, "HoldConnector")
	previewConnector := findPreview(artifacts.Preview, "HoldConnector")
	playSim := findPlay(artifacts.Play, "SimLine")
	watchSim := findWatch(artifacts.Watch, "SimLine")
	previewSim := findPreview(artifacts.Preview, "SimLine")
	playTick := findPlay(artifacts.Play, "HoldTickNote")
	watchTick := findWatch(artifacts.Watch, "HoldTickNote")
	previewTick := findPreview(artifacts.Preview, "HoldTickNote")

	assertFunctions(t, "play Stage preprocess", artifacts.Play.Nodes, playStage.Preprocess.Index, resource.RuntimeFunctionSet)
	assertFunctionCount(t, "play Stage preprocess", artifacts.Play.Nodes, playStage.Preprocess.Index, resource.RuntimeFunctionSet, 7)
	assertSetBlock(t, "play Stage skin transform", artifacts.Play.Nodes, playStage.Preprocess.Index, 1003)
	assertSetBlock(t, "play Stage bucket windows", artifacts.Play.Nodes, playStage.Preprocess.Index, 2003)
	assertFunctions(t, "play Stage input reset", artifacts.Play.Nodes, playStage.UpdateSequential.Index, resource.RuntimeFunctionSet)
	assertFunctionCount(t, "play Stage input reset", artifacts.Play.Nodes, playStage.UpdateSequential.Index, resource.RuntimeFunctionSet, 2)
	assertSetBlock(t, "play Stage claimed touch reset", artifacts.Play.Nodes, playStage.UpdateSequential.Index, 2000)
	assertFunctions(t, "play TapNote overlap split", artifacts.Play.Nodes, playNote.Touch.Index,
		resource.RuntimeFunctionGetShifted, resource.RuntimeFunctionSetShifted)
	if optimization != optimize.LevelStandard {
		assertFunctions(t, "play TapNote timing tolerance", artifacts.Play.Nodes, playNote.Touch.Index,
			resource.RuntimeFunctionAbs)
	}
	assertFunctions(t, "play Stage empty lane touch", artifacts.Play.Nodes, playStage.Touch.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionSet, resource.RuntimeFunctionPlay,
		resource.RuntimeFunctionSpawnParticleEffect, resource.RuntimeFunctionStreamSet)
	assertGetBlock(t, "play Stage claimed touch read", artifacts.Play.Nodes, playStage.Touch.Index, 2000)
	assertNoSetBlock(t, "play Stage does not claim empty lane touch", artifacts.Play.Nodes, playStage.Touch.Index, 2000)
	assertGetBlock(t, "play Stage touch input", artifacts.Play.Nodes, playStage.Touch.Index, 1002)
	assertFunctions(t, "play Stage render", artifacts.Play.Nodes, playStage.UpdateParallel.Index, resource.RuntimeFunctionDraw)
	assertFunctionCount(t, "play Stage render", artifacts.Play.Nodes, playStage.UpdateParallel.Index, resource.RuntimeFunctionDraw, 4)
	assertFunctions(t, "play TapNote preprocess", artifacts.Play.Nodes, playNote.Preprocess.Index,
		resource.RuntimeFunctionNegate, resource.RuntimeFunctionPlayScheduled, resource.RuntimeFunctionSet)
	assertFunctionCount(t, "play TapNote preprocess", artifacts.Play.Nodes, playNote.Preprocess.Index, resource.RuntimeFunctionSet, 7)
	if optimization != optimize.LevelStandard {
		assertFunctions(t, "play TapNote preprocess", artifacts.Play.Nodes, playNote.Preprocess.Index,
			resource.RuntimeFunctionBeatToTime, resource.RuntimeFunctionTimeToScaledTime)
	}
	assertFunctions(t, "play TapNote touch", artifacts.Play.Nodes, playNote.Touch.Index,
		resource.RuntimeFunctionLessOr, resource.RuntimeFunctionJudge, resource.RuntimeFunctionNotEqual,
		resource.RuntimeFunctionPlay, resource.RuntimeFunctionSpawnParticleEffect, resource.RuntimeFunctionSet)
	assertFunctionCount(t, "play TapNote touch", artifacts.Play.Nodes, playNote.Touch.Index, resource.RuntimeFunctionSpawnParticleEffect, 3)
	if optimization == optimize.LevelStandard {
		assertRuntimeCallFirstValues(t, "play TapNote judgment SFX", artifacts.Play.Nodes, playNote.Touch.Index, resource.RuntimeFunctionPlay, []float64{1, 2, 3})
	}
	assertGetBlock(t, "play TapNote claimed touch read", artifacts.Play.Nodes, playNote.Touch.Index, 2000)
	assertSetBlock(t, "play TapNote claimed touch write", artifacts.Play.Nodes, playNote.Touch.Index, 2000)
	assertGetBlock(t, "play TapNote active note read", artifacts.Play.Nodes, playNote.UpdateSequential.Index, 2000)
	assertSetBlock(t, "play TapNote active note write", artifacts.Play.Nodes, playNote.UpdateSequential.Index, 2000)
	assertFunctions(t, "play TapNote spawn order", artifacts.Play.Nodes, playNote.SpawnOrder.Index, resource.RuntimeFunctionDivide, resource.RuntimeFunctionSubtract)
	assertFunctions(t, "play TapNote should spawn", artifacts.Play.Nodes, playNote.ShouldSpawn.Index, resource.RuntimeFunctionDivide, resource.RuntimeFunctionGreaterOr)
	assertFunctions(t, "play TapNote miss", artifacts.Play.Nodes, playNote.UpdateSequential.Index,
		resource.RuntimeFunctionAdd, resource.RuntimeFunctionGet, resource.RuntimeFunctionGreater, resource.RuntimeFunctionSet)
	assertNodeValues(t, "play TapNote miss", artifacts.Play.Nodes, playNote.UpdateSequential.Index, 1, 1000)
	assertFunctions(t, "play TapNote render", artifacts.Play.Nodes, playNote.UpdateParallel.Index,
		resource.RuntimeFunctionDivide, resource.RuntimeFunctionDraw)
	assertFunctions(t, "play TapNote replay end export", artifacts.Play.Nodes, playNote.Terminate.Index,
		resource.RuntimeFunctionExportValue)
	assertFunctions(t, "play FlickNote touch", artifacts.Play.Nodes, playFlick.Touch.Index,
		resource.RuntimeFunctionJudge, resource.RuntimeFunctionPlay,
		resource.RuntimeFunctionSpawnParticleEffect, resource.RuntimeFunctionSet)
	if optimization == optimize.LevelStandard {
		assertRuntimeCallFirstValues(t, "play FlickNote judgment SFX", artifacts.Play.Nodes, playFlick.Touch.Index, resource.RuntimeFunctionPlay, []float64{4, 5, 6})
	}
	assertFunctions(t, "play FlickNote preprocess", artifacts.Play.Nodes, playFlick.Preprocess.Index, resource.RuntimeFunctionSet)
	if optimization == optimize.LevelStandard {
		assertRuntimeCallFirstValues(t, "play FlickNote auto SFX", artifacts.Play.Nodes, playFlick.Preprocess.Index, resource.RuntimeFunctionPlayScheduled, []float64{4})
	}
	assertFunctions(t, "play FlickNote spawn order", artifacts.Play.Nodes, playFlick.SpawnOrder.Index, resource.RuntimeFunctionDivide, resource.RuntimeFunctionSubtract)
	assertFunctions(t, "play FlickNote should spawn", artifacts.Play.Nodes, playFlick.ShouldSpawn.Index, resource.RuntimeFunctionGreaterOr)
	assertFunctions(t, "play FlickNote deferred judgment", artifacts.Play.Nodes, playFlick.UpdateSequential.Index,
		resource.RuntimeFunctionGreater, resource.RuntimeFunctionJudge, resource.RuntimeFunctionSet)
	assertNodeValues(t, "play FlickNote miss", artifacts.Play.Nodes, playFlick.UpdateSequential.Index, 1, 1000)
	assertFunctions(t, "play FlickNote render", artifacts.Play.Nodes, playFlick.UpdateParallel.Index,
		resource.RuntimeFunctionRem, resource.RuntimeFunctionEaseOutQuad, resource.RuntimeFunctionDraw)
	assertFunctions(t, "play DirectionalFlickNote touch", artifacts.Play.Nodes, playDirectional.Touch.Index,
		resource.RuntimeFunctionMultiply, resource.RuntimeFunctionGreater, resource.RuntimeFunctionJudge,
		resource.RuntimeFunctionSpawnParticleEffect, resource.RuntimeFunctionSet)
	if optimization == optimize.LevelStandard {
		assertRuntimeCallFirstValues(t, "play DirectionalFlickNote judgment SFX", artifacts.Play.Nodes, playDirectional.Touch.Index, resource.RuntimeFunctionPlay, []float64{4, 5, 6})
	}
	assertFunctions(t, "play DirectionalFlickNote render", artifacts.Play.Nodes, playDirectional.UpdateParallel.Index,
		resource.RuntimeFunctionGreater, resource.RuntimeFunctionDraw)
	if optimization != optimize.LevelStandard {
		assertFunctions(t, "play DirectionalFlickNote magnitude", artifacts.Play.Nodes, playDirectional.Touch.Index,
			resource.RuntimeFunctionAbs)
		assertFunctions(t, "play DirectionalFlickNote arrow count", artifacts.Play.Nodes, playDirectional.UpdateParallel.Index,
			resource.RuntimeFunctionAbs)
	}
	assertFunctions(t, "play HoldNote touch", artifacts.Play.Nodes, playHold.Touch.Index,
		resource.RuntimeFunctionLessOr, resource.RuntimeFunctionPlay,
		resource.RuntimeFunctionSpawnParticleEffect, resource.RuntimeFunctionStreamSet, resource.RuntimeFunctionSet)
	assertFunctions(t, "play HoldNote initialize", artifacts.Play.Nodes, playHold.Initialize.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionSpawn)
	assertFunctions(t, "play HoldNote update", artifacts.Play.Nodes, playHold.UpdateSequential.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionGreater, resource.RuntimeFunctionSet)
	assertSetBlock(t, "play HoldNote active touch claim", artifacts.Play.Nodes, playHold.UpdateSequential.Index, 2000)
	assertFunctions(t, "play HoldManager update", artifacts.Play.Nodes, playManager.UpdateParallel.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionPlayLooped, resource.RuntimeFunctionSpawnParticleEffect,
		resource.RuntimeFunctionMoveParticleEffect, resource.RuntimeFunctionDestroyParticleEffect,
		resource.RuntimeFunctionStopLooped, resource.RuntimeFunctionDraw, resource.RuntimeFunctionSet)
	assertGetBlock(t, "play HoldManager referenced data", artifacts.Play.Nodes, playManager.UpdateParallel.Index, 4101)
	assertGetBlock(t, "play HoldManager referenced shared", artifacts.Play.Nodes, playManager.UpdateParallel.Index, 4102)
	assertFunctions(t, "play HoldManager terminate", artifacts.Play.Nodes, playManager.Terminate.Index,
		resource.RuntimeFunctionDestroyParticleEffect, resource.RuntimeFunctionStopLooped, resource.RuntimeFunctionSet)
	assertFunctions(t, "play HoldManager touch", artifacts.Play.Nodes, playManager.Touch.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionGetShifted, resource.RuntimeFunctionStreamSet, resource.RuntimeFunctionSet)
	assertGetBlock(t, "play HoldManager touch input", artifacts.Play.Nodes, playManager.Touch.Index, 1002)
	assertSetBlock(t, "play HoldManager referenced shared write", artifacts.Play.Nodes, playManager.Touch.Index, 4102)
	assertNoSetBlock(t, "play HoldManager does not recapture touch", artifacts.Play.Nodes, playManager.Touch.Index, 2000)
	assertFunctions(t, "play HoldNote render", artifacts.Play.Nodes, playHold.UpdateParallel.Index,
		resource.RuntimeFunctionDraw, resource.RuntimeFunctionIf)
	assertFunctions(t, "play HoldEndNote render", artifacts.Play.Nodes, playEnd.UpdateParallel.Index, resource.RuntimeFunctionDraw)
	assertFunctions(t, "play HoldEndNote preprocess", artifacts.Play.Nodes, playEnd.Preprocess.Index,
		resource.RuntimeFunctionPlayScheduled, resource.RuntimeFunctionSet)
	if optimization != optimize.LevelStandard {
		assertFunctions(t, "play HoldEndNote timing", artifacts.Play.Nodes, playEnd.Preprocess.Index,
			resource.RuntimeFunctionBeatToTime, resource.RuntimeFunctionTimeToScaledTime)
	}
	assertFunctions(t, "play HoldEndNote update", artifacts.Play.Nodes, playEnd.UpdateSequential.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionLessOr, resource.RuntimeFunctionStreamSet, resource.RuntimeFunctionSet)
	assertFunctions(t, "play HoldEndNote touch", artifacts.Play.Nodes, playEnd.Touch.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionGetShifted, resource.RuntimeFunctionLessOr,
		resource.RuntimeFunctionJudge, resource.RuntimeFunctionStreamSet, resource.RuntimeFunctionSpawnParticleEffect,
		resource.RuntimeFunctionSet)
	assertGetBlock(t, "play HoldEndNote claimed touch read", artifacts.Play.Nodes, playEnd.Touch.Index, 2000)
	assertSetBlock(t, "play HoldEndNote claimed touch write", artifacts.Play.Nodes, playEnd.Touch.Index, 2000)
	assertGetBlock(t, "play HoldEndNote referenced head", artifacts.Play.Nodes, playEnd.Touch.Index, 4102)
	assertFunctions(t, "play HoldFlickNote preprocess", artifacts.Play.Nodes, playHoldFlick.Preprocess.Index,
		resource.RuntimeFunctionPlayScheduled, resource.RuntimeFunctionSet)
	assertFunctions(t, "play HoldFlickNote update", artifacts.Play.Nodes, playHoldFlick.UpdateSequential.Index,
		resource.RuntimeFunctionJudge, resource.RuntimeFunctionPlay, resource.RuntimeFunctionStreamSet,
		resource.RuntimeFunctionSpawnParticleEffect, resource.RuntimeFunctionSet)
	assertFunctions(t, "play HoldFlickNote touch", artifacts.Play.Nodes, playHoldFlick.Touch.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionGetShifted,
		resource.RuntimeFunctionJudge, resource.RuntimeFunctionPlay, resource.RuntimeFunctionStreamSet,
		resource.RuntimeFunctionSpawnParticleEffect, resource.RuntimeFunctionSet)
	assertGetBlock(t, "play HoldFlickNote referenced head", artifacts.Play.Nodes, playHoldFlick.Touch.Index, 4102)
	assertGetBlock(t, "play HoldFlickNote touch input", artifacts.Play.Nodes, playHoldFlick.Touch.Index, 1002)
	assertSetBlock(t, "play HoldFlickNote claimed touch write", artifacts.Play.Nodes, playHoldFlick.Touch.Index, 2000)
	assertFunctions(t, "play HoldFlickNote render", artifacts.Play.Nodes, playHoldFlick.UpdateParallel.Index, resource.RuntimeFunctionDraw)
	assertFunctions(t, "play HoldConnector render", artifacts.Play.Nodes, playConnector.UpdateParallel.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionDraw, resource.RuntimeFunctionSet)
	assertGetBlock(t, "play HoldConnector referenced data", artifacts.Play.Nodes, playConnector.Preprocess.Index, 4101)
	assertFunctions(t, "play HoldConnector lane tracking", artifacts.Play.Nodes, playConnector.UpdateSequential.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionRemap, resource.RuntimeFunctionSet)
	assertGetBlock(t, "play HoldConnector referenced data", artifacts.Play.Nodes, playConnector.UpdateSequential.Index, 4101)
	assertSetBlock(t, "play HoldConnector shared lane write", artifacts.Play.Nodes, playConnector.UpdateSequential.Index, 4102)
	assertFunctions(t, "play SimLine render", artifacts.Play.Nodes, playSim.UpdateParallel.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionEqual, resource.RuntimeFunctionMultiply,
		resource.RuntimeFunctionEaseOutQuad, resource.RuntimeFunctionDraw, resource.RuntimeFunctionSet)
	assertGetBlock(t, "play SimLine referenced data", artifacts.Play.Nodes, playSim.UpdateParallel.Index, 4101)
	assertGetBlock(t, "play HoldTick referenced hold", artifacts.Play.Nodes, playTick.Preprocess.Index, 4101)
	assertFunctions(t, "play HoldTickNote update", artifacts.Play.Nodes, playTick.UpdateSequential.Index,
		resource.RuntimeFunctionGreater, resource.RuntimeFunctionJudge, resource.RuntimeFunctionPlay,
		resource.RuntimeFunctionSpawnParticleEffect, resource.RuntimeFunctionSet)
	assertSetBlock(t, "play HoldTick active touch claim", artifacts.Play.Nodes, playTick.UpdateSequential.Index, 2000)
	assertFunctions(t, "play HoldTickNote touch", artifacts.Play.Nodes, playTick.Touch.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionGetShifted,
		resource.RuntimeFunctionJudge, resource.RuntimeFunctionPlay, resource.RuntimeFunctionSpawnParticleEffect,
		resource.RuntimeFunctionSet)
	assertGetBlock(t, "play HoldTick active touch", artifacts.Play.Nodes, playTick.Touch.Index, 4102)
	assertGetBlock(t, "play HoldTick touch input", artifacts.Play.Nodes, playTick.Touch.Index, 1002)
	assertSetBlock(t, "play HoldTick claimed touch write", artifacts.Play.Nodes, playTick.Touch.Index, 2000)
	assertFunctions(t, "play HoldTickNote render", artifacts.Play.Nodes, playTick.UpdateParallel.Index, resource.RuntimeFunctionDraw)

	assertFunctions(t, "watch TapNote preprocess", artifacts.Watch.Nodes, watchNote.Preprocess.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionNegate, resource.RuntimeFunctionPlayScheduled, resource.RuntimeFunctionSet)
	assertGetBlock(t, "watch TapNote replay mode", artifacts.Watch.Nodes, watchNote.Preprocess.Index, 1000)
	assertFunctionCount(t, "watch TapNote preprocess", artifacts.Watch.Nodes, watchNote.Preprocess.Index, resource.RuntimeFunctionSet, 7)
	if optimization == optimize.LevelStandard {
		assertRuntimeCallFirstValues(t, "watch TapNote judgment SFX", artifacts.Watch.Nodes, watchNote.Preprocess.Index, resource.RuntimeFunctionPlayScheduled, []float64{1, 2, 3})
	}
	assertFunctions(t, "watch Stage preprocess", artifacts.Watch.Nodes, watchStage.Preprocess.Index, resource.RuntimeFunctionSet)
	assertFunctionCount(t, "watch Stage preprocess", artifacts.Watch.Nodes, watchStage.Preprocess.Index, resource.RuntimeFunctionSet, 8)
	assertSetBlock(t, "watch Stage skin transform", artifacts.Watch.Nodes, watchStage.Preprocess.Index, 1002)
	assertSetBlock(t, "watch Stage bucket windows", artifacts.Watch.Nodes, watchStage.Preprocess.Index, 2003)
	assertFunctions(t, "watch Stage empty lane replay", artifacts.Watch.Nodes, watchStage.Preprocess.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionPlayScheduled, resource.RuntimeFunctionSpawn,
		resource.RuntimeFunctionStreamHas, resource.RuntimeFunctionStreamGetNextKey, resource.RuntimeFunctionStreamGetValue)
	assertGetBlock(t, "watch Stage replay environment", artifacts.Watch.Nodes, watchStage.Preprocess.Index, 1000)
	if optimization != optimize.LevelStandard {
		assertFunctions(t, "watch ScheduledLaneEffect spawn time", artifacts.Watch.Nodes, watchLaneEffect.SpawnTime.Index,
			resource.RuntimeFunctionTimeToScaledTime)
		assertFunctions(t, "watch ScheduledLaneEffect despawn time", artifacts.Watch.Nodes, watchLaneEffect.DespawnTime.Index,
			resource.RuntimeFunctionTimeToScaledTime)
	}
	assertFunctions(t, "watch ScheduledLaneEffect despawn time", artifacts.Watch.Nodes, watchLaneEffect.DespawnTime.Index,
		resource.RuntimeFunctionAdd)
	assertFunctions(t, "watch ScheduledLaneEffect update", artifacts.Watch.Nodes, watchLaneEffect.UpdateParallel.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionGreater, resource.RuntimeFunctionGreaterOr,
		resource.RuntimeFunctionSpawnParticleEffect)
	assertNoFunctions(t, "watch ScheduledLaneEffect does not play unscheduled SFX", artifacts.Watch.Nodes, watchLaneEffect.UpdateParallel.Index,
		resource.RuntimeFunctionPlay)
	if optimization != optimize.LevelStandard {
		assertFunctions(t, "watch TapNote preprocess", artifacts.Watch.Nodes, watchNote.Preprocess.Index,
			resource.RuntimeFunctionBeatToTime, resource.RuntimeFunctionTimeToScaledTime)
	}
	assertFunctions(t, "watch TapNote replay", artifacts.Watch.Nodes, watchNote.Terminate.Index,
		resource.RuntimeFunctionEqual, resource.RuntimeFunctionGet, resource.RuntimeFunctionSpawnParticleEffect, resource.RuntimeFunctionSet)
	assertGetBlock(t, "watch TapNote skip state", artifacts.Watch.Nodes, watchNote.Terminate.Index, 1001)
	assertFunctionCount(t, "watch TapNote replay", artifacts.Watch.Nodes, watchNote.Terminate.Index, resource.RuntimeFunctionSpawnParticleEffect, 3)
	assertFunctions(t, "watch TapNote spawn time", artifacts.Watch.Nodes, watchNote.SpawnTime.Index, resource.RuntimeFunctionDivide, resource.RuntimeFunctionSubtract)
	assertFunctions(t, "watch TapNote despawn time", artifacts.Watch.Nodes, watchNote.DespawnTime.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionIf)
	assertFunctions(t, "watch TapNote render", artifacts.Watch.Nodes, watchNote.UpdateParallel.Index,
		resource.RuntimeFunctionDivide, resource.RuntimeFunctionDraw)
	assertFunctions(t, "watch update spawn", artifacts.Watch.Nodes, artifacts.Watch.UpdateSpawn, resource.RuntimeFunctionGet)
	assertFunctions(t, "watch FlickNote replay", artifacts.Watch.Nodes, watchFlick.Terminate.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionSpawnParticleEffect, resource.RuntimeFunctionSet)
	assertFunctions(t, "watch FlickNote preprocess", artifacts.Watch.Nodes, watchFlick.Preprocess.Index, resource.RuntimeFunctionSet)
	if optimization == optimize.LevelStandard {
		assertRuntimeCallFirstValues(t, "watch FlickNote judgment SFX", artifacts.Watch.Nodes, watchFlick.Preprocess.Index, resource.RuntimeFunctionPlayScheduled, []float64{4, 5, 6})
	}
	assertFunctions(t, "watch FlickNote spawn time", artifacts.Watch.Nodes, watchFlick.SpawnTime.Index, resource.RuntimeFunctionDivide, resource.RuntimeFunctionSubtract)
	assertFunctions(t, "watch FlickNote despawn time", artifacts.Watch.Nodes, watchFlick.DespawnTime.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionIf)
	assertFunctions(t, "watch FlickNote render", artifacts.Watch.Nodes, watchFlick.UpdateParallel.Index,
		resource.RuntimeFunctionRem, resource.RuntimeFunctionEaseOutQuad, resource.RuntimeFunctionDraw)
	assertFunctions(t, "watch DirectionalFlickNote replay", artifacts.Watch.Nodes, watchDirectional.Terminate.Index,
		resource.RuntimeFunctionGreater, resource.RuntimeFunctionGet, resource.RuntimeFunctionSpawnParticleEffect, resource.RuntimeFunctionSet)
	if optimization == optimize.LevelStandard {
		assertRuntimeCallFirstValues(t, "watch DirectionalFlickNote judgment SFX", artifacts.Watch.Nodes, watchDirectional.Preprocess.Index, resource.RuntimeFunctionPlayScheduled, []float64{4, 5, 6})
	}
	assertFunctions(t, "watch DirectionalFlickNote render", artifacts.Watch.Nodes, watchDirectional.UpdateParallel.Index,
		resource.RuntimeFunctionDraw)
	if optimization != optimize.LevelStandard {
		assertFunctions(t, "watch DirectionalFlickNote arrow count", artifacts.Watch.Nodes, watchDirectional.UpdateParallel.Index,
			resource.RuntimeFunctionAbs)
	}
	assertFunctions(t, "watch HoldNote replay", artifacts.Watch.Nodes, watchHold.Terminate.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionSpawnParticleEffect, resource.RuntimeFunctionSet)
	assertFunctions(t, "watch HoldNote preprocess", artifacts.Watch.Nodes, watchHold.Preprocess.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionSpawn, resource.RuntimeFunctionSet,
		resource.RuntimeFunctionStreamHas, resource.RuntimeFunctionStreamGetNextKey, resource.RuntimeFunctionStreamGetValue,
		resource.RuntimeFunctionPlayLoopedScheduled, resource.RuntimeFunctionStopLoopedScheduled)
	assertFunctionCount(t, "watch HoldNote scheduled loop starts", artifacts.Watch.Nodes, watchHold.Preprocess.Index, resource.RuntimeFunctionPlayLoopedScheduled, 2)
	assertFunctionCount(t, "watch HoldNote scheduled loop stops", artifacts.Watch.Nodes, watchHold.Preprocess.Index, resource.RuntimeFunctionStopLoopedScheduled, 2)
	assertFunctions(t, "watch HoldNote render", artifacts.Watch.Nodes, watchHold.UpdateParallel.Index,
		resource.RuntimeFunctionStreamHas, resource.RuntimeFunctionStreamGetPreviousKey, resource.RuntimeFunctionStreamGetValue,
		resource.RuntimeFunctionDraw)
	assertFunctions(t, "watch HoldManager update", artifacts.Watch.Nodes, watchManager.UpdateParallel.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionStreamHas, resource.RuntimeFunctionStreamGetPreviousKey,
		resource.RuntimeFunctionStreamGetValue, resource.RuntimeFunctionSpawnParticleEffect,
		resource.RuntimeFunctionMoveParticleEffect, resource.RuntimeFunctionDestroyParticleEffect,
		resource.RuntimeFunctionSet)
	assertNoFunctions(t, "watch HoldManager update", artifacts.Watch.Nodes, watchManager.UpdateParallel.Index,
		resource.RuntimeFunctionPlayLooped, resource.RuntimeFunctionStopLooped)
	assertGetBlock(t, "watch HoldManager referenced data", artifacts.Watch.Nodes, watchManager.UpdateParallel.Index, 4101)
	assertGetBlock(t, "watch HoldManager referenced shared", artifacts.Watch.Nodes, watchManager.UpdateParallel.Index, 4102)
	assertFunctions(t, "watch HoldManager terminate", artifacts.Watch.Nodes, watchManager.Terminate.Index,
		resource.RuntimeFunctionDestroyParticleEffect, resource.RuntimeFunctionSet)
	assertNoFunctions(t, "watch HoldManager terminate", artifacts.Watch.Nodes, watchManager.Terminate.Index,
		resource.RuntimeFunctionStopLooped)
	assertFunctions(t, "watch HoldEndNote render", artifacts.Watch.Nodes, watchEnd.UpdateParallel.Index, resource.RuntimeFunctionDraw)
	assertFunctions(t, "watch HoldEndNote preprocess", artifacts.Watch.Nodes, watchEnd.Preprocess.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionPlayScheduled, resource.RuntimeFunctionSet)
	assertFunctions(t, "watch HoldEndNote replay", artifacts.Watch.Nodes, watchEnd.Terminate.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionSpawnParticleEffect, resource.RuntimeFunctionSet)
	assertFunctions(t, "watch HoldFlickNote preprocess", artifacts.Watch.Nodes, watchHoldFlick.Preprocess.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionPlayScheduled, resource.RuntimeFunctionSet)
	assertFunctions(t, "watch HoldFlickNote replay", artifacts.Watch.Nodes, watchHoldFlick.Terminate.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionSpawnParticleEffect, resource.RuntimeFunctionSet)
	assertFunctions(t, "watch HoldFlickNote render", artifacts.Watch.Nodes, watchHoldFlick.UpdateParallel.Index, resource.RuntimeFunctionDraw)
	assertFunctions(t, "watch HoldConnector render", artifacts.Watch.Nodes, watchConnector.UpdateParallel.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionDraw)
	assertGetBlock(t, "watch HoldConnector referenced data", artifacts.Watch.Nodes, watchConnector.Preprocess.Index, 4101)
	assertFunctions(t, "watch HoldConnector lane tracking", artifacts.Watch.Nodes, watchConnector.UpdateSequential.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionStreamHas, resource.RuntimeFunctionStreamGetPreviousKey,
		resource.RuntimeFunctionStreamGetValue, resource.RuntimeFunctionRemap, resource.RuntimeFunctionSet)
	assertGetBlock(t, "watch HoldConnector referenced data", artifacts.Watch.Nodes, watchConnector.UpdateSequential.Index, 4101)
	assertSetBlock(t, "watch HoldConnector shared lane write", artifacts.Watch.Nodes, watchConnector.UpdateSequential.Index, 4102)
	assertFunctions(t, "watch SimLine despawn", artifacts.Watch.Nodes, watchSim.DespawnTime.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionIf, resource.RuntimeFunctionMin)
	assertFunctions(t, "watch SimLine render", artifacts.Watch.Nodes, watchSim.UpdateParallel.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionEqual, resource.RuntimeFunctionMultiply,
		resource.RuntimeFunctionEaseOutQuad, resource.RuntimeFunctionDraw)
	assertGetBlock(t, "watch SimLine referenced data", artifacts.Watch.Nodes, watchSim.UpdateParallel.Index, 4101)
	assertGetBlock(t, "watch HoldTick referenced hold", artifacts.Watch.Nodes, watchTick.Preprocess.Index, 4101)
	assertFunctions(t, "watch HoldTickNote replay", artifacts.Watch.Nodes, watchTick.Terminate.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionSpawnParticleEffect, resource.RuntimeFunctionSet)
	assertFunctions(t, "watch HoldTickNote render", artifacts.Watch.Nodes, watchTick.UpdateParallel.Index, resource.RuntimeFunctionDraw)

	assertFunctions(t, "preview BPM render", artifacts.Preview.Nodes, previewBPM.Render.Index, resource.RuntimeFunctionDraw, resource.RuntimeFunctionPrint)
	assertFunctions(t, "preview Timescale render", artifacts.Preview.Nodes, previewTimescale.Render.Index, resource.RuntimeFunctionDraw, resource.RuntimeFunctionPrint)
	assertAnyFunction(t, "preview BPM columns", artifacts.Preview.Nodes, previewBPM.Render.Index,
		resource.RuntimeFunctionRem, resource.RuntimeFunctionSetRem)
	assertFunctions(t, "preview TapNote render", artifacts.Preview.Nodes, previewNote.Render.Index,
		resource.RuntimeFunctionDraw)
	assertAnyFunction(t, "preview TapNote columns", artifacts.Preview.Nodes, previewNote.Render.Index,
		resource.RuntimeFunctionRem, resource.RuntimeFunctionSetRem)
	assertFunctions(t, "preview TapNote duration", artifacts.Preview.Nodes, previewNote.Preprocess.Index,
		resource.RuntimeFunctionGet, resource.RuntimeFunctionGreater, resource.RuntimeFunctionNegate, resource.RuntimeFunctionSet)
	assertGetBlock(t, "preview TapNote duration read", artifacts.Preview.Nodes, previewNote.Preprocess.Index, 2000)
	assertSetBlock(t, "preview TapNote duration write", artifacts.Preview.Nodes, previewNote.Preprocess.Index, 2000)
	assertFunctions(t, "preview Stage preprocess", artifacts.Preview.Nodes, previewStage.Preprocess.Index, resource.RuntimeFunctionSet)
	assertFunctionCount(t, "preview Stage preprocess", artifacts.Preview.Nodes, previewStage.Preprocess.Index, resource.RuntimeFunctionSet, 4)
	assertSetBlock(t, "preview Stage canvas", artifacts.Preview.Nodes, previewStage.Preprocess.Index, 1001)
	assertGetBlock(t, "preview Stage duration", artifacts.Preview.Nodes, previewStage.Preprocess.Index, 2000)
	assertSetBlock(t, "preview Stage columns", artifacts.Preview.Nodes, previewStage.Preprocess.Index, 2000)
	assertFunctions(t, "preview Stage render", artifacts.Preview.Nodes, previewStage.Render.Index, resource.RuntimeFunctionDraw)
	assertAnyFunction(t, "preview Stage columns", artifacts.Preview.Nodes, previewStage.Render.Index,
		resource.RuntimeFunctionRem, resource.RuntimeFunctionSetRem)
	assertGetBlock(t, "preview Stage screen", artifacts.Preview.Nodes, previewStage.Render.Index, 1000)
	if optimization != optimize.LevelStandard {
		assertFunctions(t, "preview BPM measure timing", artifacts.Preview.Nodes, previewBPM.Render.Index,
			resource.RuntimeFunctionBeatToStartingBeat, resource.RuntimeFunctionBeatToTime)
		assertFunctions(t, "preview BPM column index", artifacts.Preview.Nodes, previewBPM.Render.Index, resource.RuntimeFunctionTrunc)
		assertFunctions(t, "preview TapNote column index", artifacts.Preview.Nodes, previewNote.Render.Index, resource.RuntimeFunctionTrunc)
		assertFunctions(t, "preview Stage column index", artifacts.Preview.Nodes, previewStage.Render.Index, resource.RuntimeFunctionTrunc)
	}
	assertFunctionCount(t, "preview Stage render", artifacts.Preview.Nodes, previewStage.Render.Index, resource.RuntimeFunctionDraw, 5)
	assertFunctions(t, "preview FlickNote render", artifacts.Preview.Nodes, previewFlick.Render.Index, resource.RuntimeFunctionDraw)
	assertFunctions(t, "preview DirectionalFlickNote render", artifacts.Preview.Nodes, previewDirectional.Render.Index,
		resource.RuntimeFunctionGreater, resource.RuntimeFunctionDraw)
	if optimization != optimize.LevelStandard {
		assertFunctions(t, "preview DirectionalFlickNote arrow count", artifacts.Preview.Nodes, previewDirectional.Render.Index,
			resource.RuntimeFunctionAbs)
	}
	assertFunctions(t, "preview HoldNote render", artifacts.Preview.Nodes, previewHold.Render.Index, resource.RuntimeFunctionDraw)
	assertFunctions(t, "preview HoldEndNote render", artifacts.Preview.Nodes, previewEnd.Render.Index, resource.RuntimeFunctionDraw)
	assertFunctions(t, "preview HoldFlickNote render", artifacts.Preview.Nodes, previewHoldFlick.Render.Index, resource.RuntimeFunctionDraw)
	assertFunctions(t, "preview HoldConnector render", artifacts.Preview.Nodes, previewConnector.Render.Index, resource.RuntimeFunctionDraw)
	assertFunctions(t, "preview HoldConnector columns", artifacts.Preview.Nodes, previewConnector.Render.Index,
		resource.RuntimeFunctionMin)
	if optimization != optimize.LevelStandard {
		assertFunctions(t, "preview HoldConnector column index", artifacts.Preview.Nodes, previewConnector.Render.Index,
			resource.RuntimeFunctionTrunc)
	}
	assertGetBlock(t, "preview HoldConnector referenced data", artifacts.Preview.Nodes, previewConnector.Render.Index, 4100)
	assertFunctions(t, "preview SimLine render", artifacts.Preview.Nodes, previewSim.Render.Index, resource.RuntimeFunctionDraw)
	assertGetBlock(t, "preview SimLine referenced data", artifacts.Preview.Nodes, previewSim.Render.Index, 4100)
	assertGetBlock(t, "preview HoldTick referenced hold", artifacts.Preview.Nodes, previewTick.Preprocess.Index, 4100)
	assertFunctions(t, "preview HoldTickNote render", artifacts.Preview.Nodes, previewTick.Render.Index, resource.RuntimeFunctionDraw)

	assertFunctions(t, "tutorial preprocess", artifacts.Tutorial.Nodes, artifacts.Tutorial.Preprocess, resource.RuntimeFunctionSet)
	assertFunctionCount(t, "tutorial preprocess", artifacts.Tutorial.Nodes, artifacts.Tutorial.Preprocess, resource.RuntimeFunctionSet, 8)
	assertFunctions(t, "tutorial navigate", artifacts.Tutorial.Nodes, artifacts.Tutorial.Navigate, resource.RuntimeFunctionSet)
	assertFunctions(t, "tutorial update", artifacts.Tutorial.Nodes, artifacts.Tutorial.Update,
		resource.RuntimeFunctionEqual, resource.RuntimeFunctionDivide, resource.RuntimeFunctionDraw, resource.RuntimeFunctionPaint,
		resource.RuntimeFunctionPlay, resource.RuntimeFunctionPlayLooped, resource.RuntimeFunctionStopLooped,
		resource.RuntimeFunctionSpawnParticleEffect, resource.RuntimeFunctionMoveParticleEffect,
		resource.RuntimeFunctionDestroyParticleEffect, resource.RuntimeFunctionSet)
	assertFunctionCount(t, "tutorial dual particles", artifacts.Tutorial.Nodes, artifacts.Tutorial.Update, resource.RuntimeFunctionSpawnParticleEffect, 3)
	assertRuntimeCallFirstValues(t, "tutorial alternative judgment SFX", artifacts.Tutorial.Nodes, artifacts.Tutorial.Update, resource.RuntimeFunctionPlay, []float64{4})
}

func clipNames(clips []resource.EngineEffectDataClip) []string {
	names := make([]string, len(clips))
	for i, clip := range clips {
		names[i] = string(clip.Name)
	}
	return names
}

func particleNames(effects []resource.EngineParticleDataEffect) []string {
	names := make([]string, len(effects))
	for i, effect := range effects {
		names[i] = string(effect.Name)
	}
	return names
}

func importNames(imports []resource.EngineDataArchetypeImport) []resource.EngineArchetypeDataName {
	names := make([]resource.EngineArchetypeDataName, len(imports))
	for i, imported := range imports {
		names[i] = imported.Name
	}
	return names
}

func assertFunctions(t *testing.T, name string, nodes []resource.EngineDataNode, root int, required ...resource.RuntimeFunction) {
	t.Helper()
	functions := map[resource.RuntimeFunction]bool{}
	visited := map[int]bool{}
	var visit func(int)
	visit = func(index int) {
		if index < 0 || index >= len(nodes) || visited[index] {
			return
		}
		visited[index] = true
		function, ok := nodes[index].(resource.EngineDataFunctionNode)
		if !ok {
			return
		}
		functions[function.Func] = true
		for _, argument := range function.Args {
			visit(argument)
		}
	}
	visit(root)
	for _, function := range required {
		if !functions[function] {
			t.Fatalf("%s does not contain %s; functions=%v", name, function, functions)
		}
	}
}

func assertAnyFunction(t *testing.T, name string, nodes []resource.EngineDataNode, root int, alternatives ...resource.RuntimeFunction) {
	t.Helper()
	functions := map[resource.RuntimeFunction]bool{}
	visited := map[int]bool{}
	var visit func(int)
	visit = func(index int) {
		if index < 0 || index >= len(nodes) || visited[index] {
			return
		}
		visited[index] = true
		function, ok := nodes[index].(resource.EngineDataFunctionNode)
		if !ok {
			return
		}
		functions[function.Func] = true
		for _, argument := range function.Args {
			visit(argument)
		}
	}
	visit(root)
	for _, function := range alternatives {
		if functions[function] {
			return
		}
	}
	t.Fatalf("%s does not contain any of %v; functions=%v", name, alternatives, functions)
}

func assertNoFunctions(t *testing.T, name string, nodes []resource.EngineDataNode, root int, forbidden ...resource.RuntimeFunction) {
	t.Helper()
	functions := map[resource.RuntimeFunction]bool{}
	visited := map[int]bool{}
	var visit func(int)
	visit = func(index int) {
		if index < 0 || index >= len(nodes) || visited[index] {
			return
		}
		visited[index] = true
		function, ok := nodes[index].(resource.EngineDataFunctionNode)
		if !ok {
			return
		}
		functions[function.Func] = true
		for _, argument := range function.Args {
			visit(argument)
		}
	}
	visit(root)
	for _, function := range forbidden {
		if functions[function] {
			t.Fatalf("%s unexpectedly contains %s", name, function)
		}
	}
}

func assertFunctionCount(t *testing.T, name string, nodes []resource.EngineDataNode, root int, target resource.RuntimeFunction, minimum int) {
	t.Helper()
	count := 0
	visited := map[int]bool{}
	var visit func(int)
	visit = func(index int) {
		if index < 0 || index >= len(nodes) || visited[index] {
			return
		}
		visited[index] = true
		function, ok := nodes[index].(resource.EngineDataFunctionNode)
		if !ok {
			return
		}
		if function.Func == target {
			count++
		}
		for _, argument := range function.Args {
			visit(argument)
		}
	}
	visit(root)
	if count < minimum {
		t.Fatalf("%s contains %d %s nodes, want at least %d", name, count, target, minimum)
	}
}

func assertRuntimeCallFirstValues(t *testing.T, name string, nodes []resource.EngineDataNode, root int, target resource.RuntimeFunction, want []float64) {
	t.Helper()
	values := map[float64]bool{}
	visited := map[int]bool{}
	var collect func(int)
	collect = func(index int) {
		if index < 0 || index >= len(nodes) {
			return
		}
		switch node := nodes[index].(type) {
		case resource.EngineDataValueNode:
			values[node.Value] = true
		case resource.EngineDataFunctionNode:
			for _, argument := range node.Args {
				collect(argument)
			}
		}
	}
	var visit func(int)
	visit = func(index int) {
		if index < 0 || index >= len(nodes) || visited[index] {
			return
		}
		visited[index] = true
		function, ok := nodes[index].(resource.EngineDataFunctionNode)
		if !ok {
			return
		}
		if function.Func == target && len(function.Args) != 0 {
			collect(function.Args[0])
		}
		for _, argument := range function.Args {
			visit(argument)
		}
	}
	visit(root)
	got := make([]float64, 0, len(values))
	for value := range values {
		got = append(got, value)
	}
	slices.Sort(got)
	for _, value := range want {
		if !values[value] {
			t.Fatalf("%s first arguments contain %v, want at least %v", name, got, want)
		}
	}
}

func assertNodeValues(t *testing.T, name string, nodes []resource.EngineDataNode, root int, want ...float64) {
	t.Helper()
	values := map[float64]bool{}
	visited := map[int]bool{}
	var visit func(int)
	visit = func(index int) {
		if index < 0 || index >= len(nodes) || visited[index] {
			return
		}
		visited[index] = true
		switch node := nodes[index].(type) {
		case resource.EngineDataValueNode:
			values[node.Value] = true
		case resource.EngineDataFunctionNode:
			for _, argument := range node.Args {
				visit(argument)
			}
		}
	}
	visit(root)
	for _, value := range want {
		if !values[value] {
			t.Fatalf("%s does not contain value %v: %v", name, value, values)
		}
	}
}

func assertGetBlock(t *testing.T, name string, nodes []resource.EngineDataNode, root int, block float64) {
	t.Helper()
	visited := map[int]bool{}
	var visit func(int) bool
	visit = func(index int) bool {
		if index < 0 || index >= len(nodes) || visited[index] {
			return false
		}
		visited[index] = true
		function, ok := nodes[index].(resource.EngineDataFunctionNode)
		if !ok {
			return false
		}
		if (function.Func == resource.RuntimeFunctionGet || function.Func == resource.RuntimeFunctionGetPointed || function.Func == resource.RuntimeFunctionGetShifted) && len(function.Args) != 0 {
			value, ok := nodes[function.Args[0]].(resource.EngineDataValueNode)
			if ok && value.Value == block {
				return true
			}
		}
		for _, argument := range function.Args {
			if visit(argument) {
				return true
			}
		}
		return false
	}
	if !visit(root) {
		t.Fatalf("%s does not read memory block %v", name, block)
	}
}

func assertSetBlock(t *testing.T, name string, nodes []resource.EngineDataNode, root int, block float64) {
	t.Helper()
	visited := map[int]bool{}
	var visit func(int) bool
	visit = func(index int) bool {
		if index < 0 || index >= len(nodes) || visited[index] {
			return false
		}
		visited[index] = true
		function, ok := nodes[index].(resource.EngineDataFunctionNode)
		if !ok {
			return false
		}
		if (function.Func == resource.RuntimeFunctionSet || function.Func == resource.RuntimeFunctionSetPointed || function.Func == resource.RuntimeFunctionSetShifted) && len(function.Args) != 0 {
			value, ok := nodes[function.Args[0]].(resource.EngineDataValueNode)
			if ok && value.Value == block {
				return true
			}
		}
		for _, argument := range function.Args {
			if visit(argument) {
				return true
			}
		}
		return false
	}
	if !visit(root) {
		t.Fatalf("%s does not write memory block %v", name, block)
	}
}

func assertNoSetBlock(t *testing.T, name string, nodes []resource.EngineDataNode, root int, block float64) {
	t.Helper()
	visited := map[int]bool{}
	var visit func(int)
	visit = func(index int) {
		if index < 0 || index >= len(nodes) || visited[index] {
			return
		}
		visited[index] = true
		function, ok := nodes[index].(resource.EngineDataFunctionNode)
		if !ok {
			return
		}
		if function.Func == resource.RuntimeFunctionSet && len(function.Args) > 0 {
			if value, ok := nodes[function.Args[0]].(resource.EngineDataValueNode); ok && value.Value == block {
				t.Fatalf("%s contains Set for block %v", name, block)
			}
		}
		for _, argument := range function.Args {
			visit(argument)
		}
	}
	visit(root)
}

func assertDevelopmentChart(t *testing.T, data *resource.LevelData) {
	t.Helper()
	var beats []float64
	lanes := map[float64]bool{}
	lanesAtBeat := map[float64]map[float64]bool{}
	for _, entity := range data.Entities {
		if entity.Archetype != "TapNote" {
			continue
		}
		beat, beatOK := entityValue(entity, "#BEAT")
		lane, laneOK := entityValue(entity, "lane")
		if !beatOK || !laneOK {
			t.Fatalf("TapNote %q is missing beat or lane: %#v", entity.Name, entity.Data)
		}
		if lane < -3 || lane > 3 || lane != float64(int(lane)) {
			t.Fatalf("TapNote %q has invalid lane %v", entity.Name, lane)
		}
		if beat < 0 || len(beats) != 0 && beat < beats[len(beats)-1] {
			t.Fatalf("TapNote beats must be non-negative and ordered: %v then %v", beats[len(beats)-1], beat)
		}
		if lanesAtBeat[beat] == nil {
			lanesAtBeat[beat] = map[float64]bool{}
		}
		if lanesAtBeat[beat][lane] {
			t.Fatalf("TapNote beat %v repeats lane %v", beat, lane)
		}
		beats = append(beats, beat)
		lanes[lane] = true
		lanesAtBeat[beat][lane] = true
	}
	if !reflect.DeepEqual(beats, []float64{2, 4, 6, 6, 6, 8, 10}) || len(lanes) < 3 || len(lanesAtBeat[6]) != 3 {
		t.Fatalf("unexpected development chart: beats=%v lanes=%v", beats, lanes)
	}
}

func entityValue(entity resource.LevelDataEntity, name resource.EngineArchetypeDataName) (float64, bool) {
	for _, item := range entity.Data {
		value, ok := item.(resource.LevelDataEntityValueData)
		if ok && value.Name == name {
			return value.Value, true
		}
	}
	return 0, false
}

func assertDevelopmentFlick(t *testing.T, data *resource.LevelData) {
	t.Helper()
	count := 0
	for _, entity := range data.Entities {
		if entity.Archetype != "FlickNote" {
			continue
		}
		count++
		beat, beatOK := entityValue(entity, "#BEAT")
		lane, laneOK := entityValue(entity, "lane")
		if !beatOK || !laneOK || beat != 12 || lane != 0 {
			t.Fatalf("unexpected FlickNote fixture: %#v", entity)
		}
	}
	if count != 1 {
		t.Fatalf("development chart contains %d FlickNote entities, want 1", count)
	}
}

func assertDevelopmentDirectionalFlick(t *testing.T, data *resource.LevelData) {
	t.Helper()
	wantLanes := map[float64]float64{-2: -1, 2: 1}
	directions := map[float64]bool{}
	for _, entity := range data.Entities {
		if entity.Archetype != "DirectionalFlickNote" {
			continue
		}
		beat, beatOK := entityValue(entity, "#BEAT")
		lane, laneOK := entityValue(entity, "lane")
		direction, directionOK := entityValue(entity, "direction")
		expectedLane, expectedDirection := wantLanes[direction]
		if !beatOK || !laneOK || !directionOK || beat != 14 || !expectedDirection || lane != expectedLane {
			t.Fatalf("unexpected DirectionalFlickNote fixture: %#v", entity)
		}
		directions[direction] = true
	}
	if len(directions) != 2 {
		t.Fatalf("development chart directions = %v, want left and right", directions)
	}
}

func assertDevelopmentHold(t *testing.T, data *resource.LevelData) {
	t.Helper()
	type expectedEntity struct {
		archetype  resource.EngineArchetypeName
		beat, lane float64
		head, prev string
		next       string
	}
	want := map[string]expectedEntity{
		"hold-head-1":    {"HoldHeadNote", 16, -2, "", "", "hold-tick-early-1"},
		"hold-anchor-1":  {"HoldAnchorNote", 18, 1, "", "hold-tick-middle-1", "hold-anchor-1b"},
		"hold-anchor-1b": {"HoldAnchorNote", 19, -1, "", "hold-anchor-1", "hold-tick-2"},
		"hold-end-1":     {"HoldEndNote", 20, 2, "", "hold-tick-late-1", ""},
		"hold-head-2":    {"HoldHeadNote", 22, 3, "", "", "hold-anchor-2"},
		"hold-anchor-2":  {"HoldAnchorNote", 23, -1, "", "hold-head-2", "hold-tick-3"},
		"hold-flick-2":   {"HoldFlickNote", 24, -3, "", "hold-tick-3", ""},
	}
	found := map[string]bool{}
	for _, entity := range data.Entities {
		expected, ok := want[entity.Name]
		if !ok {
			continue
		}
		beat, beatOK := entityValue(entity, "#BEAT")
		lane, laneOK := entityValue(entity, "lane")
		refs := entityRefs(entity)
		if entity.Archetype != expected.archetype || !beatOK || !laneOK || beat != expected.beat || lane != expected.lane ||
			refs["head"] != expected.head || refs["prev"] != expected.prev || refs["next"] != expected.next {
			t.Fatalf("unexpected Hold chain entity: %#v", entity)
		}
		found[entity.Name] = true
	}
	if len(found) != len(want) {
		t.Fatalf("development chart Hold chain = %v, want %v", found, want)
	}
}

func assertDevelopmentHoldConnector(t *testing.T, data *resource.LevelData) {
	t.Helper()
	want := map[string]map[resource.EngineArchetypeDataName]string{
		"hold-1-connector-a": {"first": "hold-head-1", "second": "hold-tick-early-1"},
		"hold-1-connector-b": {"first": "hold-tick-early-1", "second": "hold-tick-1"},
		"hold-1-connector-c": {"first": "hold-tick-1", "second": "hold-tick-middle-1"},
		"hold-1-connector-d": {"first": "hold-tick-middle-1", "second": "hold-anchor-1"},
		"hold-1-connector-e": {"first": "hold-anchor-1", "second": "hold-anchor-1b"},
		"hold-1-connector-f": {"first": "hold-anchor-1b", "second": "hold-tick-2"},
		"hold-1-connector-g": {"first": "hold-tick-2", "second": "hold-tick-late-1"},
		"hold-1-connector-h": {"first": "hold-tick-late-1", "second": "hold-end-1"},
		"hold-2-connector-a": {"first": "hold-head-2", "second": "hold-anchor-2"},
		"hold-2-connector-b": {"first": "hold-anchor-2", "second": "hold-tick-3"},
		"hold-2-connector-c": {"first": "hold-tick-3", "second": "hold-flick-2"},
	}
	found := map[string]bool{}
	for _, entity := range data.Entities {
		if entity.Archetype != "HoldConnector" {
			continue
		}
		refs := entityRefs(entity)
		expected, ok := want[entity.Name]
		if !ok || !reflect.DeepEqual(refs, expected) {
			t.Fatalf("unexpected HoldConnector fixture: %#v", entity)
		}
		found[entity.Name] = true
	}
	if len(found) != len(want) {
		t.Fatalf("development chart HoldConnector segments = %v, want %v", found, want)
	}
}

func entityRefs(entity resource.LevelDataEntity) map[resource.EngineArchetypeDataName]string {
	refs := map[resource.EngineArchetypeDataName]string{}
	for _, item := range entity.Data {
		if ref, ok := item.(resource.LevelDataEntityRefData); ok {
			refs[ref.Name] = ref.Ref
		}
	}
	return refs
}

func assertDevelopmentSimLine(t *testing.T, data *resource.LevelData) {
	t.Helper()
	want := map[string]map[resource.EngineArchetypeDataName]string{
		"sim-1": {"first": "tap-3-chord", "second": "tap-3-center"},
		"sim-2": {"first": "tap-3-center", "second": "tap-3"},
		"sim-3": {"first": "directional-flick-left", "second": "directional-flick-right"},
	}
	found := map[string]bool{}
	for _, entity := range data.Entities {
		if entity.Archetype != "SimLine" {
			continue
		}
		refs := map[resource.EngineArchetypeDataName]string{}
		for _, item := range entity.Data {
			if ref, ok := item.(resource.LevelDataEntityRefData); ok {
				refs[ref.Name] = ref.Ref
			}
		}
		if expected, ok := want[entity.Name]; !ok || !reflect.DeepEqual(refs, expected) {
			t.Fatalf("unexpected SimLine refs: %v", refs)
		}
		found[entity.Name] = true
	}
	if len(found) != len(want) {
		t.Fatalf("development chart SimLines = %v, want %v", found, want)
	}
}

func assertDevelopmentHoldTick(t *testing.T, data *resource.LevelData) {
	t.Helper()
	want := map[string]struct {
		beat       float64
		prev, next string
	}{
		"hold-tick-early-1":  {16.5, "hold-head-1", "hold-tick-1"},
		"hold-tick-1":        {17, "hold-tick-early-1", "hold-tick-middle-1"},
		"hold-tick-middle-1": {17.5, "hold-tick-1", "hold-anchor-1"},
		"hold-tick-2":        {19, "hold-anchor-1b", "hold-tick-late-1"},
		"hold-tick-late-1":   {19.5, "hold-tick-2", "hold-end-1"},
		"hold-tick-3":        {23.5, "hold-anchor-2", "hold-flick-2"},
	}
	found := map[string]bool{}
	for _, entity := range data.Entities {
		if entity.Archetype != "HoldTickNote" {
			continue
		}
		beat, beatOK := entityValue(entity, "#BEAT")
		expected, ok := want[entity.Name]
		refs := entityRefs(entity)
		if !ok || !beatOK || beat != expected.beat || refs["prev"] != expected.prev || refs["next"] != expected.next {
			t.Fatalf("unexpected HoldTickNote fixture: %#v", entity)
		}
		found[entity.Name] = true
	}
	if len(found) != len(want) {
		t.Fatalf("development chart HoldTick chain = %v, want %v", found, want)
	}
}

func assertDevelopmentTimescale(t *testing.T, data *resource.LevelData) {
	t.Helper()
	want := map[float64]float64{8: 0.75, 21: 1.25}
	found := map[float64]float64{}
	for _, entity := range data.Entities {
		if entity.Archetype != "#TIMESCALE_CHANGE" {
			continue
		}
		beat, beatOK := entityValue(entity, "#BEAT")
		timescale, timescaleOK := entityValue(entity, "#TIMESCALE")
		if !beatOK || !timescaleOK || want[beat] != timescale {
			t.Fatalf("unexpected timescale fixture: %#v", entity)
		}
		found[beat] = timescale
	}
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("development timescales = %v, want %v", found, want)
	}
}

func assertDevelopmentBPM(t *testing.T, data *resource.LevelData) {
	t.Helper()
	want := map[float64]float64{0: 120, 12: 180}
	found := map[float64]float64{}
	for _, entity := range data.Entities {
		if entity.Archetype != "#BPM_CHANGE" {
			continue
		}
		beat, beatOK := entityValue(entity, "#BEAT")
		bpm, bpmOK := entityValue(entity, "#BPM")
		if !beatOK || !bpmOK || want[beat] != bpm {
			t.Fatalf("unexpected BPM fixture: %#v", entity)
		}
		found[beat] = bpm
	}
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("development BPM changes = %v, want %v", found, want)
	}
	chartDuration := 12.0/120*60 + (24.0-12)/180*60
	columnCount := previewColumnsForDuration(chartDuration)
	canvasDuration := previewColumnSeconds * float64(columnCount)
	if columnCount != 6 {
		t.Fatalf("development chart column count = %d, want 6", columnCount)
	}
	if chartDuration > canvasDuration {
		t.Fatalf("development chart duration %v exceeds preview canvas duration %v", chartDuration, canvasDuration)
	}
}

func assertNodes(t *testing.T, mode string, nodes []resource.EngineDataNode) {
	t.Helper()
	if len(nodes) == 0 {
		t.Fatalf("%s node pool is empty", mode)
	}
	for i, node := range nodes {
		function, ok := node.(resource.EngineDataFunctionNode)
		if !ok {
			continue
		}
		for _, argument := range function.Args {
			if argument < 0 || argument >= len(nodes) {
				t.Fatalf("%s node %d has invalid argument %d", mode, i, argument)
			}
		}
	}
}

func assertIndex(t *testing.T, name string, index, length int) {
	t.Helper()
	if index < 0 || index >= length {
		t.Fatalf("%s index %d is outside node pool of length %d", name, index, length)
	}
}

func findPlay(data *resource.EnginePlayData, name resource.EngineArchetypeName) *resource.EnginePlayDataArchetype {
	for i := range data.Archetypes {
		if data.Archetypes[i].Name == name {
			return &data.Archetypes[i]
		}
	}
	return nil
}

func findWatch(data *resource.EngineWatchData, name resource.EngineArchetypeName) *resource.EngineWatchDataArchetype {
	for i := range data.Archetypes {
		if data.Archetypes[i].Name == name {
			return &data.Archetypes[i]
		}
	}
	return nil
}

func findPreview(data *resource.EnginePreviewData, name resource.EngineArchetypeName) *resource.EnginePreviewDataArchetype {
	for i := range data.Archetypes {
		if data.Archetypes[i].Name == name {
			return &data.Archetypes[i]
		}
	}
	return nil
}
