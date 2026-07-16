package catalog

import "github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

type RecipeKind string

const (
	RecipeRuntime     RecipeKind = "runtime"
	RecipeAggregate   RecipeKind = "aggregate"
	RecipeMemory      RecipeKind = "memory"
	RecipeResource    RecipeKind = "resource"
	RecipeContainer   RecipeKind = "container"
	RecipeCompileTime RecipeKind = "compile-time"
	RecipeForbidden   RecipeKind = "forbidden"
)

type Recipe struct {
	Kind      RecipeKind
	Runtime   resource.RuntimeFunction
	Prefix    []float64
	Operation string
	Reason    string
	Storage   string
	Offset    int
	Stride    int
	IndexArg  int
	Write     bool
}

type memoryRecipe struct {
	storage  string
	offset   int
	stride   int
	indexArg int
	write    bool
}

var memoryRecipes = map[string]memoryRecipe{
	"sonolus.Bucket.Window":    {"LevelBucket", 0, 6, 0, false},
	"sonolus.Bucket.SetWindow": {"LevelBucket", 0, 6, 0, true},

	"sonolus/play.timeAPI.Now":                  {"RuntimeUpdate", 0, 0, -1, false},
	"sonolus/play.timeAPI.Delta":                {"RuntimeUpdate", 1, 0, -1, false},
	"sonolus/play.timeAPI.Scaled":               {"RuntimeUpdate", 2, 0, -1, false},
	"sonolus/play.audioAPI.Offset":              {"RuntimeEnvironment", 2, 0, -1, false},
	"sonolus/play.inputAPI.Offset":              {"RuntimeEnvironment", 3, 0, -1, false},
	"sonolus/play.multiplayerAPI.IsMultiplayer": {"RuntimeEnvironment", 4, 0, -1, false},
	"sonolus/play.touchesAPI.Count":             {"RuntimeUpdate", 3, 0, -1, false},
	"sonolus/play.backgroundAPI.Get":            {"RuntimeBackground", 0, 0, -1, false},
	"sonolus/play.backgroundAPI.Set":            {"RuntimeBackground", 0, 0, -1, true},
	"sonolus/play.entityAPI.Info":               {"CurrentEntityInfo", 0, 0, -1, false},
	"sonolus/play.entityAPI.InfoAt":             {"EntityInfo", 0, 3, 0, false},
	"sonolus/play.entityAPI.Result":             {"CurrentInputResult", 0, 0, -1, false},
	"sonolus/play.entityAPI.SetResult":          {"CurrentInputResult", 0, 0, -1, true},
	"sonolus/play.entityAPI.Despawn":            {"CurrentEntityDespawn", 0, 0, -1, false},
	"sonolus/play.entityAPI.SetDespawn":         {"CurrentEntityDespawn", 0, 0, -1, true},
	"sonolus/play.lifeAPI.Initial":              {"LevelLife", 6, 0, -1, false},
	"sonolus/play.lifeAPI.SetInitial":           {"LevelLife", 6, 0, -1, true},
	"sonolus/play.lifeAPI.Max":                  {"LevelLife", 7, 0, -1, false},
	"sonolus/play.lifeAPI.SetMax":               {"LevelLife", 7, 0, -1, true},
	"sonolus/play.lifeAPI.Archetype":            {"ArchetypeLife", 0, 4, 0, false},
	"sonolus/play.lifeAPI.SetArchetype":         {"ArchetypeLife", 0, 4, 0, true},
	"sonolus/play.lifeAPI.Consecutive":          {"LevelLife", 0, 2, 0, false},
	"sonolus/play.lifeAPI.SetConsecutive":       {"LevelLife", 0, 2, 0, true},
	"sonolus/play.scoreAPI.Archetype":           {"ArchetypeScore", 0, 1, 0, false},
	"sonolus/play.scoreAPI.SetArchetype":        {"ArchetypeScore", 0, 1, 0, true},
	"sonolus/play.scoreAPI.Base":                {"LevelScore", 0, 0, -1, false},
	"sonolus/play.scoreAPI.SetBase":             {"LevelScore", 0, 0, -1, true},
	"sonolus/play.scoreAPI.Consecutive":         {"LevelScore", 3, 3, 0, false},
	"sonolus/play.scoreAPI.SetConsecutive":      {"LevelScore", 3, 3, 0, true},
	"sonolus/play.transformAPI.Get":             {"RuntimeTransform", 0, 0, -1, false},
	"sonolus/play.transformAPI.Set":             {"RuntimeTransform", 0, 0, -1, true},
	"sonolus/play.environmentAPI.Debug":         {"RuntimeEnvironment", 0, 0, -1, false},
	"sonolus/play.environmentAPI.AspectRatio":   {"RuntimeEnvironment", 1, 0, -1, false},
	"sonolus/play.environmentAPI.Multiplayer":   {"RuntimeEnvironment", 4, 0, -1, false},
	"sonolus/play.levelMemoryAPI.Get":           {"LevelMemory", 0, 1, 0, false},
	"sonolus/play.levelMemoryAPI.Set":           {"LevelMemory", 0, 1, 0, true},
	"sonolus/play.levelDataAPI.Get":             {"LevelData", 0, 1, 0, false},
	"sonolus/play.levelDataAPI.Set":             {"LevelData", 0, 1, 0, true},

	"sonolus/watch.timeAPI.Now":                {"RuntimeUpdate", 0, 0, -1, false},
	"sonolus/watch.timeAPI.Delta":              {"RuntimeUpdate", 1, 0, -1, false},
	"sonolus/watch.timeAPI.Scaled":             {"RuntimeUpdate", 2, 0, -1, false},
	"sonolus/watch.timeAPI.Skip":               {"RuntimeUpdate", 3, 0, -1, false},
	"sonolus/watch.audioAPI.Offset":            {"RuntimeEnvironment", 2, 0, -1, false},
	"sonolus/watch.inputAPI.Offset":            {"RuntimeEnvironment", 3, 0, -1, false},
	"sonolus/watch.replayAPI.IsReplay":         {"RuntimeEnvironment", 4, 0, -1, false},
	"sonolus/watch.backgroundAPI.Get":          {"RuntimeBackground", 0, 0, -1, false},
	"sonolus/watch.backgroundAPI.Set":          {"RuntimeBackground", 0, 0, -1, true},
	"sonolus/watch.transformAPI.Get":           {"RuntimeTransform", 0, 0, -1, false},
	"sonolus/watch.transformAPI.Set":           {"RuntimeTransform", 0, 0, -1, true},
	"sonolus/watch.entityAPI.Info":             {"CurrentEntityInfo", 0, 0, -1, false},
	"sonolus/watch.entityAPI.InfoAt":           {"EntityInfo", 0, 3, 0, false},
	"sonolus/watch.entityAPI.Result":           {"CurrentInputResult", 0, 0, -1, false},
	"sonolus/watch.entityAPI.SetResult":        {"CurrentInputResult", 0, 0, -1, true},
	"sonolus/watch.lifeAPI.Initial":            {"LevelLife", 6, 0, -1, false},
	"sonolus/watch.lifeAPI.SetInitial":         {"LevelLife", 6, 0, -1, true},
	"sonolus/watch.lifeAPI.Max":                {"LevelLife", 7, 0, -1, false},
	"sonolus/watch.lifeAPI.SetMax":             {"LevelLife", 7, 0, -1, true},
	"sonolus/watch.lifeAPI.Archetype":          {"ArchetypeLife", 0, 4, 0, false},
	"sonolus/watch.lifeAPI.SetArchetype":       {"ArchetypeLife", 0, 4, 0, true},
	"sonolus/watch.lifeAPI.Consecutive":        {"LevelLife", 0, 2, 0, false},
	"sonolus/watch.lifeAPI.SetConsecutive":     {"LevelLife", 0, 2, 0, true},
	"sonolus/watch.scoreAPI.Archetype":         {"ArchetypeScore", 0, 1, 0, false},
	"sonolus/watch.scoreAPI.SetArchetype":      {"ArchetypeScore", 0, 1, 0, true},
	"sonolus/watch.scoreAPI.Base":              {"LevelScore", 0, 0, -1, false},
	"sonolus/watch.scoreAPI.SetBase":           {"LevelScore", 0, 0, -1, true},
	"sonolus/watch.scoreAPI.Consecutive":       {"LevelScore", 3, 3, 0, false},
	"sonolus/watch.scoreAPI.SetConsecutive":    {"LevelScore", 3, 3, 0, true},
	"sonolus/watch.environmentAPI.Debug":       {"RuntimeEnvironment", 0, 0, -1, false},
	"sonolus/watch.environmentAPI.AspectRatio": {"RuntimeEnvironment", 1, 0, -1, false},
	"sonolus/watch.environmentAPI.Replay":      {"RuntimeEnvironment", 4, 0, -1, false},
	"sonolus/watch.levelMemoryAPI.Get":         {"LevelMemory", 0, 1, 0, false},
	"sonolus/watch.levelMemoryAPI.Set":         {"LevelMemory", 0, 1, 0, true},
	"sonolus/watch.levelDataAPI.Get":           {"LevelData", 0, 1, 0, false},
	"sonolus/watch.levelDataAPI.Set":           {"LevelData", 0, 1, 0, true},

	"sonolus/preview.canvasAPI.Scroll":           {"RuntimeCanvas", 0, 0, -1, false},
	"sonolus/preview.canvasAPI.Size":             {"RuntimeCanvas", 1, 0, -1, false},
	"sonolus/preview.canvasAPI.Set":              {"RuntimeCanvas", 0, 0, -1, true},
	"sonolus/preview.levelDataAPI.Get":           {"PreviewData", 0, 1, 0, false},
	"sonolus/preview.levelDataAPI.Set":           {"PreviewData", 0, 1, 0, true},
	"sonolus/preview.transformAPI.Get":           {"RuntimeTransform", 0, 0, -1, false},
	"sonolus/preview.transformAPI.Set":           {"RuntimeTransform", 0, 0, -1, true},
	"sonolus/preview.entityAPI.Info":             {"CurrentEntityInfo", 0, 0, -1, false},
	"sonolus/preview.entityAPI.InfoAt":           {"EntityInfo", 0, 2, 0, false},
	"sonolus/preview.environmentAPI.Debug":       {"RuntimeEnvironment", 0, 0, -1, false},
	"sonolus/preview.environmentAPI.AspectRatio": {"RuntimeEnvironment", 1, 0, -1, false},

	"sonolus/tutorial.timeAPI.Now":                {"RuntimeUpdate", 0, 0, -1, false},
	"sonolus/tutorial.timeAPI.Delta":              {"RuntimeUpdate", 1, 0, -1, false},
	"sonolus/tutorial.timeAPI.Scaled":             {"RuntimeUpdate", 0, 0, -1, false},
	"sonolus/tutorial.timeAPI.OffsetAdjusted":     {"RuntimeUpdate", 0, 0, -1, false},
	"sonolus/tutorial.audioAPI.Offset":            {"RuntimeEnvironment", 2, 0, -1, false},
	"sonolus/tutorial.backgroundAPI.Get":          {"RuntimeBackground", 0, 0, -1, false},
	"sonolus/tutorial.backgroundAPI.Set":          {"RuntimeBackground", 0, 0, -1, true},
	"sonolus/tutorial.navigationAPI.Direction":    {"RuntimeUpdate", 2, 0, -1, false},
	"sonolus/tutorial.memoryAPI.Get":              {"TutorialMemory", 0, 1, 0, false},
	"sonolus/tutorial.memoryAPI.Set":              {"TutorialMemory", 0, 1, 0, true},
	"sonolus/tutorial.dataAPI.Get":                {"TutorialData", 0, 1, 0, false},
	"sonolus/tutorial.dataAPI.Set":                {"TutorialData", 0, 1, 0, true},
	"sonolus/tutorial.transformAPI.Get":           {"RuntimeTransform", 0, 0, -1, false},
	"sonolus/tutorial.transformAPI.Set":           {"RuntimeTransform", 0, 0, -1, true},
	"sonolus/tutorial.environmentAPI.Debug":       {"RuntimeEnvironment", 0, 0, -1, false},
	"sonolus/tutorial.environmentAPI.AspectRatio": {"RuntimeEnvironment", 1, 0, -1, false},
}

