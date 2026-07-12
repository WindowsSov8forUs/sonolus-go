package main

import (
	"math"
	"math/rand"

	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
)

type Note struct {
	play.Archetype `sonolus:"name=Note"`
	Ref            sonolus.EntityRef[Spawned]     `sonolus:"imported"`
	Samples        [2]float64                     `sonolus:"memory"`
	History        sonolus.VarArray[float64]      `sonolus:"memory,cap=4"`
	Points         sonolus.VarArray[sonolus.Vec2] `sonolus:"memory,cap=2"`
}

type Spawned struct {
	play.Archetype `sonolus:"name=Spawned"`
	Imported       float64 `sonolus:"imported"`
	Data           float64 `sonolus:"data"`
	Value          float64 `sonolus:"memory"`
	Shared         float64 `sonolus:"shared"`
	Exported       float64 `sonolus:"exported"`
}

type Skin struct {
	sonolus.SkinResource
	Note sonolus.Sprite
}

var Assets = &Skin{Note: sonolus.SkinSprite("note")}

type EffectsData struct {
	sonolus.EffectResource
	Hit sonolus.Clip
}

var Effects = &EffectsData{Hit: sonolus.EffectClip("hit")}

type ParticlesData struct {
	sonolus.ParticleResource
	Hit sonolus.Effect
}

var Particles = &ParticlesData{Hit: sonolus.ParticleEffect("hit")}

func pair(value float64) (left, right float64) {
	left = value
	right = value + 1
	return
}

func partialPair(value float64) (left, right float64) {
	left = value
	return
}

func identity(value float64) float64 { return value }

func variadicSum(values ...float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total
}

func forwardVariadic(values ...float64) float64 { return variadicSum(values...) }

func pickVariadic(index int, values ...sonolus.Vec2) sonolus.Vec2 { return values[index] }

func apply(function func(float64) float64, value float64) float64 { return function(value) }

func identitySprite(sprite sonolus.Sprite) sonolus.Sprite { return sprite }

func overwrite(value float64) float64 {
	value++
	return value
}

func increment(value *float64) { *value++ }

func genericPair[T ~float64](value T) (left, right T) {
	left = value
	right = value + 1
	return
}

