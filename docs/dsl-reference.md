# Go DSL 参考

## 基本原则

编译器接受的是静态可分析的 Go 子集，而不是任意合法 Go 程序。所有 Sonolus API 通过 `go/types.Object` 身份与 catalog 项匹配，因此 import alias 合法，同名用户函数不能伪造 intrinsic 或资源 constructor。

四种模式使用 build tag `play`、`watch`、`preview`、`tutorial` 独立加载。未带模式 tag 的文件会进入每种模式。

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

模式资源支持范围：

| 模式 | 资源 |
|---|---|
| Play | skin、effect、particle、buckets |
| Watch | skin、effect、particle、buckets |
| Preview | skin |
| Tutorial | skin、effect、particle、instruction、instructionIcon |

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

规则：

- `float64`、`bool`、`int` 字段分别使用 `SliderOption`、`ToggleOption`、`SelectOption` 初始化。
- option metadata 使用对应的强类型 config；空 `Name` 默认使用 Go 字段名。
- `SelectOptionConfig.Default` 是 `Values` 的索引，values 必须是非空静态字符串列表。
- `sonolus.UIConfig` 字段自动作为 UI 配置识别，最多一个。
- `[]string` 字段自动作为 replay fallback 识别，最多一个；其值必须引用 option 的最终外部名称，禁止空值和重复。

## ROM

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

只允许一个 ROM 声明。`ROMFile` 必须精确绑定一个文件，文件长度必须是 4 的倍数。用户 ROM 按 little-endian float32 保存；backend 会在前面添加 NaN、`+Inf`、`-Inf` 三个固定值。

## Archetype

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

Archetype 支持一个直接匿名嵌入的基类，例如匿名字段 `Base` 使用 `archetype:"base"` tag。基类必须在当前模式中声明；字段按 storage 分区成为派生布局前缀，callback method、order 与 runtime 类型关系沿单继承链继承，派生直接方法和显式 order 会覆盖基类。

`sonolus.EntityRef[T].Get()` 返回目标 Archetype 的类型化 runtime view。`GetUnchecked` 跳过引用检查；`EntityRefAs`、`EntityRefMatches` 与 `EntityRefGetAs` 支持 `AnyArchetype` 和继承关系下的静态转换、strict/non-strict 匹配与访问。启用 runtime checks 时，`Get`/`GetAs` 会验证非负 index 与目标 MRO。

视图允许读取 `imported`、`data` 与 `shared` 字段，并在对应 callback 阶段写入允许写入的 `data`/`shared` 字段；其他实体的 `memory` 与 `exported` 字段始终拒绝访问。视图可通过 helper 返回、保存到同一静态目标类型的本地 struct、定长数组或 catalog container，并按 entity index 比较；仍不能进入 callback 返回、Archetype/resource/static storage、接口，或对字段取地址。动态实体 index 与嵌套数组 index 会组合成单一 affine memory address，每层边界检查独立保留，且各 index 只求值一次。

## Replay Streams

Play 与 Watch 可用同构的 `StreamResource` singleton 声明强类型 replay streams。typed stream ID 从 1 开始；`Stream[T]` 按 `T` 的 slot 数占用连续 ID，定长数组继续连续展开。`StreamData[T]` 始终只占一个 ID，Play 使用 `Set(value)` 将各 slot 写入整数 key 并补齐半整数 sentinel，Watch 使用 `Get()` 从同一 backing stream 的 slot key 读取完整值。

`Stream[T]` 在 Play 使用 `Set`，在 Watch 使用 `Has`、`Get`、前后 key/value 查询，以及正序、倒序和 previous-frame iterator。Play/Watch 声明必须逐字段一致。存在 typed stream 声明时，低层 `play.Streams`/`watch.Streams` 的常量 ID 不得落入其保留区；动态 ID 无法证明不冲突，因此会被拒绝。

## Play Level Memory

`play.LevelMemory.Get(index)` 与 `play.LevelMemory.Set(index, value)` 提供 Play 模式的引擎级共享数值槽，用于不同 Archetype callback 之间的低层协调。索引必须是非负整数表达式；调用方负责为各用途分配稳定且互不重叠的区间。

该接口直接映射 Sonolus `LevelMemory`，不会自动分配布局，也不会跨模式共享。需要结构化局部集合时优先使用 `VarArray`、`ArrayMap` 或 `ArraySet`；只有确实需要跨 Archetype 状态时才使用 `LevelMemory`。