var facadeRuntimeRecipes = map[string]resource.RuntimeFunction{
	"sonolus.Sign":                                      resource.RuntimeFunctionSign,
	"sonolus.Frac":                                      resource.RuntimeFunctionFrac,
	"sonolus.Clamp":                                     resource.RuntimeFunctionClamp,
	"sonolus.Lerp":                                      resource.RuntimeFunctionLerp,
	"sonolus.LerpClamped":                               resource.RuntimeFunctionLerpClamped,
	"sonolus.Unlerp":                                    resource.RuntimeFunctionUnlerp,
	"sonolus.UnlerpClamped":                             resource.RuntimeFunctionUnlerpClamped,
	"sonolus.Remap":                                     resource.RuntimeFunctionRemap,
	"sonolus.RemapClamped":                              resource.RuntimeFunctionRemapClamped,
	"sonolus/play.audioAPI.Play":                        resource.RuntimeFunctionPlay,
	"sonolus/play.audioAPI.PlayScheduled":               resource.RuntimeFunctionPlayScheduled,
	"sonolus/play.audioAPI.PlayLooped":                  resource.RuntimeFunctionPlayLooped,
	"sonolus/play.debugAPI.Log":                         resource.RuntimeFunctionDebugLog,
	"sonolus/play.debugAPI.Pause":                       resource.RuntimeFunctionDebugPause,
	"sonolus/play.inputAPI.Judge":                       resource.RuntimeFunctionJudge,
	"sonolus/play.lifeAPI.AddScheduled":                 resource.RuntimeFunctionAddLifeScheduled,
	"sonolus/play.streamsAPI.Set":                       resource.RuntimeFunctionStreamSet,
	"sonolus/play.timeAPI.BeatToBPM":                    resource.RuntimeFunctionBeatToBPM,
	"sonolus/play.timeAPI.BeatToTime":                   resource.RuntimeFunctionBeatToTime,
	"sonolus/play.timeAPI.BeatToStartingBeat":           resource.RuntimeFunctionBeatToStartingBeat,
	"sonolus/play.timeAPI.BeatToStartingTime":           resource.RuntimeFunctionBeatToStartingTime,
	"sonolus/play.timeAPI.TimeToScaledTime":             resource.RuntimeFunctionTimeToScaledTime,
	"sonolus/play.timeAPI.TimeToStartingScaledTime":     resource.RuntimeFunctionTimeToStartingScaledTime,
	"sonolus/play.timeAPI.TimeToStartingTime":           resource.RuntimeFunctionTimeToStartingTime,
	"sonolus/play.timeAPI.TimeToTimeScale":              resource.RuntimeFunctionTimeToTimeScale,
	"sonolus/watch.audioAPI.Play":                       resource.RuntimeFunctionPlay,
	"sonolus/watch.audioAPI.PlayScheduled":              resource.RuntimeFunctionPlayScheduled,
	"sonolus/watch.debugAPI.Log":                        resource.RuntimeFunctionDebugLog,
	"sonolus/watch.debugAPI.Pause":                      resource.RuntimeFunctionDebugPause,
	"sonolus/watch.inputAPI.Judge":                      resource.RuntimeFunctionJudge,
	"sonolus/watch.lifeAPI.AddScheduled":                resource.RuntimeFunctionAddLifeScheduled,
	"sonolus/watch.streamsAPI.Has":                      resource.RuntimeFunctionStreamHas,
	"sonolus/watch.streamsAPI.PreviousKey":              resource.RuntimeFunctionStreamGetPreviousKey,
	"sonolus/watch.streamsAPI.NextKey":                  resource.RuntimeFunctionStreamGetNextKey,
	"sonolus/watch.streamsAPI.Value":                    resource.RuntimeFunctionStreamGetValue,
	"sonolus/watch.timeAPI.BeatToBPM":                   resource.RuntimeFunctionBeatToBPM,
	"sonolus/watch.timeAPI.BeatToTime":                  resource.RuntimeFunctionBeatToTime,
	"sonolus/watch.timeAPI.BeatToStartingBeat":          resource.RuntimeFunctionBeatToStartingBeat,
	"sonolus/watch.timeAPI.BeatToStartingTime":          resource.RuntimeFunctionBeatToStartingTime,
	"sonolus/watch.timeAPI.TimeToScaledTime":            resource.RuntimeFunctionTimeToScaledTime,
	"sonolus/watch.timeAPI.TimeToStartingScaledTime":    resource.RuntimeFunctionTimeToStartingScaledTime,
	"sonolus/watch.timeAPI.TimeToStartingTime":          resource.RuntimeFunctionTimeToStartingTime,
	"sonolus/watch.timeAPI.TimeToTimeScale":             resource.RuntimeFunctionTimeToTimeScale,
	"sonolus/preview.timeAPI.BeatToBPM":                 resource.RuntimeFunctionBeatToBPM,
	"sonolus/preview.timeAPI.BeatToTime":                resource.RuntimeFunctionBeatToTime,
	"sonolus/preview.timeAPI.BeatToStartingBeat":        resource.RuntimeFunctionBeatToStartingBeat,
	"sonolus/preview.timeAPI.BeatToStartingTime":        resource.RuntimeFunctionBeatToStartingTime,
	"sonolus/preview.timeAPI.TimeToScaledTime":          resource.RuntimeFunctionTimeToScaledTime,
	"sonolus/preview.timeAPI.TimeToStartingScaledTime":  resource.RuntimeFunctionTimeToStartingScaledTime,
	"sonolus/preview.timeAPI.TimeToStartingTime":        resource.RuntimeFunctionTimeToStartingTime,
	"sonolus/preview.timeAPI.TimeToTimeScale":           resource.RuntimeFunctionTimeToTimeScale,
	"sonolus/preview.canvasAPI.Print":                   resource.RuntimeFunctionPrint,
	"sonolus/preview.debugAPI.Log":                      resource.RuntimeFunctionDebugLog,
	"sonolus/preview.debugAPI.Pause":                    resource.RuntimeFunctionDebugPause,
	"sonolus/tutorial.audioAPI.Play":                    resource.RuntimeFunctionPlay,
	"sonolus/tutorial.audioAPI.PlayScheduled":           resource.RuntimeFunctionPlayScheduled,
	"sonolus/tutorial.debugAPI.Log":                     resource.RuntimeFunctionDebugLog,
	"sonolus/tutorial.debugAPI.Pause":                   resource.RuntimeFunctionDebugPause,
	"sonolus/tutorial.instructionAPI.Paint":             resource.RuntimeFunctionPaint,
	"sonolus/tutorial.timeAPI.BeatToBPM":                resource.RuntimeFunctionBeatToBPM,
	"sonolus/tutorial.timeAPI.BeatToTime":               resource.RuntimeFunctionBeatToTime,
	"sonolus/tutorial.timeAPI.BeatToStartingBeat":       resource.RuntimeFunctionBeatToStartingBeat,
	"sonolus/tutorial.timeAPI.BeatToStartingTime":       resource.RuntimeFunctionBeatToStartingTime,
	"sonolus/tutorial.timeAPI.TimeToScaledTime":         resource.RuntimeFunctionTimeToScaledTime,
	"sonolus/tutorial.timeAPI.TimeToStartingScaledTime": resource.RuntimeFunctionTimeToStartingScaledTime,
	"sonolus/tutorial.timeAPI.TimeToStartingTime":       resource.RuntimeFunctionTimeToStartingTime,
	"sonolus/tutorial.timeAPI.TimeToTimeScale":          resource.RuntimeFunctionTimeToTimeScale,
}

