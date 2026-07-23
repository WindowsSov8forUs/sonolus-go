# Go DSL 参考

## 基本原则

编译器接受的是静态可分析的 Go 子集，而不是任意合法 Go 程序。所有 Sonolus API 通过 `go/types.Object` 身份与 catalog 项匹配，因此 import alias 合法，同名用户函数不能伪造 intrinsic 或资源 constructor。

引擎 `package main` 只是 compiler 输入，不是可通过普通 Go 执行来观察玩法行为的程序。`sonolus` 目录及其模式子包全部是 API 声明桩；函数体只用于 Go 类型检查，实际语义由 frontend lowering 与 catalog 决定。即使引擎源码能够被 `go build` 或 `go run` 接受，零值返回和空操作也不代表最终 EngineData 或 Sonolus Runtime 行为。

四种模式使用 build tag `play`、`watch`、`preview`、`tutorial` 独立加载。未带模式 tag 的文件会进入每种模式。

## Development Level

`sonolus.LevelFile` 是 `dev` 命令识别的 embed 声明桩，不进入 callback lowering 或 `compiler.Artifacts`：

```go
//sonolus:level basic
//go:embed basic.json
var BasicLevel sonolus.LevelFile
```

每个变量必须恰好 embed 一个 LevelData JSON。可以声明多个开发关卡；多关卡时每个 `//sonolus:level` 必须带一个唯一 item name，显示标题由变量名拆分生成。单个无参数声明继续映射为 `dev` / `Dev Level`。全部声明必须同时对 Play、Watch、Preview 可见，变量、item name 与 embed 文件在三个模式中必须一致；加载器按变量名稳定排序，并逐个执行三模式 archetype/import 和实体引用校验。任一关卡失败都不会替换 `dev` 当前提供的完整成功快照。

## 名称型资源

资源由嵌入强类型 marker 的结构和唯一 singleton 变量声明：

```go
type SkinData struct {
	sonolus.SkinResource

	Note    sonolus.Sprite
	Digits  [10]sonolus.Sprite
}

var Skin = &SkinData{
	SkinResource: sonolus.SkinResource{RenderMode: sonolus.RenderModeStandard},
	Note: sonolus.SkinSprite("#NOTE_HEAD_CYAN"),
	Digits: [10]sonolus.Sprite{
		sonolus.SkinSprite("digit-0"),
		// ...
	},
}
```

支持的名称型资源和 constructor：

| Marker | 字段类型 | Constructor |
|---|---|---|
| `sonolus.SkinResource` | `sonolus.Sprite` | `sonolus.SkinSprite(name)` |
| `sonolus.EffectResource` | `sonolus.Clip` | `sonolus.EffectClip(name)` |
| `sonolus.ParticleResource` | `sonolus.Effect` | `sonolus.ParticleEffect(name)` |
| `sonolus.InstructionResource` | `sonolus.Text` | `sonolus.InstructionText(name)` |
| `sonolus.InstructionIconResource` | `sonolus.Icon` | `sonolus.InstructionIcon(name)` |

Handle 是空结构，用户无法直接指定 ID。名称只从准确 constructor 调用的静态字符串参数中提取。字段顺序和定长数组元素顺序决定连续资源 ID。

资源与 Configuration 是引擎共享声明，应放在不带模式 build tag 的文件中。带 `play`、`watch`、`preview` 或 `tutorial` build tag 的文件用于对应模式的 archetype 与 callback。

四种模式的 frontend 都解析并校验完整资源声明，backend 按目标 EngineData 的字段选择输出：

| 模式 | 资源 |
|---|---|
| Play | skin、effect、particle、buckets |
| Watch | skin、effect、particle、buckets |
| Preview | skin |
| Tutorial | skin、effect、particle、instruction、instructionIcon |

资源 ID 由共享声明中的字段顺序确定。目标 EngineData 没有对应字段时，backend 不输出该类资源。

## Bucket

Bucket 有独立结构，不属于名称型资源：

```go
type BucketData struct {
	sonolus.BucketsResource

	Tap sonolus.Bucket
}

var Buckets = &BucketData{
	Tap: sonolus.JudgmentBucket(
		"#MILLISECONDS",
		sonolus.JudgmentBucketSprite(Skin.Note, 0, 0, 1, 1, 0),
	),
}
```

