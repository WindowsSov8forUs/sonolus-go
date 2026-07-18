# 快速开始

## 前置条件

- Go 1.25 或更高版本。
- `sonolus-go` 可执行文件，或直接使用 `go run ./cmd/sonolus-go`。
- 引擎源码必须属于一个 Go module，入口 package 必须为 `main`。

## 创建引擎工程

先使用 `mod init` 创建包含 module metadata 的引擎工程根，再在其中创建一个或多个引擎 package：

```bash
sonolus-go mod init example.com/sirius sirius
cd sirius
sonolus-go init ./engines/first
sonolus-go init ./engines/second
go mod tidy
sonolus-go vet ./engines/first ./engines/second
```

`mod init` 在 `sirius/` 生成共同的 `go.mod`、`go.sum`、`.gitignore` 和推荐的 gopls 配置；`init` 只在各引擎目录生成共享入口和四个模式文件。两条命令都不联网，也不会覆盖已有源码；创建引擎后由 `go mod tidy` 下载与当前 CLI 发布版本匹配的 `sonolus-go` module。开发版 CLI 无法自动确定依赖版本时，对 `mod init` 使用 `-sonolus-version v2.x.y` 显式指定。

`go.mod` 与 `go.sum` 属于 module 根。多引擎工程中它们位于所有引擎 package 的共同祖先；单引擎工程也可以让 module 根同时作为唯一的引擎 package：

```bash
sonolus-go mod init example.com/single single
cd single
sonolus-go init .
go mod tidy
sonolus-go vet .
```

`init` 会从目标目录向上查找最近的 module metadata，两者必须同时存在且为普通文件。module 根只有在除 `.git`、`.gitignore`、`.vscode`、`go.mod` 和 `go.sum` 外为空时才能就地初始化；一旦已有共享目录或其他源码，就必须在子目录创建引擎。缺失文件、损坏的 `go.mod` 或不符合该结构都会报错。

工程结构示例：

```text
sirius/
├─ go.mod
├─ go.sum
├─ internal/shared/
└─ engines/
   ├─ first/
   └─ second/
```

也可以手动建立相同结构，但必须先准备完整的 module metadata：

```bash
mkdir sirius
cd sirius
go mod init example.com/sirius
go get github.com/WindowsSov8forUs/sonolus-go/v2
sonolus-go init ./engines/first
go mod tidy
```

推荐用 build tags 隔离四种模式。同名资源和 archetype 可以分别出现在 `play.go`、`watch.go`、`preview.go`、`tutorial.go` 中。

## 编辑器配置

gopls 一次只能按一种模式的 build tag 分析引擎 package。使用 VS Code 时，建议在引擎项目的本地 `.vscode/settings.json` 中选择当前主要编辑的模式：

```json
{
  "gopls": {
    "buildFlags": ["-tags=play"],
    "standaloneTags": ["ignore"],
    "staticcheck": true,
    "analyses": {
      "SA4017": false
    },
    "gofumpt": false
  },
  "[go]": {
    "editor.defaultFormatter": "golang.go",
    "editor.formatOnSave": true,
    "editor.codeActionsOnSave": {
      "source.organizeImports": "explicit"
    }
  }
}
```

编辑其他模式时，将 `play` 替换为 `watch`、`preview` 或 `tutorial`，然后执行 `Go: Restart Language Server`。不要同时启用四个 tag；互斥模式文件中允许存在同名声明，同时加载会产生重复定义。

`standaloneTags` 不用于选择 Sonolus 模式。它会把带指定 tag 的单个 `package main` 文件视为完整的独立程序，无法与无 tag 的共享文件组成引擎 package；应保持默认的 `ignore`。也不要通过全局 `GOFLAGS` 设置模式 tag，否则终端中的 `go test`、`go generate` 等命令也会受到影响。