var uiMemoryRecipes = map[string]memoryRecipe{
	"sonolus/play.uiAPI.Menu":                         {"RuntimeUI", 0, 0, -1, false},
	"sonolus/play.uiAPI.SetMenu":                      {"RuntimeUI", 0, 0, -1, true},
	"sonolus/play.uiAPI.Judgment":                     {"RuntimeUI", 10, 0, -1, false},
	"sonolus/play.uiAPI.SetJudgment":                  {"RuntimeUI", 10, 0, -1, true},
	"sonolus/play.uiAPI.ComboValue":                   {"RuntimeUI", 20, 0, -1, false},
	"sonolus/play.uiAPI.SetComboValue":                {"RuntimeUI", 20, 0, -1, true},
	"sonolus/play.uiAPI.ComboText":                    {"RuntimeUI", 30, 0, -1, false},
	"sonolus/play.uiAPI.SetComboText":                 {"RuntimeUI", 30, 0, -1, true},
	"sonolus/play.uiAPI.PrimaryMetricBar":             {"RuntimeUI", 40, 0, -1, false},
	"sonolus/play.uiAPI.SetPrimaryMetricBar":          {"RuntimeUI", 40, 0, -1, true},
	"sonolus/play.uiAPI.PrimaryMetricValue":           {"RuntimeUI", 50, 0, -1, false},
	"sonolus/play.uiAPI.SetPrimaryMetricValue":        {"RuntimeUI", 50, 0, -1, true},
	"sonolus/play.uiAPI.SecondaryMetricBar":           {"RuntimeUI", 60, 0, -1, false},
	"sonolus/play.uiAPI.SetSecondaryMetricBar":        {"RuntimeUI", 60, 0, -1, true},
	"sonolus/play.uiAPI.SecondaryMetricValue":         {"RuntimeUI", 70, 0, -1, false},
	"sonolus/play.uiAPI.SetSecondaryMetricValue":      {"RuntimeUI", 70, 0, -1, true},
	"sonolus/play.uiAPI.MenuConfiguration":            {"RuntimeUIConfiguration", 0, 0, -1, false},
	"sonolus/play.uiAPI.JudgmentConfiguration":        {"RuntimeUIConfiguration", 2, 0, -1, false},
	"sonolus/play.uiAPI.ComboConfiguration":           {"RuntimeUIConfiguration", 4, 0, -1, false},
	"sonolus/play.uiAPI.PrimaryMetricConfiguration":   {"RuntimeUIConfiguration", 6, 0, -1, false},
	"sonolus/play.uiAPI.SecondaryMetricConfiguration": {"RuntimeUIConfiguration", 8, 0, -1, false},

	"sonolus/watch.uiAPI.Menu":                         {"RuntimeUI", 0, 0, -1, false},
	"sonolus/watch.uiAPI.SetMenu":                      {"RuntimeUI", 0, 0, -1, true},
	"sonolus/watch.uiAPI.Judgment":                     {"RuntimeUI", 10, 0, -1, false},
	"sonolus/watch.uiAPI.SetJudgment":                  {"RuntimeUI", 10, 0, -1, true},
	"sonolus/watch.uiAPI.ComboValue":                   {"RuntimeUI", 20, 0, -1, false},
	"sonolus/watch.uiAPI.SetComboValue":                {"RuntimeUI", 20, 0, -1, true},
	"sonolus/watch.uiAPI.ComboText":                    {"RuntimeUI", 30, 0, -1, false},
	"sonolus/watch.uiAPI.SetComboText":                 {"RuntimeUI", 30, 0, -1, true},
	"sonolus/watch.uiAPI.PrimaryMetricBar":             {"RuntimeUI", 40, 0, -1, false},
	"sonolus/watch.uiAPI.SetPrimaryMetricBar":          {"RuntimeUI", 40, 0, -1, true},
	"sonolus/watch.uiAPI.PrimaryMetricValue":           {"RuntimeUI", 50, 0, -1, false},
	"sonolus/watch.uiAPI.SetPrimaryMetricValue":        {"RuntimeUI", 50, 0, -1, true},
	"sonolus/watch.uiAPI.SecondaryMetricBar":           {"RuntimeUI", 60, 0, -1, false},
	"sonolus/watch.uiAPI.SetSecondaryMetricBar":        {"RuntimeUI", 60, 0, -1, true},
	"sonolus/watch.uiAPI.SecondaryMetricValue":         {"RuntimeUI", 70, 0, -1, false},
	"sonolus/watch.uiAPI.SetSecondaryMetricValue":      {"RuntimeUI", 70, 0, -1, true},
	"sonolus/watch.uiAPI.Progress":                     {"RuntimeUI", 80, 0, -1, false},
	"sonolus/watch.uiAPI.SetProgress":                  {"RuntimeUI", 80, 0, -1, true},
	"sonolus/watch.uiAPI.ProgressGraph":                {"RuntimeUI", 90, 0, -1, false},
	"sonolus/watch.uiAPI.SetProgressGraph":             {"RuntimeUI", 90, 0, -1, true},
	"sonolus/watch.uiAPI.MenuConfiguration":            {"RuntimeUIConfiguration", 0, 0, -1, false},
	"sonolus/watch.uiAPI.JudgmentConfiguration":        {"RuntimeUIConfiguration", 2, 0, -1, false},
	"sonolus/watch.uiAPI.ComboConfiguration":           {"RuntimeUIConfiguration", 4, 0, -1, false},
	"sonolus/watch.uiAPI.PrimaryMetricConfiguration":   {"RuntimeUIConfiguration", 6, 0, -1, false},
	"sonolus/watch.uiAPI.SecondaryMetricConfiguration": {"RuntimeUIConfiguration", 8, 0, -1, false},
	"sonolus/watch.uiAPI.ProgressConfiguration":        {"RuntimeUIConfiguration", 10, 0, -1, false},

	"sonolus/preview.uiAPI.Menu":                  {"RuntimeUI", 0, 0, -1, false},
	"sonolus/preview.uiAPI.SetMenu":               {"RuntimeUI", 0, 0, -1, true},
	"sonolus/preview.uiAPI.Progress":              {"RuntimeUI", 9, 0, -1, false},
	"sonolus/preview.uiAPI.SetProgress":           {"RuntimeUI", 9, 0, -1, true},
	"sonolus/preview.uiAPI.MenuConfiguration":     {"RuntimeUIConfiguration", 0, 0, -1, false},
	"sonolus/preview.uiAPI.ProgressConfiguration": {"RuntimeUIConfiguration", 2, 0, -1, false},

	"sonolus/tutorial.uiAPI.Menu":                     {"RuntimeUI", 0, 0, -1, false},
	"sonolus/tutorial.uiAPI.SetMenu":                  {"RuntimeUI", 0, 0, -1, true},
	"sonolus/tutorial.uiAPI.Previous":                 {"RuntimeUI", 9, 0, -1, false},
	"sonolus/tutorial.uiAPI.SetPrevious":              {"RuntimeUI", 9, 0, -1, true},
	"sonolus/tutorial.uiAPI.Next":                     {"RuntimeUI", 18, 0, -1, false},
	"sonolus/tutorial.uiAPI.SetNext":                  {"RuntimeUI", 18, 0, -1, true},
	"sonolus/tutorial.uiAPI.Instruction":              {"RuntimeUI", 27, 0, -1, false},
	"sonolus/tutorial.uiAPI.SetInstruction":           {"RuntimeUI", 27, 0, -1, true},
	"sonolus/tutorial.uiAPI.MenuConfiguration":        {"RuntimeUIConfiguration", 0, 0, -1, false},
	"sonolus/tutorial.uiAPI.NavigationConfiguration":  {"RuntimeUIConfiguration", 2, 0, -1, false},
	"sonolus/tutorial.uiAPI.InstructionConfiguration": {"RuntimeUIConfiguration", 4, 0, -1, false},
}