Bucket constructor 必须是 catalog 中的准确对象，参数必须能在声明阶段静态解释。

Play/Watch 的 preprocess 可通过强类型 window API 初始化 `LevelBucket`：

```go
window := sonolus.JudgmentWindows{
    Perfect: sonolus.NewRange(-50, 50),
    Great:   sonolus.NewRange(-100, 100),
    Good:    sonolus.NewRange(-150, 150),
}
Buckets.Tap.SetWindow(window)
current := Buckets.Tap.Window()
```

每个 Bucket 按声明 ID 映射到六个连续 slot；`SetWindow` 只允许在 Play/Watch preprocess 写入，`Window` 在 Play/Watch 中读取同一布局。通常 bucket 使用毫秒单位，因此应把秒制输入判定窗口乘以 1000 后写入。

## Configuration

Configuration option 使用强类型静态构造器声明：

```go
type GameConfiguration struct {
	sonolus.Configuration

	Speed  float64
	Auto   bool
	Lane   int
	UI     sonolus.UIConfig
	Replay []string
}

var Config = GameConfiguration{
	Speed: sonolus.SliderOption(sonolus.SliderOptionConfig{
		Name: "Speed", Default: 1, Min: 0.5, Max: 2, Step: 0.1, Unit: "#TIMES",
	}),
	Auto: sonolus.ToggleOption(sonolus.ToggleOptionConfig{Name: "Auto"}),
	Lane: sonolus.SelectOption(sonolus.SelectOptionConfig{
		Name: "Lane", Default: 0, Values: []string{"4", "6", "8"},
	}),
	UI: sonolus.UIConfig{
		Scope:          "game",
		PrimaryMetric:  sonolus.UIMetricArcade,
		SecondaryMetric: sonolus.UIMetricLife,
	},
	Replay: []string{"Speed", "Lane"},
}
```

`UIConfig` 采用与 Py `UiConfig` 相同的声明默认值：省略的 metric、visibility、animation、judgment error style 与 placement 分别补为 `arcade`、`life`、`scale=1/alpha=1`、Py 默认 tween、`late` 和 `top`。Static tracer 会保留 struct literal 字段是否显式出现，因此 `UIVisibility{Scale: 0, Alpha: 0}` 仍是显式隐藏，而不是被默认值覆盖。

公开枚举完整覆盖 Sonolus wire schema：12 种 `UIMetric`、`linear/none` 及四个方向组合的八组 `UIEase`、13 种 `UIJudgmentErrorStyle` 和 7 种 `UIJudgmentErrorPlacement`。Compiler 会在声明阶段拒绝未知字符串；最终 Configuration 可由 `sonolus-core-go` 严格反序列化。

规则：

- `float64`、`bool`、`int` 字段分别使用 `SliderOption`、`ToggleOption`、`SelectOption` 初始化。
- option metadata 使用对应的强类型 config；空 `Name` 默认使用 Go 字段名。
- `SelectOptionConfig.Default` 是 `Values` 的索引，values 必须是非空静态字符串列表。
- `sonolus.UIConfig` 字段自动作为 UI 配置识别，最多一个。
- `[]string` 字段自动作为 replay fallback 识别，最多一个；其值必须引用 option 的最终外部名称，禁止空值和重复。

## ROM

ROM 声明不是必需的。不需要读取 ROM 时建议不要声明；当源码没有 ROM 声明、没有提供 fallback，且优化后的 callback IR 不读取 ROM 时，compiler 不生成 `EngineRom`。

静态 ROM：

```go
var ROM = sonolus.ROMValues{1, 2, 3}
```

嵌入 ROM：

```go
import _ "embed"

//go:embed rom.bin
var ROM sonolus.ROMFile
```

Go 要求包含 `//go:embed` directive 的源文件导入 `embed`。这里没有直接使用 `embed.FS` 等包内标识符，所以使用空白导入；它只负责让 Go 接受并处理 directive，实际 ROM 变量仍使用 `sonolus.ROMFile` marker。

只允许一个 ROM 声明。`ROMFile` 必须精确绑定一个文件，文件长度必须是 4 的倍数。用户 ROM 按 little-endian float32 保存；需要生成 ROM 时，backend 会在前面添加 NaN、`+Inf`、`-Inf` 三个固定值。显式的空 `ROMValues{}` 仍视为 ROM 声明并生成只含这三个固定值的 `EngineRom`。