`preview.LevelData.Get(index)` 与 `preview.LevelData.Set(index, value)` 提供 Preview 模式的共享数值槽，映射 Sonolus `PreviewData`。它适合在较早执行的 archetype `Preprocess` 中汇总关卡范围，再由较高 callback order 的 Stage 读取并设置 Canvas；索引布局同样由引擎源码自行稳定分配。

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
- `if/else`、三段式与条件 `for`、expression/type `switch`、`fallthrough`、带标签或无标签的 `break`/`continue`、`return`；不支持 `goto`。
- `range` 整数、定长数组、定长数组指针、静态 variadic 参数和 catalog 容器。与 Go 一致，数组值在进入 value iteration 前复制快照；数组指针表达式只求值一次并从原 backing 实时读取。只迭代 index 时允许 nil 数组指针，因为不会发生解引用。`len`/`cap` 同样接受定长数组指针并直接返回静态长度。
- Go 1.25 range-over-func 的 0/1/2 value yield、`break`、`continue` 和外层 `return`。
- 跨用户包 helper、具体方法、泛型实例、立即调用闭包、命名/多返回值和裸 return。泛型实例在 inline frame 内使用具体 type substitution，因此 scalar、`[N]T`、`struct{...T}`、generic named type、`*T`、`Zero[T]` 与 `SlotsOf[T]` 都按实例化后的多槽 layout lowering。
- 返回闭包、bound method value、具体类型与静态 interface 的 method expression（`T.M`、`(*T).M`、`I.M`）、package 级不可变 callable、有限 runtime callable target dispatch，以及由编译期常量状态证明终止的最多 256 层递归。method expression 的显式 receiver 严格求值一次；interface receiver 继续通过 finite concrete-type variant 去虚拟化，不生成 runtime interface 对象。
- 静态 pointer alias、`new(T)` 非逃逸局部、nil pointer 零值/转换/比较、有限 runtime pointer target 重绑与 target-set 合并、pointer helper 参数/返回、runtime-selected 数组 pointer 的动态索引、解引用读写、pointer 比较、嵌套动态数组 place 和逐 slot comparable aggregate 比较。nil 解引用在所有 runtime-check 等级终止 callback，`notify` 额外发出诊断。
- 静态 interface concrete type 传播、有限 concrete-type variant、helper 参数/返回、method devirtualization、type switch/type assertion，以及泛型实例中的同类静态分派。
- callable helper 的有限 runtime 返回目标、variadic callable 参数包的转发/索引/range、保持静态目标身份与泛型实例环境的 named function type 转换（包括 `iter.Seq[T](func...)`）、`min`、`max`、`Zero[T]`、`SlotsOf[T]`。泛型函数值和离开创建 frame 的闭包会携带其具体 type substitution；`Zero[*T]()` 返回准确的 nil pointer，compile-time-only function/interface/container/resource 类型不接受 `Zero`。
- `VarArray` 支持 checked/unchecked 读写与追加、查询、删除、交换、重排、正反向 values/items iterator、稳定排序、原子 `Extend`、稳定 min/max；`ArrayMap` 与 `ArraySet` 支持容量查询，map 还提供 key/value/item iterator。容器变量、参数和 helper 返回值可在有限 runtime 分支中选择不同 backing/capacity；descriptor 按值快照，mutation 继续作用于被选中的原 backing。零槽可比较 element/key 也合法，容器只保留 size 与必要的非零槽 backing。
- `SortLinkedEntities` 与 `SortDoublyLinkedEntities` 使用稳定 bottom-up merge sort，仅重排链接。链表输入必须无环。

静态 variadic 参数只允许 `len/cap`、索引、`range` 和向另一个 variadic helper 静态转发。

明确拒绝：

- 可逃逸或存储的通用 runtime interface 对象、反射、goroutine、channel、`defer`。
- 动态递归、重复递归状态、runtime-created closure、函数值逃逸到 aggregate/interface/storage。
- 普通运行时 slice/map/string 操作和未登记 builtin。
- 无法枚举有限 backing/target 集合的动态容器或调用，以及容器 descriptor 向 runtime storage 的逃逸。

## 标准库

`math` 支持 catalog 登记的常量及函数，例如 `Abs`、`Floor`、`Sin`、`Atan2`、`Min`、`Max`、`Pow`、`Mod`。`math/rand.Float64()` 映射到 Sonolus Runtime RNG，`rand.Intn(n)` 映射到 `[0,n)` 整数随机；常量 `n <= 0` 在编译期报错。

Go 运算符遵循 Go 数值规则：`int(x)` 与整数 `/` 向零截断，`%` 使用 remainder。需要 Sonolus/Python modulo 时显式调用对应 native API。非 `int` 固定宽度整数与 unsigned runtime 运算不属于 DSL；超出 float32 精确整数范围的 Go overflow 行为不作承诺。

## Runtime checks 与 simulator

`Assert` 遵循 Compiler runtime-check 等级，`Require` 始终检查，`StaticAssert` 必须在编译期成立。静态为 false 的 `Assert`/`Require` 仍生成 callback termination，而不是编译错误；`StaticAssert` 与 `Unreachable` 才用于要求编译期证明。`Terminate(message)` 可从任意内联 helper 终止当前 callback，`Notify(message)` 仅在 `notify` 等级发出诊断并继续执行。`RuntimeChecksEnabled()` 是当前编译选项的编译期常量，可用于裁剪仅在检查开启时需要的代码；`Unreachable(message)` 只允许位于被常量裁剪的不可达路径，任何实际 lowering 到的调用都会稳定报错。`none` 移除动态检查，`terminate` 失败时退出 callback，`notify` 还会依次发出诊断码的 `DebugLog` 与 `DebugPause`。诊断码在完整 Project 聚合后按 mode、global/archetype、callback、RPO/instruction、源码位置和 inline stack 稳定编号，并只保证在相同源码快照内稳定。诊断表保存在 `compiler.Artifacts.Diagnostics`，不进入 EngineData wire schema。

动态 `int`（含底层类型为 `int` 的 named type）除法和 remainder 会先物化除数，并始终执行非零 `Require` guard；`none` 与 `terminate` 失败时终止 callback，`notify` 额外发出稳定诊断。float 除零继续遵循 IEEE NaN/Inf。

公开包 `sonolus/sim` 编译并解释最终 EngineData node tree，可注入初始 block memory、规范化 stream state、随机 seed、step limit 与自定义 RuntimeFunction handler，并返回 callback 结果、最终 memory、按 ID/key 排序的 stream state 和有序副作用日志。执行失败返回 `*sim.ExecutionError`；调用方可用 `errors.As` 检查 `invalid-request`、`invalid-node`、`invalid-arity`、`invalid-argument`、`invalid-state`、`missing-handler` 或 `step-limit`。Simulator 对 control、pure、memory、stream、random、effect 与 handler-required RuntimeFunction 做生成式穷举分类；未知函数不会静默当作自定义函数执行。