var aggregateRecipes = map[string]string{
	"sonolus.Ease":                    "ease",
	"sonolus.Linstep":                 "easing.linstep",
	"sonolus.Smoothstep":              "easing.smoothstep",
	"sonolus.Smootherstep":            "easing.smootherstep",
	"sonolus.StepStart":               "easing.stepStart",
	"sonolus.StepEnd":                 "easing.stepEnd",
	"sonolus.NewRange":                "range.new",
	"sonolus.UnitVec2":                "vec2.unit",
	"sonolus.IsPlay":                  "mode.isPlay",
	"sonolus.IsWatch":                 "mode.isWatch",
	"sonolus.IsPreview":               "mode.isPreview",
	"sonolus.IsTutorial":              "mode.isTutorial",
	"sonolus.IsPreprocessing":         "mode.isPreprocessing",
	"sonolus.Vec2.Lerp":               "vec2.lerp",
	"sonolus.Vec2.LerpClamped":        "vec2.lerpClamped",
	"sonolus.Range.Length":            "range.length",
	"sonolus.Range.IsEmpty":           "range.isEmpty",
	"sonolus.Range.Mid":               "range.mid",
	"sonolus.Range.Contains":          "range.contains",
	"sonolus.Range.ContainsRange":     "range.containsRange",
	"sonolus.Range.Add":               "range.add",
	"sonolus.Range.Sub":               "range.sub",
	"sonolus.Range.Mul":               "range.mul",
	"sonolus.Range.Div":               "range.div",
	"sonolus.Range.Intersect":         "range.intersect",
	"sonolus.Range.Shrink":            "range.shrink",
	"sonolus.Range.Expand":            "range.expand",
	"sonolus.Range.Lerp":              "range.lerp",
	"sonolus.Range.LerpClamped":       "range.lerpClamped",
	"sonolus.Range.Unlerp":            "range.unlerp",
	"sonolus.Range.UnlerpClamped":     "range.unlerpClamped",
	"sonolus.Range.Clamp":             "range.clamp",
	"sonolus/play.debugAPI.Terminate": "control.terminate",
	"sonolus/play.touchesAPI.Get":     "touch.get",
	"sonolus/play.entityAPI.Key":      "entity.key",
	"sonolus/watch.entityAPI.Key":     "entity.key",
	"sonolus/preview.entityAPI.Key":   "entity.key",
	"sonolus/play.timeAPI.Previous":   "time.previous", "sonolus/watch.timeAPI.Previous": "time.previous", "sonolus/tutorial.timeAPI.Previous": "time.previous",
	"sonolus/play.timeAPI.OffsetAdjusted": "time.offsetAdjusted", "sonolus/watch.timeAPI.OffsetAdjusted": "time.offsetAdjusted",
	"sonolus/play.screenAPI.Rect":     "screen.rect",
	"sonolus/watch.screenAPI.Rect":    "screen.rect",
	"sonolus/preview.screenAPI.Rect":  "screen.rect",
	"sonolus/tutorial.screenAPI.Rect": "screen.rect",
	"sonolus.NewVec2":                 "vec2.new", "sonolus.Vec2.Add": "vec2.add", "sonolus.Vec2.Sub": "vec2.sub",
	"sonolus.Vec2.Mul": "vec2.mul", "sonolus.Vec2.Div": "vec2.div", "sonolus.Vec2.Dot": "vec2.dot",
	"sonolus.Vec2.MulVec": "vec2.mulVec", "sonolus.Vec2.DivVec": "vec2.divVec", "sonolus.Vec2.Negate": "vec2.negate",
	"sonolus.Vec2.MagnitudeSquared": "vec2.magnitudeSquared", "sonolus.Vec2.Magnitude": "vec2.magnitude",
	"sonolus.Vec2.Angle": "vec2.angle", "sonolus.Vec2.Orthogonal": "vec2.orthogonal",
	"sonolus.Vec2.Normalize": "vec2.normalize", "sonolus.Vec2.NormalizeOrZero": "vec2.normalizeOrZero",
	"sonolus.Vec2.Rotate": "vec2.rotate", "sonolus.Vec2.RotateAbout": "vec2.rotateAbout",
	"sonolus.Vec2.AngleDiff": "vec2.angleDiff", "sonolus.Vec2.SignedAngleDiff": "vec2.signedAngleDiff",
	"sonolus.RectFromCenter": "rect.fromCenter", "sonolus.RectFromMargin": "rect.fromMargin",
	"sonolus.Rect.Width": "rect.width", "sonolus.Rect.Height": "rect.height", "sonolus.Rect.Center": "rect.center",
	"sonolus.Rect.BL": "rect.bl", "sonolus.Rect.TL": "rect.tl", "sonolus.Rect.TR": "rect.tr", "sonolus.Rect.BR": "rect.br",
	"sonolus.Rect.Top": "rect.top", "sonolus.Rect.Right": "rect.right", "sonolus.Rect.Bottom": "rect.bottom", "sonolus.Rect.Left": "rect.left",
	"sonolus.Rect.Translate": "rect.translate", "sonolus.Rect.Scale": "rect.scale",
	"sonolus.Rect.ScaleVec": "rect.scaleVec", "sonolus.Rect.ScaleAbout": "rect.scaleAbout",
	"sonolus.Rect.ScaleCentered": "rect.scaleCentered", "sonolus.Rect.Expand": "rect.expand", "sonolus.Rect.Shrink": "rect.shrink",
	"sonolus.Rect.ToQuad": "rect.toQuad", "sonolus.Rect.Contains": "rect.contains",
	"sonolus.Quad.Center": "quad.center", "sonolus.Quad.Translate": "quad.translate", "sonolus.Quad.Scale": "quad.scale",
	"sonolus.Quad.ScaleVec": "quad.scaleVec", "sonolus.Quad.ScaleAbout": "quad.scaleAbout", "sonolus.Quad.ScaleCentered": "quad.scaleCentered",
	"sonolus.Quad.Rotate": "quad.rotate", "sonolus.Quad.Top": "quad.top", "sonolus.Quad.Right": "quad.right",
	"sonolus.Quad.RotateAbout": "quad.rotateAbout", "sonolus.Quad.RotateCentered": "quad.rotateCentered",
	"sonolus.Quad.Bottom": "quad.bottom", "sonolus.Quad.Left": "quad.left", "sonolus.Quad.Permute": "quad.permute",
	"sonolus.Quad.Contains":         "quad.contains",
	"sonolus.IdentityTransform2D":   "transform.identity",
	"sonolus.Transform2D.Translate": "transform.translate", "sonolus.Transform2D.Scale": "transform.scale",
	"sonolus.Transform2D.Rotate": "transform.rotate", "sonolus.Transform2D.Compose": "transform.compose",
	"sonolus.Transform2D.ComposeBefore": "transform.composeBefore", "sonolus.Transform2D.ScaleAbout": "transform.scaleAbout",
	"sonolus.Transform2D.RotateAbout": "transform.rotateAbout", "sonolus.Transform2D.TransformVec": "transform.vec",
	"sonolus.Transform2D.TransformQuad": "transform.quad", "sonolus.Transform2D.TransformRect": "transform.rect", "sonolus.Transform2D.ShearX": "transform.shearX",
	"sonolus.Transform2D.ShearY": "transform.shearY", "sonolus.Transform2D.SimplePerspectiveX": "transform.simplePerspectiveX",
	"sonolus.Transform2D.SimplePerspectiveY": "transform.simplePerspectiveY", "sonolus.Transform2D.PerspectiveX": "transform.perspectiveX",
	"sonolus.Transform2D.PerspectiveY": "transform.perspectiveY", "sonolus.Transform2D.InversePerspectiveX": "transform.inversePerspectiveX",
	"sonolus.Transform2D.InversePerspectiveY": "transform.inversePerspectiveY", "sonolus.Transform2D.Normalize": "transform.normalize",
	"sonolus.IdentityInvertibleTransform2D":   "invertibleTransform.identity",
	"sonolus.InvertibleTransform2D.Translate": "invertibleTransform.translate", "sonolus.InvertibleTransform2D.Scale": "invertibleTransform.scale",
	"sonolus.InvertibleTransform2D.ScaleAbout": "invertibleTransform.scaleAbout", "sonolus.InvertibleTransform2D.Rotate": "invertibleTransform.rotate",
	"sonolus.InvertibleTransform2D.RotateAbout": "invertibleTransform.rotateAbout", "sonolus.InvertibleTransform2D.ShearX": "invertibleTransform.shearX",
	"sonolus.InvertibleTransform2D.ShearY": "invertibleTransform.shearY", "sonolus.InvertibleTransform2D.SimplePerspectiveX": "invertibleTransform.simplePerspectiveX",
	"sonolus.InvertibleTransform2D.SimplePerspectiveY": "invertibleTransform.simplePerspectiveY", "sonolus.InvertibleTransform2D.PerspectiveX": "invertibleTransform.perspectiveX",
	"sonolus.InvertibleTransform2D.PerspectiveY": "invertibleTransform.perspectiveY", "sonolus.InvertibleTransform2D.Normalize": "invertibleTransform.normalize",
	"sonolus.InvertibleTransform2D.Compose": "invertibleTransform.compose", "sonolus.InvertibleTransform2D.ComposeBefore": "invertibleTransform.composeBefore",
	"sonolus.InvertibleTransform2D.TransformVec": "invertibleTransform.vec", "sonolus.InvertibleTransform2D.InverseTransformVec": "invertibleTransform.inverseVec",
	"sonolus.InvertibleTransform2D.TransformQuad": "invertibleTransform.quad", "sonolus.InvertibleTransform2D.InverseTransformQuad": "invertibleTransform.inverseQuad",
	"sonolus.InvertibleTransform2D.TransformRect": "invertibleTransform.rect", "sonolus.InvertibleTransform2D.InverseTransformRect": "invertibleTransform.inverseRect",
	"sonolus.PerspectiveApproach": "transform.perspectiveApproach",
}