## Archetype

标准 timing 声明除 `ArchetypeBPMChange`、`ArchetypeTimeScaleChange`、`ImportBeat`、`ImportBPM` 与 `ImportTimeScale` 外，还包括 `ArchetypeTimeScaleGroup`、`ImportTimeScaleSkip`、`ImportTimeScaleGroup`、`ImportTimeScaleEase` 和 `TimescaleEaseNone/Linear`。这些常量只提供准确的标准名称与数值，不改变普通 Archetype tag 的布局规则。

```go
type TapNote struct {
	play.Archetype      `archetype:"name=TapNote,hasInput=true"`
	play.CallbackOrders `archetype:"preprocess=-10,updateSequential=5"`

	Beat     float64          `archetype:"imported,name=#BEAT,default=0"`
	Data     float64          `archetype:"data"`
	Position sonolus.Vec2     `archetype:"memory"`
	Shared   float64          `archetype:"shared"`
	HitTime  float64          `archetype:"exported,name=hitTime"`
}
```

Storage：

| Tag | 含义 |
|---|---|
| `imported` | Entity Data import，可配置 `name` 和 `default` |
| `data` | 对应模式的 Entity Data storage |
| `memory` | 当前实体 memory |
| `shared` | archetype shared memory |
| `exported` | Play export，仅 Play 可用，必须提供稳定外部名称 |

固定记录、定长数组和 catalog 容器按 catalog layout 占用多个连续 slot。`VarArray`、`ArrayMap`、`ArraySet` 必须带编译期可确定的 capacity/backing。

`imported` 与 `exported` 字段按 runtime slot 展开外部名称：数组使用 `name[i]`，多槽 record 使用 `name.field`，单槽 record 折叠回 `name`。例如 `Position sonolus.Vec2` 且 `name=position` 会生成 `position.x`、`position.y`。多槽 imported 字段不能用单一 `default=`；展开后的名称在当前类型及继承布局中都必须唯一。这与 Py 的 `_flat_keys_` 规则一致，也同时决定 `list` 输出和 Development Level 字段校验。

Archetype 支持一个直接匿名嵌入的基类，例如匿名字段 `Base` 使用 `archetype:"base"` tag。基类必须在当前模式中声明；字段按 storage 分区成为派生布局前缀，callback method、order 与 runtime 类型关系沿单继承链继承，派生直接方法和显式 order 会覆盖基类。

无 Sonolus declaration marker 的普通具名 struct 可不带 tag 匿名嵌入 Archetype，作为 structural mixin 复用字段与 callback：

```go
// package shared
type BasicNote struct {
    Beat float64 `archetype:"imported,name=#BEAT"`
    Time float64 `archetype:"data"`
}

func (note *BasicNote) Preprocess() {
    note.Time = note.Beat
}

// package main
type TapNote struct {
    play.Archetype `archetype:"name=TapNote,hasInput=true"`
    shared.BasicNote
}
```

structural mixin 按匿名嵌入位置递归、深度优先展开其中带 storage tag 的字段，promoted callback 在每个使用它的具体 Archetype 和模式中分别校验与 lowering；未带 storage tag 的普通字段不进入 runtime layout。同一个 mixin 可被不同 Archetype 或不同模式复用，但同一具体 Archetype 中不得经多条嵌入路径重复出现。mixin 不进入 Archetype declaration 或 MRO，不取得 runtime ID/key，不继承 callback order，也不能作为 `EntityRef[T]`、`CurrentEntityRef[T]`、`ArchetypeID[T]` 或 `ArchetypeKey[T]` 的 Archetype 目标。需要这些 runtime 类型关系时仍应使用当前模式的 `archetype:"base"` 单继承。

只作为字段/callback 模板而不进入 EngineData 的基类使用 `archetype:"abstract"`：

```go
type BaseNote struct {
    play.Archetype `archetype:"abstract"`
    Beat float64 `archetype:"imported,name=#BEAT"`
}

type TapNote struct {
    BaseNote `archetype:"base"`
    play.Archetype `archetype:"name=TapNote,hasInput=true"`
}
```

