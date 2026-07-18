# 快速开始

## 前置条件

- Go 1.25 或更高版本。
- `sonolus-go` 可执行文件，或直接使用 `go run ./cmd/sonolus-go`。
- 引擎源码必须属于一个 Go module，入口 package 必须为 `main`。

## 创建 module

```bash
mkdir example-engine
cd example-engine
go mod init example-engine
go get github.com/WindowsSov8forUs/sonolus-go/v2
```

推荐用 build tags 隔离四种模式。同名资源和 archetype 可以分别出现在 `play.go`、`watch.go`、`preview.go`、`tutorial.go` 中。

## 共享声明

创建 `main.go`：

```go
package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type GameConfiguration struct {
	sonolus.Configuration
	Speed float64
}

var Configuration = GameConfiguration{Speed: sonolus.SliderOption(sonolus.SliderOptionConfig{
	Name: "Speed", Default: 1, Min: 0.5, Max: 2, Step: 0.1,
})}

func main() {}
```

Configuration 和 ROM 是引擎级共享产物。四种模式分别加载后，compiler 会比较它们的规范化结果；不同模式下的声明必须语义一致。ROM 声明不是必需的；不需要 ROM 时不要声明，compiler 会省略 `EngineRom`。缺失声明与显式空声明在跨模式比较中等价，但显式空声明仍会要求生成 ROM。

## Play 模式

创建 `play.go`：

```go
//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type SkinData struct {
	sonolus.SkinResource

	Note sonolus.Sprite
}

var Skin = &SkinData{
	SkinResource: sonolus.SkinResource{RenderMode: sonolus.RenderModeStandard},
	Note: sonolus.SkinSprite("#NOTE_HEAD_CYAN"),
}

type TapNote struct {
	play.Archetype      `archetype:"name=TapNote,hasInput=true"`
	play.CallbackOrders `archetype:"preprocess=-10"`

	Beat float64 `archetype:"imported,name=#BEAT,default=0"`
	X    float64 `archetype:"memory"`
	Hit  float64 `archetype:"exported,name=hitTime"`
}

func (n *TapNote) Preprocess() {
	n.X = n.Beat
}

func (*TapNote) ShouldSpawn() bool { return true }
```

## 其他模式

每种模式使用对应 marker：

```go
// Play
type Note struct { play.Archetype `archetype:"name=Note"` }

// Watch
type Note struct { watch.Archetype `archetype:"name=Note"` }

// Preview
type Note struct { preview.Archetype `archetype:"name=Note"` }
```

Tutorial 没有 archetype，使用全局 callback marker：

```go
//go:build tutorial

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/tutorial"

type Globals struct{ tutorial.GlobalCallbacks }
var Global Globals

func Preprocess() {}
func Navigate()   {}
func Update()     {}
```

跨 Archetype 共享状态可使用结构化 level global：

```go
type RuntimeState struct {
    sonolus.LevelMemoryResource
    Active sonolus.VarArray[sonolus.EntityRef[Note]]
}

var State = RuntimeState{
    Active: sonolus.NewVarArray[sonolus.EntityRef[Note]](16),
}
```

需要仅在 `Preprocess` 初始化、随后只读的数据时嵌入 `sonolus.LevelDataResource`。多个声明会自动取得互不重叠的稳定布局；完整模式映射与写权限见 [DSL 参考](dsl-reference.md#level-memory-与-level-data)。

Watch 的可选全局 callback 使用 `watch.GlobalCallbacks`，并可定义 `func UpdateSpawn() float64`。Tutorial 的三个全局 callback 和 Watch 的 `UpdateSpawn` 都是可选的；存在时必须严格符合签名。

## 编译

在 `sonolus-go` 仓库外使用已安装的可执行文件：

```bash
sonolus-go build -m all .
```

输出目录默认为 `dist/example`，包含：

```text
EngineConfiguration
EngineRom
EnginePlayData
EngineWatchData
EnginePreviewData
EngineTutorialData
```

这些文件是 gzip 数据。仅编译单模式时，只写该模式的 EngineData。

## 开发服务器

可选地在共享文件中声明一个开发关卡：

```go
import _ "embed"

//sonolus:level
//go:embed dev-level.json
var DevelopmentLevel sonolus.LevelFile
```

`dev-level.json` 使用 Sonolus LevelData schema：

```json
{
  "bgmOffset": 0,
  "entities": [
    {
      "name": "note-0",
      "archetype": "Note",
      "data": [{ "name": "#BEAT", "value": 1 }]
    }
  ]
}
```

开发关卡也可以使用宿主侧 `sonolus/level` 包从类型化 Go 值生成，再将生成的 JSON 保持为 checked-in embed 文件：

```go
type NoteData struct {
    Beat float64 `level:"#BEAT"`
    Lane float64 `level:"lane"`
    Next level.Ref[NoteData] `level:"next,omitempty"`
}

var Note = level.MustDefine[NoteData]("Note")

func buildLevel() (*resource.LevelData, error) {
    first := Note.New(NoteData{Beat: 1, Lane: -1})
    second := Note.New(NoteData{Beat: 2, Lane: 1})
    first.Data.Next = second.Ref()
    return level.NewBuilder().Add(first, second).Build()
}
```

`sonolus/level` 在普通 Go 进程中运行，不属于 callback DSL，也不进入 Compiler IR。它会稳定自动命名实体、验证同关卡引用，并按与 Archetype imports 相同的数组/record 规则展开字段。Godori 使用 `go generate ./godori` 更新 `godori/dev-level.json`，测试会逐字节确认生成结果没有过期。

同一文件必须对 Play、Watch 和 Preview 可见。实体 archetype 和每个 data name 必须存在于三个模式对应的声明中；实体引用必须指向同一关卡内的命名实体。未声明时使用空开发关卡。

```bash
sonolus-go dev .
```

开发服务器提供完整 Sonolus 路由和内置开发资源，可由客户端直接打开 `Dev Level`。它监听 Go 源文件和 embed 文件并自动重编译；重编译失败时继续提供上一次成功快照。开发关卡只供 `dev` 使用，不会写入 `build` 产物。

## 下一步

- 仓库中的 [`godori/`](../godori/) 是可直接编译和游玩的四模式引擎，并由端到端测试持续验证。可从 [`play.go`](../godori/play.go)、[`watch.go`](../godori/watch.go)、[`preview.go`](../godori/preview.go) 和 [`tutorial.go`](../godori/tutorial.go) 查看各模式实现。
- `godori` 参考 `sonolus.py@1040bc0dcc116efdbca05f144edec302e839bcd3` 中 pydori 的整体设计，以当前 Go DSL 独立重写。当前覆盖 Tap、Flick、Directional Flick、由 abstract note 与 `prev`/`next` 统一链描述的任意 anchor Hold、同时押宽 hitbox 切分、九槽 projective stage transform、统一 layer z、音符淡入淡出、周期 Flick 箭头与 affine particle quad、SimLine、BPM/Timescale、99999 实体容量的 Play/Watch stream、Watch replay Hold 分段音效、组合 judgment bucket、动态效果 archetype、结构化 level globals、两秒分栏 Preview 与 Tutorial；它仍是编译器验收工程，不是 pydori 的逐源码移植。
- 泛型 helper、闭包、variadic helper、整数 range 和静态 callable 等 DSL 边界由内部 conformance fixture 覆盖，不作为公开引擎示例。
- 在 [DSL 参考](dsl-reference.md) 中查看资源、字段 storage 和 callback 表。
- 在 [命令行](cli.md) 中查看 package pattern、ROM fallback 和优化等级。
- 在 [优化器](optimization.md) 中选择 `-O` 等级。