func (n *Note) Preprocess() {
	_ = n.Ref.Index
	for _, sample := range n.Samples {
		play.Debug.Log(sample)
	}
	values := [3]float64{0: 1, 2: 3}
	sum := float64(len(values) + cap(values))
	for i := range 3 {
		sum += float64(i)
	}
	for i := range int(sum) {
		if i > 2 {
			break
		}
		sum += float64(i)
	}
	sum += variadicSum() + variadicSum(1, 2, 3) + forwardVariadic(4, 5)
	literalSum := func(values ...float64) float64 {
		total := 0.0
		for _, value := range values {
			total += value
		}
		return total
	}
	sum += literalSum(6, 7)
	sum += pickVariadic(int(sum), sonolus.NewVec2(1, 2), sonolus.NewVec2(3, 4)).X
	factor := 2.0
	closure := func(value float64) float64 { return value * factor }
	factor = 3
	sum += apply(closure, 2)
	sum += closure(1)
	factor = 4
	sum += closure(1)
	named := identity
	sum += named(4)
	for i, value := range values {
		sum += func(x float64) float64 { return x + float64(i) }(value)
	}
	for _, value := range [2]float64{7, 8} {
		sum += value
	}
	rangeIndex, rangeValue := 0, 0.0
	for rangeIndex, rangeValue = range values {
		sum += float64(rangeIndex) + rangeValue
	}
	_, _ = rangeIndex, rangeValue
	sum += [2]float64{9, 10}[int(sum)]
	holder := struct {
		Prefix float64
		Values [2]float64
	}{Prefix: 4, Values: [2]float64{5, 6}}
	sum += holder.Values[int(sum)]
	_ = sum
	a, b := pair(sum)
	sum += overwrite(1)
	partialLeft, partialRight := partialPair(sum)
	sum += partialLeft + partialRight
	increment(&a)
	a, b = b, a
	var c, d = pair(a)
	c, e := d, c
	f, g := genericPair(a)
	func(float64) {}(g)
	_, _, _, _ = c, d, e, f
	switch {
	case a > b:
		sum = a
	case b > 0:
		sum = b
	default:
		sum = 0
	}
	switch sum {
	case identity(1):
		sum = 1
	default:
	}
	flag := rand.Float64() > 0 && sum > 0
	switch rand.Intn(2) {
	case 0:
		sum++
	default:
		sum--
	}
	_ = flag
	sum += sonolus.Sign(-1) + sonolus.Frac(1.5) + sonolus.Clamp(sum, 0, 10)
	sum += sonolus.Lerp(0, 1, 0.5) + sonolus.LerpClamped(0, 1, 0.5)
	sum += sonolus.Unlerp(0, 1, 0.5) + sonolus.Remap(0, 1, 2, 3, 0.5)
	sum += math.Abs(-1) + math.Floor(1.5) + math.Ceil(1.5) + math.Round(1.5) + math.Trunc(1.5) + math.Log(2)
	sum += math.Sin(1) + math.Cos(1) + math.Tan(1) + math.Asin(0.5) + math.Acos(0.5) + math.Atan(1)
	sum += math.Atan2(1, 2) + math.Min(1, 2) + math.Max(1, 2) + math.Pow(2, 3) + math.Mod(3, 2)
	sum += math.E + math.Pi + math.Phi + math.Sqrt2 + math.SqrtE + math.SqrtPi + math.SqrtPhi
	sum += math.Ln2 + math.Log2E + math.Ln10 + math.Log10E
	_, _ = a, b
	v := sonolus.NewVec2(a, b).Add(sonolus.NewVec2(1, 2)).Mul(2)
	_ = v.Normalize()
	_, _, _, _ = v.Sub(sonolus.NewVec2(1, 1)), v.Div(2), v.Dot(v), v.MagnitudeSquared()
	_, _, _, _ = v.Angle(), v.Orthogonal(), v.NormalizeOrZero(), v.Rotate(0.5)
	_, _, _ = v.RotateAbout(sonolus.NewVec2(1, 1), 0.5), v.AngleDiff(v), v.SignedAngleDiff(v)
	r := sonolus.Rect{T: 2, R: 2, B: -2, L: -2}.Translate(v)
	_, _, _, _, _ = v.Magnitude(), r.Width(), r.Height(), r.Center(), r.Scale(2)
	_ = r.Contains(v)
	q := r.ToQuad()
	_, _, _, _, _ = q.Center(), q.Translate(v), q.Scale(2), q.Rotate(0.5), q.Permute(1)
	_, _, _, _, _ = q.Top(), q.Right(), q.Bottom(), q.Left(), q.Contains(v)
	transform := sonolus.Transform2D{A00: 1, A11: 1}
	transform = transform.Translate(v).Scale(v).Rotate(0.5).Compose(transform).ComposeBefore(transform)
	transform = transform.ScaleAbout(v, v).RotateAbout(0.5, v)
	_, _ = transform.TransformVec(v), transform.TransformQuad(q)
	_ = sonolus.Ease(sonolus.EaseIn, sonolus.EaseSine, 0.5)
	Assets.Note.Draw(r.ToQuad(), 1, 1)
	identitySprite(Assets.Note).Draw(q, 1, 1)
	Assets.Note.DrawCurvedB(r.ToQuad(), v, 8, 1, 1)
	Assets.Note.DrawCurvedT(q, v, 8, 1, 1)
	Assets.Note.DrawCurvedL(q, v, 8, 1, 1)
	Assets.Note.DrawCurvedR(q, v, 8, 1, 1)
	Assets.Note.DrawCurvedBT(q, v, v, 8, 1, 1)
	Assets.Note.DrawCurvedLR(q, v, v, 8, 1, 1)
	_ = Assets.Note.Exists()
	Effects.Hit.Play(0)
	Effects.Hit.PlayScheduled(1, 0)
	loop := Effects.Hit.PlayLooped()
	loop.Stop()
	scheduledLoop := Effects.Hit.PlayLoopedScheduled(1)
	scheduledLoop.Stop(2)
	particle := Particles.Hit.Spawn(r.ToQuad(), 1, false)
	particle.Move(q)
	particle.Destroy()
	list := sonolus.NewVarArray[float64](4)
	list.Append(a)
	list.Append(b)
	list.Set(0, list.Get(1))
	for _, value := range list {
		sum += value
	}
	snapshot := sonolus.NewVarArray[float64](4)
	snapshot.Append(1)
	for range snapshot {
		snapshot.Append(2)
	}
	_ = list.Pop()
	list.Insert(0, a)
	_, _, _ = list.Capacity(), list.IsFull(), list.Contains(a)
	_ = list.Len()
	set := sonolus.NewArraySet[float64](4)
	_ = set.Add(a)
	_ = set.Contains(b)
	_ = set.Remove(a)
	_ = set.Capacity()
	_ = set.Len()
	dict := sonolus.NewArrayMap[float64, float64](4)
	dict.Set(a, b)
	_ = dict.Get(a)
	_ = dict.Contains(b)
	_ = dict.Delete(a)
	_ = dict.Capacity()
	_ = dict.Len()
	points := sonolus.NewArrayMap[float64, sonolus.Vec2](2)
	points.Set(a, v)
	sum += float64(len(points) + cap(points))
	for _, entry := range points {
		sum += entry.First + entry.Second.X
	}
	for _, value := range set {
		sum += value
	}
	list.Clear()
	set.Clear()
	dict.Clear()
	n.History.Append(sum)
	n.Points.Append(v)
	for _, value := range n.History {
		sum += value
	}
	sum += play.Time.Now() + play.Time.Delta() + play.Time.Scaled() + play.Time.Previous()
	sum += play.Time.OffsetAdjusted() + play.Time.BeatToTime(1) + play.Time.TimeToScaledTime(1)
	sum += play.SafeArea.Rect().Width() + play.Screen.Rect().Width() + play.Audio.Offset()
	play.Audio.Play(Effects.Hit, 0)
	play.Audio.PlayScheduled(Effects.Hit, 1, 0)
	audioLoop := play.Audio.PlayLooped(Effects.Hit)
	audioLoop.Stop()
	background := play.Background.Get()
	play.Background.Set(background)
	touch := play.Touches.Get(0)
	sum += touch.Speed + float64(play.Touches.Count())
	skinTransform := play.SkinTransform.Get()
	play.SkinTransform.Set(skinTransform)
	particleTransform := play.ParticleTransform.Get()
	play.ParticleTransform.Set(particleTransform)
	_, _ = play.Entity.Info(), play.Entity.InfoAt(0)
	play.Entity.SetDespawn(play.Entity.Despawn())
	play.Entity.SetResult(play.Entity.Result())
	archetypeScore := play.Score.Archetype(0)
	play.Score.SetArchetype(0, archetypeScore)
	play.Score.SetBase(play.Score.Base())
	play.Score.SetConsecutive(sonolus.JudgmentPerfect, play.Score.Consecutive(sonolus.JudgmentPerfect))
	play.Life.SetInitial(play.Life.Initial())
	play.Life.SetMax(play.Life.Max())
	play.Life.SetArchetype(0, play.Life.Archetype(0))
	play.Life.SetConsecutive(sonolus.JudgmentPerfect, play.Life.Consecutive(sonolus.JudgmentPerfect))
	play.Life.AddScheduled(1, 2)
	_, _ = play.Input.Offset(), play.Input.Judge(1, 1, sonolus.JudgmentWindows{})
	_, _, _, _ = play.Multiplayer.IsMultiplayer(), play.Environment.Debug(), play.Environment.Multiplayer(), play.Environment.AspectRatio()
	play.Streams.Set(0, 1, 2)
	play.Debug.Log(sum)
	_, _ = (sonolus.JudgmentWindow{}).Judge(a, b), (sonolus.JudgmentWindows{}).Judge(a, b)
	layout := play.UI.Menu()
	layout.Alpha = sum
	play.UI.SetMenu(layout)
	play.UI.SetJudgment(play.UI.Judgment())
	play.UI.SetComboValue(play.UI.ComboValue())
	play.UI.SetComboText(play.UI.ComboText())
	play.UI.SetPrimaryMetricBar(play.UI.PrimaryMetricBar())
	play.UI.SetPrimaryMetricValue(play.UI.PrimaryMetricValue())
	play.UI.SetSecondaryMetricBar(play.UI.SecondaryMetricBar())
	play.UI.SetSecondaryMetricValue(play.UI.SecondaryMetricValue())
	sum += play.UI.MenuConfiguration().Scale + play.UI.JudgmentConfiguration().Scale
	sum += play.UI.ComboConfiguration().Scale + play.UI.PrimaryMetricConfiguration().Scale + play.UI.SecondaryMetricConfiguration().Scale
	play.Spawn(Spawned{Imported: 10, Data: 20, Value: sum, Shared: 30, Exported: 40})
}

func (*Note) UpdateParallel() {
	play.Debug.Log(1)
	play.Debug.Pause()
}

func (*Note) Terminate() { play.Debug.Terminate() }

func main() {}