abstract archetype 不得声明 runtime name 或 `hasInput`，不会出现在 EngineData、`list` schema 或 Development Level 契约中，但可作为 `EntityRef[T]` 的静态目标；non-strict match 与 checked `Get` 接受任意具体派生类型，strict match 永远不把 abstract base 当作实际 runtime archetype。abstract archetype 不能被 `Spawn`。

具体 Archetype 可在 marker 中声明有限数值 key，例如 `archetype:"name=TapNote,key=1"`。未声明时 key 为 `-1`，派生类型默认继承基类 key。`play/watch/preview.Entity.Key()` 返回当前 callback 所属具体 Archetype 的 key；`EntityRef[T].Key()` 根据引用实体的实际 runtime archetype 返回 key，即使 `T` 是 abstract base 或 `AnyArchetype`。这与 Py 的 entity key 语义一致，不要求调用方枚举 runtime archetype ID。

`sonolus.EntityRef[T].Get()` 返回目标 Archetype 的类型化 runtime view。`GetUnchecked` 跳过引用检查；`EntityRefAs`、`EntityRefMatches` 与 `EntityRefGetAs` 支持 `AnyArchetype` 和继承关系下的静态转换、strict/non-strict 匹配与访问。启用 runtime checks 时，`Get`/`GetAs` 会验证非负 index 与目标 MRO。

Archetype callback 内可使用 `play.CurrentEntityRef[T]()`、`watch.CurrentEntityRef[T]()` 或 `preview.CurrentEntityRef[T]()` 获取当前实体引用。`T` 必须是当前具体 Archetype、其 MRO 中的基类或 `sonolus.AnyArchetype`；这对应 Py 的 `self.ref()`，无需手工读取 `Entity.Info().Index` 后构造引用。

Play、Watch 与 Preview 可使用对应模式的 `ArchetypeID[T]()` 与 `ArchetypeKey[T]()` 取得具体 Archetype 的编译期 runtime ID 和 key；未声明 key 时返回 `-1`。`T` 必须是当前模式中非 abstract 的准确声明；这些值适合在 Stage 等全局实体中一次性初始化 archetype score/life 等按 ID 排列的 runtime 表。abstract base 没有单一 runtime ID/key，因此稳定拒绝。这是 Go 对 Py “将 archetype class 作为静态值传入 helper”的类型安全替代。

视图允许读取 `imported`、`data` 与 `shared` 字段，并在对应 callback 阶段写入允许写入的 `data`/`shared` 字段；其他实体的 `memory` 与 `exported` 字段始终拒绝访问。视图可通过 helper 返回、保存到同一静态目标类型的本地 struct、定长数组或 catalog container，并按 entity index 比较；仍不能进入 callback 返回、Archetype/resource/static storage、接口，或对字段取地址。动态实体 index 与嵌套数组 index 会组合成单一 affine memory address，每层边界检查独立保留，且各 index 只求值一次。

## Replay Streams

Play 与 Watch 可用同构的 `StreamResource` singleton 声明强类型 replay streams。typed stream ID 从 1 开始；`Stream[T]` 按 `T` 的 slot 数占用连续 ID，定长数组继续连续展开。`StreamData[T]` 始终只占一个 ID，Play 使用 `Set(value)` 将各 slot 写入整数 key 并补齐半整数 sentinel，Watch 使用 `Get()` 从同一 backing stream 的 slot key 读取完整值。

`Stream[T]` 在 Play 使用 `Set`，在 Watch 使用 `Has`、`Get`、前后 key/value 查询，以及正序、倒序和 previous-frame iterator。Play/Watch 声明必须逐字段一致。存在 typed stream 声明时，低层 `play.Streams`/`watch.Streams` 的常量 ID 不得落入其保留区；动态 ID 无法证明不冲突，因此会被拒绝。

## Level Memory 与 Level Data

跨 Archetype 或全局 callback 的结构化状态使用 `LevelMemoryResource` 与 `LevelDataResource`：

```go
type InputMemoryData struct {
    sonolus.LevelMemoryResource
    ClaimedTouches sonolus.ArraySet[float64]
}

var InputMemory = InputMemoryData{
    ClaimedTouches: sonolus.NewArraySet[float64](16),
}

type ComputedLayoutData struct {
    sonolus.LevelDataResource
    LastTime float64
    Points   [4]sonolus.Vec2
}

var ComputedLayout = ComputedLayoutData{}
```

