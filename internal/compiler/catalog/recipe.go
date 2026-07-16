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

	"sonolus/watch.timeAPI.Now":                {"RuntimeUpdate", 0, 0, -1, false},
	"sonolus/watch.timeAPI.Delta":              {"RuntimeUpdate", 1, 0, -1, false},
	"sonolus/watch.timeAPI.Scaled":             {"RuntimeUpdate", 2, 0, -1, false},
	"sonolus/watch.timeAPI.Skip":               {"RuntimeUpdate", 3, 0, -1, false},
	"sonolus/watch.audioAPI.Offset":            {"RuntimeEnvironment", 2, 0, -1, false},
	"sonolus/watch.inputAPI.Offset":            {"RuntimeEnvironment", 3, 0, -1, false},
	"sonolus/watch.replayAPI.IsReplay":         {"RuntimeEnvironment", 4, 0, -1, false},
	"sonolus/watch.backgroundAPI.Get":          {"RuntimeBackground", 0, 0, -1, false},
	"sonolus/watch.backgroundAPI.Set":          {"RuntimeBackground", 0, 0, -1, true},
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

	"sonolus/preview.canvasAPI.Scroll":           {"RuntimeCanvas", 0, 0, -1, false},
	"sonolus/preview.canvasAPI.Size":             {"RuntimeCanvas", 1, 0, -1, false},
	"sonolus/preview.canvasAPI.Set":              {"RuntimeCanvas", 0, 0, -1, true},
	"sonolus/preview.entityAPI.Info":             {"CurrentEntityInfo", 0, 0, -1, false},
	"sonolus/preview.entityAPI.InfoAt":           {"EntityInfo", 0, 2, 0, false},
	"sonolus/preview.environmentAPI.Debug":       {"RuntimeEnvironment", 0, 0, -1, false},
	"sonolus/preview.environmentAPI.AspectRatio": {"RuntimeEnvironment", 1, 0, -1, false},

	"sonolus/tutorial.timeAPI.Now":                {"RuntimeUpdate", 0, 0, -1, false},
	"sonolus/tutorial.timeAPI.Delta":              {"RuntimeUpdate", 1, 0, -1, false},
	"sonolus/tutorial.backgroundAPI.Get":          {"RuntimeBackground", 0, 0, -1, false},
	"sonolus/tutorial.backgroundAPI.Set":          {"RuntimeBackground", 0, 0, -1, true},
	"sonolus/tutorial.navigationAPI.Direction":    {"RuntimeUpdate", 2, 0, -1, false},
	"sonolus/tutorial.memoryAPI.Get":              {"TutorialMemory", 0, 1, 0, false},
	"sonolus/tutorial.memoryAPI.Set":              {"TutorialMemory", 0, 1, 0, true},
	"sonolus/tutorial.dataAPI.Get":                {"TutorialData", 0, 1, 0, false},
	"sonolus/tutorial.environmentAPI.Debug":       {"RuntimeEnvironment", 0, 0, -1, false},
	"sonolus/tutorial.environmentAPI.AspectRatio": {"RuntimeEnvironment", 1, 0, -1, false},
}