`SA4017` 会把“忽略无副作用函数的返回值”报告为问题，但 `sonolus` 公开包中的函数体只是供 Go 类型检查使用的声明桩。例如 `particle.Spawn(...)` 在普通 Go 分析中看似只返回零值，compiler 实际会将它 lower 为有写副作用的 `SpawnParticleEffect`；非循环粒子不需要后续移动或销毁时可以直接忽略返回的 handle。因此推荐保留其他 Staticcheck 分析，只单独关闭 `SA4017`。如果不想修改分析配置，也可以用 `_ = particle.Spawn(...)` 显式丢弃 handle。

需要频繁同时编辑多种模式时，可以为每种模式准备一个本地 `.code-workspace` 文件，并在独立窗口中为各自的 `gopls.buildFlags` 固定对应 tag。

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
EnginePlayData
EngineWatchData
EnginePreviewData
EngineTutorialData
```

这些文件是 gzip 数据。仅编译单模式时，只写该模式的 EngineData；只有源码声明 ROM、提供 fallback 或 callback 实际读取 ROM 时才额外写出 `EngineRom`。

## 开发服务器

可选地在共享文件中声明一个开发关卡：

```go
import _ "embed"

//sonolus:level
//go:embed dev-level.json
var DevelopmentLevel sonolus.LevelFile
```

单关卡声明可以省略名称，此时开发服务器保持兼容并提供 `dev` / `Dev Level`。需要多个开发关卡时，每个声明必须通过 directive 参数提供唯一的 Sonolus item 名称；显示标题由 Go 变量名稳定生成：

```go
//sonolus:level basic
//go:embed basic.json
var BasicLevel sonolus.LevelFile

//sonolus:level stress
//go:embed stress.json
var StressTest sonolus.LevelFile
```

上例分别生成 `basic` / `Basic Level` 与 `stress` / `Stress Test`。声明按 Go 变量名稳定排序；每个 LevelData 独立解码和校验，任一关卡失败时开发服务器继续提供上一份完整成功快照。

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

所有声明必须对 Play、Watch 和 Preview 可见，并在三个模式中保持相同的变量、名称参数和 embed 文件。每个实体 archetype 和 data name 必须存在于三个模式对应的声明中；实体引用必须指向同一关卡内的命名实体。未声明时使用空开发关卡。

```bash
sonolus-go dev .
```

开发服务器提供完整 Sonolus 路由、开发关卡列表和内置开发资源。它监听 Go 源文件和所有 LevelData embed 文件并自动重编译；重编译失败时继续提供上一次完整成功快照。开发关卡只供 `dev` 使用，不会写入 `build` 产物。

## 下一步

- 仓库中的 [`godori/`](../godori/) 是可直接编译和游玩的四模式引擎，并由端到端测试持续验证。可从 [`play.go`](../godori/play.go)、[`watch.go`](../godori/watch.go)、[`preview.go`](../godori/preview.go) 和 [`tutorial.go`](../godori/tutorial.go) 查看各模式实现。
- `godori` 参考 `sonolus.py@1040bc0dcc116efdbca05f144edec302e839bcd3` 中 pydori 的整体设计，以当前 Go DSL 独立重写。当前覆盖 Tap、Flick、Directional Flick、由 abstract note 与 `prev`/`next` 统一链描述的任意 anchor Hold、同时押宽 hitbox 切分、九槽 projective stage transform、统一 layer z、音符淡入淡出、周期 Flick 箭头与 affine particle quad、SimLine、BPM/Timescale、99999 实体容量的 Play/Watch stream、Watch replay Hold 分段音效、组合 judgment bucket、动态效果 archetype、结构化 level globals、两秒分栏 Preview 与 Tutorial；它仍是编译器验收工程，不是 pydori 的逐源码移植。
- 泛型 helper、闭包、variadic helper、整数 range 和静态 callable 等 DSL 边界由内部 conformance fixture 覆盖，不作为公开引擎示例。
- 在 [DSL 参考](dsl-reference.md) 中查看资源、字段 storage 和 callback 表。
- 在 [命令行](cli.md) 中查看 package pattern、ROM fallback 和优化等级。
- 在 [优化器](optimization.md) 中选择 `-O` 等级。