marker 必须直接以 value 形式嵌入 named struct；每个声明类型必须对应恰好一个具有显式静态 initializer 的 package singleton。一个模式可以声明多个 memory/data singleton；Compiler 按 package path、类型名和字段顺序在对应 runtime block 中连续分配，不需要手工维护 offset。普通字段必须保持零初始化；`VarArray`、`ArrayMap` 与 `ArraySet` 字段使用准确的 `New*` 构造器声明静态 capacity，运行时初始内容仍为空。

`LevelMemoryResource` 在 Play、Watch、Tutorial 可用，分别映射 `LevelMemory`、`LevelMemory`、`TutorialMemory`。Play 只允许 `Preprocess`、`UpdateSequential`、`Touch` 写入；Watch 只允许 `Preprocess`、`UpdateSequential` 写入；Tutorial 的三个全局 callback 均可写。

`LevelDataResource` 在四种模式均可用，分别映射 Play/Watch `LevelData`、`PreviewData` 与 `TutorialData`。所有模式都只允许 `Preprocess` 写入，其他 callback 为只读，因此 Standard 优化可安全提升和合并只读 load。

字段可递归包含普通 record、定长数组与 `VarArray`、`ArrayMap`、`ArraySet`。嵌套 container 仍必须在 singleton initializer 中使用准确的 `New*` 构造器；定长数组的每个元素必须具有相同 capacity/layout，因此常量和动态索引都能映射到稳定 backing。整个结构不会复制到 Temporary Memory，selector 与每层数组索引直接派生 semantic memory place，并分别保留边界检查。

低层 `play.LevelMemory/LevelData`、`watch.LevelMemory/LevelData`、`preview.LevelData` 与 `tutorial.LevelMemory/LevelData` 的 `Get/Set` 仍作为显式 offset escape hatch；其写权限与结构化 marker 完全一致。Tutorial 原有的 `TutorialMemory`/`TutorialData` 名称也映射同一 block。除非需要与外部固定 offset 协议交互，优先使用结构化声明。

## 数值与几何值

`Vec2`、`Rect`、`Quad`、`Transform2D` 与 `InvertibleTransform2D` 提供固定 slot 的几何运算；调用会直接 lower 为纯 Runtime 表达式，不创建运行时 Go 对象。`RectFromCenter`、`RectFromMargin`、vector scale、scale/rotate about center、expand/shrink 对应 Py 的常用 Quad/Rect helper。

`Transform2D` 是九槽二维齐次矩阵，不是六槽 affine 简写。`IdentityTransform2D` 创建单位矩阵；除 translate、scale、rotate 与 compose 外，还支持 shear、simple/full perspective、inverse perspective、normalize，以及带透视除法的 vector/quad transform。九个槽映射到 Sonolus 4×4 Skin/Particle Transform 的 `0,1,3,4,5,7,12,13,15` 偏移。`PerspectiveApproach` 提供与 Py 相同的透视正确 approach progress。

`InvertibleTransform2D` 同时维护 forward 与 inverse 矩阵，提供相同的可逆变换链和正反向 vector/quad transform。Play、Watch 与 Tutorial 提供 `SkinTransform`、`ParticleTransform`；Preview 提供 `SkinTransform`。写入权限遵循各模式 runtime transform block：Preview 仅 preprocess，Play/Watch 与对应 LevelMemory 写阶段一致。

共享 helper 可使用 `IsPlay`、`IsWatch`、`IsPreview`、`IsTutorial` 与 `IsPreprocessing`。这些调用在 frontend 中直接专门化为编译期常量，不读取 runtime mode tag，也不会在最终 EngineData 中留下动态分支。

`sonolus.Range` 是两槽闭区间，对应 Py 的 `Interval` 能力。除 `Min`/`Max` 字段外，可使用 `Length`、`IsEmpty`、`Mid`、标量或区间包含检查、`Add/Sub/Mul/Div`、`Intersect`、`Shrink/Expand`、`Lerp/Unlerp` 及其 clamped 形式和 `Clamp`。Go 不支持用户自定义运算符，因此 Py 的 `interval + value`、`value in interval` 分别写为 `interval.Add(value)`、`interval.Contains(value)`。

