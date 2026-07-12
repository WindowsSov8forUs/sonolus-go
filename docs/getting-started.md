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
go get github.com/WindowsSov8forUs/sonolus-go
```

推荐用 build tags 隔离四种模式。同名资源和 archetype 可以分别出现在 `play.go`、`watch.go`、`preview.go`、`tutorial.go` 中。

## 共享声明

创建 `main.go`：

```go
package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type GameConfiguration struct {
	sonolus.Configuration
	Speed float64
}

var Configuration = GameConfiguration{Speed: sonolus.SliderOption(sonolus.SliderOptionConfig{
	Name: "Speed", Default: 1, Min: 0.5, Max: 2, Step: 0.1,
})}
var ROM = sonolus.ROMValues{}

func main() {}
```

Configuration 和 ROM 是引擎级共享产物。四种模式分别加载后，compiler 会比较它们的规范化结果；不同模式下的声明必须语义一致。缺失声明等价于空声明。

## Play 模式

创建 `play.go`：

```go
//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
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
	play.Archetype      `sonolus:"name=TapNote,hasInput=true"`
	play.CallbackOrders `sonolus:"preprocess=-10"`

	Beat float64 `sonolus:"imported,name=#BEAT,default=0"`
	X    float64 `sonolus:"memory"`
	Hit  float64 `sonolus:"exported,name=hitTime"`
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
type Note struct { play.Archetype `sonolus:"name=Note"` }

// Watch
type Note struct { watch.Archetype `sonolus:"name=Note"` }

// Preview
type Note struct { preview.Archetype `sonolus:"name=Note"` }
```

Tutorial 没有 archetype，使用全局 callback marker：

```go
//go:build tutorial

package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus/tutorial"

type Globals struct{ tutorial.GlobalCallbacks }
var Global Globals

func Preprocess() {}
func Navigate()   {}
func Update()     {}
```

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

同一文件必须对 Play、Watch 和 Preview 可见。实体 archetype 和每个 data name 必须存在于三个模式对应的声明中；实体引用必须指向同一关卡内的命名实体。未声明时使用空开发关卡。

```bash
sonolus-go dev .
```

开发服务器提供完整 Sonolus 路由和内置开发资源，可由客户端直接打开 `Dev Level`。它监听 Go 源文件和 embed 文件并自动重编译；重编译失败时继续提供上一次成功快照。开发关卡只供 `dev` 使用，不会写入 `build` 产物。

## 下一步

- 仓库中的 [`examples/minimal`](../examples/minimal) 是可直接编译的四模式最小示例，并由端到端测试持续验证。
- [`examples/conformance`](../examples/conformance) 集中覆盖泛型 helper、闭包、控制流和更多静态资源，可用于核对 DSL 边界。
- 在 [DSL 参考](dsl-reference.md) 中查看资源、字段 storage 和 callback 表。
- 在 [命令行](cli.md) 中查看 package pattern、ROM fallback 和优化等级。
- 在 [优化器](optimization.md) 中选择 `-O` 等级。