var facadeRuntimeRecipes = map[string]resource.RuntimeFunction{
	"sonolus.Sign":                            resource.RuntimeFunctionSign,
	"sonolus.Frac":                            resource.RuntimeFunctionFrac,
	"sonolus.Clamp":                           resource.RuntimeFunctionClamp,
	"sonolus.Lerp":                            resource.RuntimeFunctionLerp,
	"sonolus.LerpClamped":                     resource.RuntimeFunctionLerpClamped,
	"sonolus.Unlerp":                          resource.RuntimeFunctionUnlerp,
	"sonolus.Remap":                           resource.RuntimeFunctionRemap,
	"sonolus/play.audioAPI.Play":              resource.RuntimeFunctionPlay,
	"sonolus/play.audioAPI.PlayScheduled":     resource.RuntimeFunctionPlayScheduled,
	"sonolus/play.audioAPI.PlayLooped":        resource.RuntimeFunctionPlayLooped,
	"sonolus/play.debugAPI.Log":               resource.RuntimeFunctionDebugLog,
	"sonolus/play.debugAPI.Pause":             resource.RuntimeFunctionDebugPause,
	"sonolus/play.inputAPI.Judge":             resource.RuntimeFunctionJudge,
	"sonolus/play.lifeAPI.AddScheduled":       resource.RuntimeFunctionAddLifeScheduled,
	"sonolus/play.streamsAPI.Set":             resource.RuntimeFunctionStreamSet,
	"sonolus/play.timeAPI.BeatToTime":         resource.RuntimeFunctionBeatToTime,
	"sonolus/play.timeAPI.TimeToScaledTime":   resource.RuntimeFunctionTimeToScaledTime,
	"sonolus/watch.audioAPI.Play":             resource.RuntimeFunctionPlay,
	"sonolus/watch.audioAPI.PlayScheduled":    resource.RuntimeFunctionPlayScheduled,
	"sonolus/watch.debugAPI.Log":              resource.RuntimeFunctionDebugLog,
	"sonolus/watch.debugAPI.Pause":            resource.RuntimeFunctionDebugPause,
	"sonolus/watch.inputAPI.Judge":            resource.RuntimeFunctionJudge,
	"sonolus/watch.lifeAPI.AddScheduled":      resource.RuntimeFunctionAddLifeScheduled,
	"sonolus/watch.streamsAPI.Has":            resource.RuntimeFunctionStreamHas,
	"sonolus/watch.streamsAPI.PreviousKey":    resource.RuntimeFunctionStreamGetPreviousKey,
	"sonolus/watch.streamsAPI.NextKey":        resource.RuntimeFunctionStreamGetNextKey,
	"sonolus/watch.streamsAPI.Value":          resource.RuntimeFunctionStreamGetValue,
	"sonolus/watch.timeAPI.BeatToTime":        resource.RuntimeFunctionBeatToTime,
	"sonolus/preview.canvasAPI.Print":         resource.RuntimeFunctionPrint,
	"sonolus/preview.debugAPI.Log":            resource.RuntimeFunctionDebugLog,
	"sonolus/preview.debugAPI.Pause":          resource.RuntimeFunctionDebugPause,
	"sonolus/tutorial.audioAPI.Play":          resource.RuntimeFunctionPlay,
	"sonolus/tutorial.audioAPI.PlayScheduled": resource.RuntimeFunctionPlayScheduled,
	"sonolus/tutorial.debugAPI.Log":           resource.RuntimeFunctionDebugLog,
	"sonolus/tutorial.debugAPI.Pause":         resource.RuntimeFunctionDebugPause,
	"sonolus/tutorial.instructionAPI.Paint":   resource.RuntimeFunctionPaint,
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
	"sonolus/play.debugAPI.Terminate": "control.terminate",
	"sonolus/play.touchesAPI.Get":     "touch.get",
	"sonolus/play.timeAPI.Previous":   "time.previous", "sonolus/watch.timeAPI.Previous": "time.previous",
	"sonolus/play.timeAPI.OffsetAdjusted": "time.offsetAdjusted",
	"sonolus/play.screenAPI.Rect":         "screen.rect",
	"sonolus/watch.screenAPI.Rect":        "screen.rect",
	"sonolus/preview.screenAPI.Rect":      "screen.rect",
	"sonolus/tutorial.screenAPI.Rect":     "screen.rect",
	"sonolus.NewVec2":                     "vec2.new", "sonolus.Vec2.Add": "vec2.add", "sonolus.Vec2.Sub": "vec2.sub",
	"sonolus.Vec2.Mul": "vec2.mul", "sonolus.Vec2.Div": "vec2.div", "sonolus.Vec2.Dot": "vec2.dot",
	"sonolus.Vec2.MagnitudeSquared": "vec2.magnitudeSquared", "sonolus.Vec2.Magnitude": "vec2.magnitude",
	"sonolus.Vec2.Angle": "vec2.angle", "sonolus.Vec2.Orthogonal": "vec2.orthogonal",
	"sonolus.Vec2.Normalize": "vec2.normalize", "sonolus.Vec2.NormalizeOrZero": "vec2.normalizeOrZero",
	"sonolus.Vec2.Rotate": "vec2.rotate", "sonolus.Vec2.RotateAbout": "vec2.rotateAbout",
	"sonolus.Vec2.AngleDiff": "vec2.angleDiff", "sonolus.Vec2.SignedAngleDiff": "vec2.signedAngleDiff",
	"sonolus.Rect.Width": "rect.width", "sonolus.Rect.Height": "rect.height", "sonolus.Rect.Center": "rect.center",
	"sonolus.Rect.Translate": "rect.translate", "sonolus.Rect.Scale": "rect.scale",
	"sonolus.Rect.ToQuad": "rect.toQuad", "sonolus.Rect.Contains": "rect.contains",
	"sonolus.Quad.Center": "quad.center", "sonolus.Quad.Translate": "quad.translate", "sonolus.Quad.Scale": "quad.scale",
	"sonolus.Quad.Rotate": "quad.rotate", "sonolus.Quad.Top": "quad.top", "sonolus.Quad.Right": "quad.right",
	"sonolus.Quad.Bottom": "quad.bottom", "sonolus.Quad.Left": "quad.left", "sonolus.Quad.Permute": "quad.permute",
	"sonolus.Quad.Contains":         "quad.contains",
	"sonolus.Transform2D.Translate": "transform.translate", "sonolus.Transform2D.Scale": "transform.scale",
	"sonolus.Transform2D.Rotate": "transform.rotate", "sonolus.Transform2D.Compose": "transform.compose",
	"sonolus.Transform2D.ComposeBefore": "transform.composeBefore", "sonolus.Transform2D.ScaleAbout": "transform.scaleAbout",
	"sonolus.Transform2D.RotateAbout": "transform.rotateAbout", "sonolus.Transform2D.TransformVec": "transform.vec",
	"sonolus.Transform2D.TransformQuad": "transform.quad",
}

