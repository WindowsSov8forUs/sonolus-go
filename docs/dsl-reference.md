# Go DSL 参考

## 基本原则

编译器接受的是静态可分析的 Go 子集，而不是任意合法 Go 程序。所有 Sonolus API 通过 `go/types.Object` 身份与 catalog 项匹配，因此 import alias 合法，同名用户函数不能伪造 intrinsic 或资源 constructor。

四种模式使用 build tag `play`、`watch`、`preview`、`tutorial` 独立加载。未带模式 tag 的文件会进入每种模式。

## 名称型资源

资源由一对带相同 directive 的类型和唯一 singleton 变量声明：

```go
//sonolus:resource skin standard
type SkinData struct {
	Note    sonolus.Sprite
	Digits  [10]sonolus.Sprite
}

//sonolus:resource skin standard
var Skin = &SkinData{
	Note: sonolus.SkinSprite("#NOTE_HEAD_CYAN"),
	Digits: [10]sonolus.Sprite{
		sonolus.SkinSprite("digit-0"),
		// ...
	},
}
```

支持的名称型资源和 constructor：

| Directive | 字段类型 | Constructor |
|---|---|---|
| `skin [default|standard|lightweight]` | `sonolus.Sprite` | `sonolus.SkinSprite(name)` |
| `effect` | `sonolus.Clip` | `sonolus.EffectClip(name)` |
| `particle` | `sonolus.Effect` | `sonolus.ParticleEffect(name)` |
| `instruction` | `sonolus.Text` | `sonolus.InstructionText(name)` |
| `instructionIcon` | `sonolus.Icon` | `sonolus.InstructionIcon(name)` |

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
//sonolus:resource buckets
type BucketData struct {
	Tap sonolus.Bucket
}

//sonolus:resource buckets
var Buckets = &BucketData{
	Tap: sonolus.JudgmentBucket(
		"#MILLISECONDS",
		sonolus.JudgmentBucketSprite(Skin.Note, 0, 0, 1, 1, 0),
	),
}
```

Bucket constructor 必须是 catalog 中的准确对象，参数必须能在声明阶段静态解释。

## Configuration

Configuration 使用专用 `configuration` tag：

```go
type GameConfiguration struct {
	sonolus.Configuration

	Speed  float64          `configuration:"slider,name=Speed,def=1,min=0.5,max=2,step=0.1,unit=#TIMES"`
	Auto   bool             `configuration:"toggle,name=Auto,def=false"`
	Lane   int              `configuration:"select,name=Lane,def=0,values=4|6|8"`
	UI     sonolus.UIConfig `configuration:"ui"`
	Replay []string         `configuration:"replayFallback"`
}

var Config = GameConfiguration{
	UI: sonolus.UIConfig{
		Scope:          "game",
		PrimaryMetric:  sonolus.UIMetricArcade,
		SecondaryMetric: sonolus.UIMetricLife,
	},
	Replay: []string{"Speed", "Lane"},
}
```

规则：

- `slider` 字段必须为 `float64`，使用 `name/def/min/max/step`，`unit` 可选。
- `toggle` 字段必须为 `bool`，使用 `name/def`。
- `select` 字段必须为 `int`，使用 `name/def/values`；values 用 `|` 分隔。
- `ui` 字段必须为 `sonolus.UIConfig`。
- `replayFallback` 字段必须为 `[]string`，值必须引用 option 的最终外部名称，禁止空值和重复。
- 旧 `sonolus` Configuration tag 不受支持。

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
	play.Archetype      `sonolus:"name=TapNote,hasInput=true"`
	play.CallbackOrders `sonolus:"preprocess=-10,updateSequential=5"`

	Beat     float64          `sonolus:"imported,name=#BEAT,default=0"`
	Data     float64          `sonolus:"data"`
	Position sonolus.Vec2     `sonolus:"memory"`
	Shared   float64          `sonolus:"shared"`
	HitTime  float64          `sonolus:"exported,name=hitTime"`
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
- 算术、比较、逻辑短路、显式数值转换。
- `if/else`、三段式与条件 `for`、`switch`、`break`、`continue`、`return`。
- `range` 整数、定长数组、静态 variadic 参数和 catalog 容器。
- 跨用户包 helper、具体方法、泛型实例、立即调用闭包、命名/多返回值和裸 return。
- 唯一初始化且不逃逸的局部函数变量；调用会被静态内联。

静态 variadic 参数只允许 `len/cap`、索引、`range` 和向另一个 variadic helper 静态转发。

明确拒绝：

- 递归，包括通过函数变量形成的间接递归。
- 接口派发、反射、类型断言、goroutine、channel、`defer`。
- package 级函数变量、函数返回值、函数值重赋、逃逸或运行时目标选择。
- 普通运行时 slice/map/string 操作和未登记 builtin。
- 动态容器 backing/capacity 和无法静态确定目标的调用。

## 标准库

`math` 支持 catalog 登记的常量及函数，例如 `Abs`、`Floor`、`Sin`、`Atan2`、`Min`、`Max`、`Pow`、`Mod`。`math/rand.Float64()` 映射到 Sonolus Runtime RNG，`rand.Intn(n)` 映射到 `[0,n)` 整数随机；常量 `n <= 0` 在编译期报错。

这些映射遵循 Sonolus Runtime 数值语义，不承诺与普通 Go 对 NaN、Inf、溢出、seed 或并发行为完全一致。