`sonolus.Ease` 使用编译期常量 direction 与 curve 选择完整的 In、Out、InOut、OutIn easing RuntimeFunction；`Linstep`、`Smoothstep`、`Smootherstep`、`StepStart` 与 `StepEnd` 对应 Py easing 模块的静态数值 helper。标量 `Lerp`、`Unlerp`、`Remap` 均同时提供 clamped 形式；Py 接受任意 ArrayLike 的 `interp` 不直接复制，Go 引擎使用固定数组或显式分段 helper 表达相同算法。

`UnitVec2(angle)` 构造单位方向向量，`Vec2.Lerp` 与 `Vec2.LerpClamped` 提供 Py 泛型 lerp 在 Go 几何类型上的强类型对应。

四种模式的 `Time` facade 均完整暴露 `BeatToBPM`、`BeatToTime`、`BeatToStartingBeat`、`BeatToStartingTime`、`TimeToScaledTime`、`TimeToStartingScaledTime`、`TimeToStartingTime` 与 `TimeToTimeScale`。Play、Watch 与 Tutorial 还提供当前帧时间读取，Watch 额外提供 `Skip`。业务引擎优先使用这些接口，`sonolus/native` 只保留给没有高层语义包装的显式 Runtime escape hatch。

## Callback

Callback 名由方法名决定，必须是无参数 receiver 方法：

| 模式 | Callback | 返回值 |
|---|---|---|
| Play | `Preprocess`、`Initialize`、`UpdateSequential`、`Touch`、`UpdateParallel`、`Terminate` | 无 |
| Play | `SpawnOrder` | `float64` |
| Play | `ShouldSpawn` | `bool` |
| Watch | `Preprocess`、`Initialize`、`UpdateSequential`、`UpdateParallel`、`Terminate` | 无 |
| Watch | `SpawnTime`、`DespawnTime` | `float64` |
| Preview | `Preprocess`、`Render` | 无 |

`CallbackOrders` 中的 key 使用 lower camel callback 名。不存在对应方法、未知 callback 或重复 marker 都会报错。

全局 callback：

- Watch：嵌入 `watch.GlobalCallbacks` 后可选 `func UpdateSpawn() float64`。
- Tutorial：嵌入 `tutorial.GlobalCallbacks` 后可选 `func Preprocess()`、`Navigate()`、`Update()`。

## 支持的 Go 子集

支持：