var forbiddenRecipes = map[string]string{}

var resourceRecipes = map[string]string{
	"sonolus/play.Spawn":  "archetype.spawn",
	"sonolus.Sprite.Draw": "sprite.draw", "sonolus.Sprite.Exists": "sprite.exists",
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
	"sonolus.VarArray.IsFull": "varArray.isFull", "sonolus.VarArray.Get": "varArray.get", "sonolus.VarArray.Set": "varArray.set",
	"sonolus.VarArray.Append": "varArray.append", "sonolus.VarArray.Pop": "varArray.pop", "sonolus.VarArray.Clear": "varArray.clear",
	"sonolus.VarArray.Contains": "varArray.contains", "sonolus.VarArray.Insert": "varArray.insert",
	"sonolus.NewArrayMap": "arrayMap.new", "sonolus.ArrayMap.Len": "arrayMap.len", "sonolus.ArrayMap.Capacity": "arrayMap.capacity",
	"sonolus.ArrayMap.Get": "arrayMap.get", "sonolus.ArrayMap.Set": "arrayMap.set", "sonolus.ArrayMap.Delete": "arrayMap.delete",
	"sonolus.ArrayMap.Contains": "arrayMap.contains", "sonolus.ArrayMap.Clear": "arrayMap.clear",
	"sonolus.NewArraySet": "arraySet.new", "sonolus.ArraySet.Len": "arraySet.len", "sonolus.ArraySet.Capacity": "arraySet.capacity",
	"sonolus.ArraySet.Add": "arraySet.add", "sonolus.ArraySet.Remove": "arraySet.remove", "sonolus.ArraySet.Contains": "arraySet.contains", "sonolus.ArraySet.Clear": "arraySet.clear",
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
		}
	}
	return Recipe{Kind: RecipeForbidden, Reason: "callback lowering recipe has not been defined"}
}