var forbiddenRecipes = map[string]string{}

var resourceRecipes = map[string]string{
	"sonolus/play.Spawn":                      "archetype.spawn",
	"sonolus/watch.Spawn":                     "archetype.spawn",
	"sonolus/play.ArchetypeID":                "archetype.id",
	"sonolus/watch.ArchetypeID":               "archetype.id",
	"sonolus/preview.ArchetypeID":             "archetype.id",
	"sonolus/play.ArchetypeKey":               "archetype.key",
	"sonolus/watch.ArchetypeKey":              "archetype.key",
	"sonolus/preview.ArchetypeKey":            "archetype.key",
	"sonolus/play.CurrentEntityRef":           "entity.currentRef",
	"sonolus/watch.CurrentEntityRef":          "entity.currentRef",
	"sonolus/preview.CurrentEntityRef":        "entity.currentRef",
	"sonolus.EntityRef.Get":                   "entityRef.get",
	"sonolus.EntityRef.GetUnchecked":          "entityRef.getUnchecked",
	"sonolus.EntityRef.Key":                   "entityRef.key",
	"sonolus.EntityRefAs":                     "entityRef.as",
	"sonolus.EntityRefMatches":                "entityRef.matches",
	"sonolus.EntityRefGetAs":                  "entityRef.getAs",
	"sonolus/play.touchesAPI.Values":          "touch.values",
	"sonolus/play.touchesAPI.Items":           "touch.items",
	"sonolus.Stream.Set":                      "stream.set",
	"sonolus.Stream.Has":                      "stream.has",
	"sonolus.Stream.PreviousKey":              "stream.previousKey",
	"sonolus.Stream.NextKey":                  "stream.nextKey",
	"sonolus.Stream.Get":                      "stream.get",
	"sonolus.Stream.PreviousKeyOrDefault":     "stream.previousKeyOrDefault",
	"sonolus.Stream.NextKeyOrDefault":         "stream.nextKeyOrDefault",
	"sonolus.Stream.HasPreviousKey":           "stream.hasPreviousKey",
	"sonolus.Stream.HasNextKey":               "stream.hasNextKey",
	"sonolus.Stream.PreviousKeyInclusive":     "stream.previousKeyInclusive",
	"sonolus.Stream.NextKeyInclusive":         "stream.nextKeyInclusive",
	"sonolus.Stream.GetPrevious":              "stream.getPrevious",
	"sonolus.Stream.GetNext":                  "stream.getNext",
	"sonolus.Stream.GetPreviousInclusive":     "stream.getPreviousInclusive",
	"sonolus.Stream.GetNextInclusive":         "stream.getNextInclusive",
	"sonolus.Stream.ItemsFrom":                "stream.itemsFrom",
	"sonolus.Stream.ItemsFromDescending":      "stream.itemsFromDescending",
	"sonolus.Stream.ItemsSincePreviousFrame":  "stream.itemsSincePreviousFrame",
	"sonolus.Stream.KeysFrom":                 "stream.keysFrom",
	"sonolus.Stream.KeysFromDescending":       "stream.keysFromDescending",
	"sonolus.Stream.KeysSincePreviousFrame":   "stream.keysSincePreviousFrame",
	"sonolus.Stream.ValuesFrom":               "stream.valuesFrom",
	"sonolus.Stream.ValuesFromDescending":     "stream.valuesFromDescending",
	"sonolus.Stream.ValuesSincePreviousFrame": "stream.valuesSincePreviousFrame",
	"sonolus.StreamData.Set":                  "streamData.set",
	"sonolus.StreamData.Get":                  "streamData.get",
	"sonolus.Sprite.Draw":                     "sprite.draw", "sonolus.Sprite.Exists": "sprite.exists",
	"sonolus.Sprite.DrawCurvedB": "sprite.drawCurvedB", "sonolus.Sprite.DrawCurvedT": "sprite.drawCurvedT",
	"sonolus.Sprite.DrawCurvedL": "sprite.drawCurvedL", "sonolus.Sprite.DrawCurvedR": "sprite.drawCurvedR",
	"sonolus.Sprite.DrawCurvedBT": "sprite.drawCurvedBT", "sonolus.Sprite.DrawCurvedLR": "sprite.drawCurvedLR",
	"sonolus.JudgmentWindow.Judge": "judgment.judge", "sonolus.JudgmentWindows.Judge": "judgment.judge",
	"sonolus.Clip.Play": "clip.play", "sonolus.Clip.PlayScheduled": "clip.playScheduled",
	"sonolus.Clip.PlayLooped": "clip.playLooped", "sonolus.Clip.PlayLoopedScheduled": "clip.playLoopedScheduled",
	"sonolus.LoopedEffectHandle.Stop": "loop.stop", "sonolus.ScheduledLoopedEffectHandle.Stop": "loop.stopScheduled",
	"sonolus.Effect.Spawn": "particle.spawn", "sonolus.ParticleHandle.Move": "particle.move", "sonolus.ParticleHandle.Destroy": "particle.destroy",
	"sonolus/tutorial.instructionAPI.Show": "instruction.show", "sonolus/tutorial.instructionAPI.Clear": "instruction.clear",
}