- 常量、局部变量、短声明、普通/复合/多重赋值、自增自减。
- 标量、已登记 struct 和定长数组复合值，字段访问和数组索引。
- 算术、比较、逻辑短路、显式数值转换，以及 `go/types` 可证明的 bool/int/float constant builtin 结果（例如 `len("static")`）；这不会开放 runtime string。
- `if/else`、三段式与条件 `for`、expression/type `switch`、`fallthrough`、`goto`、带标签或无标签的 `break`/`continue`、`return`。`goto` 直接 lower 为函数内 CFG 跳转；跨作用域、跳过变量声明等非法目标继续由 Go 类型检查器拒绝。
- 单次注册的 `defer`，包括 runtime 分支中的 defer、参数立即求值、闭包引用捕获、命名返回值修改与 LIFO 执行。循环中的 defer 或与 `goto` 共存的 defer 可能重复注册，需要 Sonolus 不具备的 runtime defer stack，因此稳定拒绝。
- `range` 整数、定长数组、定长数组指针、静态 variadic 参数和 catalog 容器。与 Go 一致，数组值在进入 value iteration 前复制快照；数组指针表达式只求值一次并从原 backing 实时读取。只迭代 index 时允许 nil 数组指针，因为不会发生解引用。`len`/`cap` 同样接受定长数组指针并直接返回静态长度。
- Go 1.25 range-over-func 的 0/1/2 value yield、`break`、`continue` 和外层 `return`。
- `play.Touches.Values()` 与 `play.Touches.Items()` 提供当前 frame touch 集合的 value/index-value iterator；进入 iterator 时冻结 touch count，每个 Touch 从原 RuntimeTouch backing 读取。低层 `Count/Get` 保留为显式索引访问。
- 时间 facade 与 Py 的模式语义一致：Play、Watch、Tutorial 都提供 `Now`、`Delta`、`Scaled`、`Previous` 与 `OffsetAdjusted`；Tutorial 的 `Scaled`、`OffsetAdjusted` 等于 `Now`，`Previous` 为 `Now-Delta`。Preview 只提供谱面 timing 转换，不伪造 runtime frame 时间。
- 跨用户包 helper、具体方法、泛型实例、立即调用闭包、命名/多返回值和裸 return。泛型实例在 inline frame 内使用具体 type substitution，因此 scalar、`[N]T`、`struct{...T}`、generic named type、`*T`、`Zero[T]` 与 `SlotsOf[T]` 都按实例化后的多槽 layout lowering。
- 返回闭包、bound method value、具体类型与静态 interface 的 method expression（`T.M`、`(*T).M`、`I.M`）、package 级不可变 callable、局部 callable/参数的普通重赋值与有限 runtime target dispatch，以及由编译期常量状态证明终止的最多 256 层递归。局部 callable 即使在 runtime 分支内声明也保持静态目标；nil callable 调用在所有 runtime-check 等级终止 callback，`notify` 额外发出诊断。method expression 的显式 receiver 严格求值一次，interface receiver 继续通过 finite concrete-type variant 去虚拟化，不生成 runtime interface 对象。
- 一维固定 `[N]func(...) ...` 数组的 literal、零值、值复制、动态索引、元素重赋值、value range、helper 参数与返回。每个元素保存有限 target tag；数组赋值和 value range 复制当时目标，后续修改不会反向影响快照。具有静态 initializer 的 package 数组可作为不可变函数表使用，对应 pydori 的 `PHASES` 等 module-level callable tuple；callback 内不得修改其元素或整体值。
- 普通 package scalar、record 与定长数组同样要求 pure static initializer，并在 callback 中保持不可变。常量索引直接选择对应 slot；动态索引生成带边界检查的有限 switch，只物化所选元素，不为每次访问复制完整 package array backing。
- 具有 pure static initializer 的 package scalar、runtime record 与定长数组可在 callback 中读取；数组额外支持动态索引和 value range。值会在使用点按 runtime layout 重建且始终不可变，callback 不能修改 package 值整体、字段或数组元素。该能力对应 Py module-level 常量与常量表，但不把 package variable 变成 Sonolus runtime storage。
- 静态 pointer alias、`new(T)` 非逃逸局部、nil pointer 零值/转换/比较、有限 runtime pointer target 重绑与 target-set 合并、pointer helper 参数/返回、runtime-selected 数组 pointer 的动态索引、解引用读写、pointer 比较、嵌套动态数组 place 和逐 slot comparable aggregate 比较。callback-local 普通 struct 可保存 pointer alias 字段；零值、字段重绑、pointer receiver、整 struct 赋值、嵌套 struct、值参数与 helper 返回都保持 Go 的别名和按值快照语义。包含 pointer/container descriptor 的局部定长数组支持零值/literal、整值复制、常量和动态索引、元素/字段重绑、value range、helper 参数与返回。`new(descriptorStruct)` 与 `&localStruct` 具有独立 aggregate 身份；其 pointer 可保存到局部变量或 struct 字段、动态重绑、比较、传参和返回，解引用写入继续作用于被选中的原对象。nil 解引用在所有 runtime-check 等级终止 callback，`notify` 额外发出诊断。
- Go 多返回会逐项保留 callback-local descriptor，而不是按 pointee runtime layout 展平。多个 aggregate pointer 可经动态重绑、命名裸返回、helper 转发或立即调用闭包返回；调用端多重赋值继续保持每个对象的独立身份、nil 状态与写回路径。
- 静态 interface concrete type 传播、有限 concrete-type variant、helper 参数/返回、method devirtualization、type switch/type assertion，以及泛型实例中的同类静态分派。
- Static interface 可保存到 callback-local 普通 struct/定长数组字段；aggregate 复制会快照 concrete-type tag 与 payload descriptor。Pointer concrete payload 保留 aggregate 身份，因此接口字段重绑不会改变既有副本，而接口方法对所选对象的修改继续通过原别名可见。
- callable helper 的有限 runtime 返回目标、variadic callable 参数包的转发/索引/range、保持静态目标身份与泛型实例环境的 named function type 转换（包括 `iter.Seq[T](func...)`）、`min`、`max`、`Zero[T]`、`SlotsOf[T]`。泛型函数值和离开创建 frame 的闭包会携带其具体 type substitution；`Zero[*T]()` 返回准确的 nil pointer，compile-time-only function/interface/container/resource 类型不接受 `Zero`。
- `VarArray` 支持 checked/unchecked 读写与追加、查询、删除、交换、重排、正反向 values/items iterator、稳定排序、原子 `Extend`、稳定 min/max；`ArrayMap` 与 `ArraySet` 支持容量查询，map 还提供 key/value/item iterator。容器变量、参数、helper 返回值与 callback-local 普通 struct 字段可在有限 runtime 分支中选择不同 backing/capacity；descriptor 按值快照，mutation 继续作用于被选中的原 backing。包含 container 字段的局部 struct 支持 pointer receiver、整值复制、嵌套 struct、值参数与 helper 返回。零槽可比较 element/key 也合法，容器只保留 size 与必要的非零槽 backing。
- `SortLinkedEntities` 与 `SortDoublyLinkedEntities` 使用稳定 bottom-up merge sort，仅重排链接。链表输入必须无环。

