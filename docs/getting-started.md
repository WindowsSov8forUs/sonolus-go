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
	Speed float64 `configuration:"slider,name=Speed,def=1,min=0.5,max=2,step=0.1"`
}

var Configuration GameConfiguration
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

//sonolus:resource skin standard
type SkinData struct {
	Note sonolus.Sprite
}

//sonolus:resource skin standard
var Skin = &SkinData{
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
sonolus-go build -name example -m all .
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

```bash
sonolus-go serve -name example .
```

开发服务器直接在内存中保存 compiler artifacts，监听 Go 源文件和 embed 文件并自动重编译。重编译失败时继续提供上一次成功快照。

## 下一步

- 在 [DSL 参考](dsl-reference.md) 中查看资源、字段 storage 和 callback 表。
- 在 [命令行](cli.md) 中查看 package pattern、ROM fallback 和优化等级。
- 在 [优化器](optimization.md) 中选择 `-O` 等级。