var containerRecipes = map[string]string{
	"sonolus.NewVarArray": "varArray.new", "sonolus.VarArray.Len": "varArray.len", "sonolus.VarArray.Capacity": "varArray.capacity",
	"sonolus.VarArray.IsFull": "varArray.isFull", "sonolus.VarArray.Get": "varArray.get", "sonolus.VarArray.GetUnchecked": "varArray.getUnchecked", "sonolus.VarArray.Set": "varArray.set", "sonolus.VarArray.SetUnchecked": "varArray.setUnchecked",
	"sonolus.VarArray.Append": "varArray.append", "sonolus.VarArray.AppendUnchecked": "varArray.appendUnchecked", "sonolus.VarArray.Pop": "varArray.pop", "sonolus.VarArray.Clear": "varArray.clear",
	"sonolus.VarArray.Contains": "varArray.contains", "sonolus.VarArray.Insert": "varArray.insert",
	"sonolus.VarArray.RemoveAt": "varArray.removeAt", "sonolus.VarArray.Remove": "varArray.remove",
	"sonolus.VarArray.Index": "varArray.index", "sonolus.VarArray.LastIndex": "varArray.lastIndex", "sonolus.VarArray.Count": "varArray.count",
	"sonolus.VarArray.Swap": "varArray.swap", "sonolus.VarArray.SwapUnchecked": "varArray.swapUnchecked", "sonolus.VarArray.Reverse": "varArray.reverse",
	"sonolus.VarArray.Shuffle": "varArray.shuffle", "sonolus.VarArray.SortFunc": "varArray.sortFunc", "sonolus.VarArray.Extend": "varArray.extend",
	"sonolus.VarArray.IndexMinFunc": "varArray.indexMinFunc", "sonolus.VarArray.IndexMaxFunc": "varArray.indexMaxFunc",
	"sonolus.VarArray.MinFunc": "varArray.minFunc", "sonolus.VarArray.MaxFunc": "varArray.maxFunc", "sonolus.VarArray.Values": "varArray.values", "sonolus.VarArray.ValuesReversed": "varArray.valuesReversed", "sonolus.VarArray.Items": "varArray.items",
	"sonolus.SortLinkedEntities": "entities.sortLinked", "sonolus.SortDoublyLinkedEntities": "entities.sortDoublyLinked",
	"sonolus.NewArrayMap": "arrayMap.new", "sonolus.ArrayMap.Len": "arrayMap.len", "sonolus.ArrayMap.Capacity": "arrayMap.capacity", "sonolus.ArrayMap.IsFull": "arrayMap.isFull",
	"sonolus.ArrayMap.Get": "arrayMap.get", "sonolus.ArrayMap.Set": "arrayMap.set", "sonolus.ArrayMap.Delete": "arrayMap.delete",
	"sonolus.ArrayMap.Contains": "arrayMap.contains", "sonolus.ArrayMap.Clear": "arrayMap.clear",
	"sonolus.ArrayMap.GetOK": "arrayMap.getOK", "sonolus.ArrayMap.Pop": "arrayMap.pop",
	"sonolus.ArrayMap.Keys": "arrayMap.keys", "sonolus.ArrayMap.Values": "arrayMap.values", "sonolus.ArrayMap.Items": "arrayMap.items",
	"sonolus.NewArraySet": "arraySet.new", "sonolus.ArraySet.Len": "arraySet.len", "sonolus.ArraySet.Capacity": "arraySet.capacity", "sonolus.ArraySet.IsFull": "arraySet.isFull",
	"sonolus.ArraySet.Add": "arraySet.add", "sonolus.ArraySet.Remove": "arraySet.remove", "sonolus.ArraySet.Contains": "arraySet.contains", "sonolus.ArraySet.Clear": "arraySet.clear",
	"sonolus.ArraySet.Values": "arraySet.values",
}