静态 variadic 参数只允许 `len/cap`、索引、`range` 和向另一个 variadic helper 静态转发。

明确拒绝：

- 可逃逸或存储的通用 runtime interface 对象、反射、goroutine、channel，以及需要动态栈的重复 `defer`。
- 动态递归、重复递归状态、runtime-created closure、函数值逃逸到 aggregate/interface/storage。
- 普通运行时 slice/map/string 操作和未登记 builtin。
- 无法枚举有限 backing/target 集合的动态容器或调用，以及容器 descriptor 向 runtime storage 的逃逸。
- pointer/container/interface descriptor 仍不能进入 package static、Archetype、LevelMemory、LevelData、replay 或其他跨 callback storage；它们只表示单次 callback lowering 中可枚举的有限身份与 backing，不会成为 EngineData 的持久对象引用。

## 标准库

`math` 支持 catalog 登记且可保持 Go 语义的常量及函数，例如 `Abs`、`Floor`、`Sin`、`Sinh`、`Atan2`、`Min`、`Max`、`Pow`、`Mod`。`math/rand.Float64()` 映射到 Sonolus Runtime RNG，`rand.Intn(n)` 映射到 `[0,n)` 整数随机；常量 `n <= 0` 在编译期报错，动态 `n <= 0` 在所有 runtime-check 等级终止 callback，`notify` 额外输出诊断。

Go 运算符与标准库遵循 Go 数值规则：`int(x)` 与整数 `/` 向零截断，`%` 和 `math.Mod` 使用 remainder，`math.Round` 的半数远离零。需要 Sonolus/Python modulo 或 JS `Math.round` 时分别显式调用 `native.Mod`、`native.Round`。非 `int` 固定宽度整数与 unsigned runtime 运算不属于 DSL；超出 float32 精确整数范围的 Go overflow 行为不作承诺。

## Runtime checks

`Assert` 遵循 Compiler runtime-check 等级，`Require` 始终检查，`StaticAssert` 必须在编译期成立。静态为 false 的 `Assert`/`Require` 仍生成 callback termination，而不是编译错误；`StaticAssert` 与 `Unreachable` 才用于要求编译期证明。`Terminate(message)` 可从任意内联 helper 终止当前 callback，`Notify(message)` 仅在 `notify` 等级发出诊断并继续执行。`RuntimeChecksEnabled()` 是当前编译选项的编译期常量，可用于裁剪仅在检查开启时需要的代码；`Unreachable(message)` 只允许位于被常量裁剪的不可达路径，任何实际 lowering 到的调用都会稳定报错。`none` 移除动态检查，`terminate` 失败时退出 callback，`notify` 还会依次发出诊断码的 `DebugLog` 与 `DebugPause`。诊断码在完整 Project 聚合后按 mode、global/archetype、callback、RPO/instruction、源码位置和 inline stack 稳定编号，并只保证在相同源码快照内稳定。诊断表保存在 `compiler.Artifacts.Diagnostics`，不进入 EngineData wire schema。

动态 `int`（含底层类型为 `int` 的 named type）除法和 remainder 会先物化除数，并始终执行非零 `Require` guard；`none` 与 `terminate` 失败时终止 callback，`notify` 额外发出稳定诊断。float 除零继续遵循 IEEE NaN/Inf。