// LookupRecipe is the sole classification point for public DSL symbols. A
// forbidden recipe is deliberate and differs from a missing catalog symbol.
func LookupRecipe(symbol *Symbol) Recipe {
	if symbol == nil {
		return Recipe{Kind: RecipeForbidden, Reason: "symbol is not in the Sonolus catalog"}
	}
	if symbol.Runtime != "" && !symbol.Internal {
		return Recipe{Kind: RecipeRuntime, Runtime: symbol.Runtime}
	}
	if operation, ok := aggregateRecipes[symbol.Key()]; ok {
		return Recipe{Kind: RecipeAggregate, Operation: operation}
	}
	if operation, ok := resourceRecipes[symbol.Key()]; ok {
		return Recipe{Kind: RecipeResource, Operation: operation}
	}
	if operation, ok := containerRecipes[symbol.Key()]; ok {
		return Recipe{Kind: RecipeContainer, Operation: operation}
	}
	if recipe, ok := memoryRecipes[symbol.Key()]; ok {
		return Recipe{Kind: RecipeMemory, Storage: recipe.storage, Offset: recipe.offset, Stride: recipe.stride, IndexArg: recipe.indexArg, Write: recipe.write}
	}
	if recipe, ok := uiMemoryRecipes[symbol.Key()]; ok {
		return Recipe{Kind: RecipeMemory, Storage: recipe.storage, Offset: recipe.offset, Stride: recipe.stride, IndexArg: recipe.indexArg, Write: recipe.write}
	}
	if runtime, ok := facadeRuntimeRecipes[symbol.Key()]; ok {
		return Recipe{Kind: RecipeRuntime, Runtime: runtime}
	}
	if reason, ok := forbiddenRecipes[symbol.Key()]; ok {
		return Recipe{Kind: RecipeForbidden, Reason: reason}
	}
	if symbol.Internal {
		return Recipe{Kind: RecipeForbidden, Reason: "compiler-internal RuntimeFunction"}
	}
	if symbol.Kind == KindType || symbol.Kind == KindConstant {
		return Recipe{Kind: RecipeCompileTime}
	}
	if symbol.Package == "sonolus" {
		switch symbol.Name {
		case "SkinSprite", "EffectClip", "ParticleEffect", "InstructionText", "InstructionIcon", "JudgmentBucket", "JudgmentBucketSprite", "JudgmentBucketSpriteWithFallback":
			return Recipe{Kind: RecipeCompileTime, Reason: "resource constructor"}
		case "SliderOption", "ToggleOption", "SelectOption":
			return Recipe{Kind: RecipeCompileTime, Reason: "configuration constructor"}
		case "Zero", "SlotsOf", "Assert", "Require", "StaticAssert", "RuntimeChecksEnabled", "Unreachable", "Terminate", "Notify":
			return Recipe{Kind: RecipeCompileTime, Reason: "static type utility"}
		}
	}
	return Recipe{Kind: RecipeForbidden, Reason: "callback lowering recipe has not been defined"}
}
