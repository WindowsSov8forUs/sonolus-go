```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

# DSL 语言参考
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus-go 将 Go 语法的一个受限子集编译为 Sonolus EngineData。本参考面向引擎作者——如果你刚开始，先看[快速入门](getting-started.md)。
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

## 快速上手
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

package myengine
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

type Skin struct { Note float64 }
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

type Note struct {
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    Beat float64 `sonolus:"imported"`
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    X    float64 `sonolus:"memory"`
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

}
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

func (n *Note) Initialize() {
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    n.X = sonolus.Sin(n.Beat)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    sonolus.SkinSprite("Note").Draw(sonolus.NewQuad(n.X, 0, n.X+1, 0, n.X+1, 1, n.X, 1))
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

	sonolus.EffectClip("Hit").Play(0.1)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

}
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

编译：
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```bash
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus-go build -m play engine.go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

## 包声明与 Import
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

package myengine    // 包名用作引擎名
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

### 导入子包（多文件引擎）
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

import "notes"      // → 自动发现 ./notes/*.go 中的原型
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

import "stage"      // → 自动发现 ./stage/*.go 中的原型
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

子目录中只能定义原型 struct 和回调——资源类型（Skin / Effect / Buckets / Instruction）必须在主包。
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

### 导入 sonolus 包（推荐）
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

之后所有运行时调用使用 `sonolus.` 前缀：
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.Draw(...)        // 函数调用
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

x := sonolus.Time        // 全局变量
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

pos := sonolus.NewVec2(x, y) // 构造器
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.DebugPause()     // 调试
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

**函数名映射**：`lowerFirst(Sel.Name)` — `Draw` → `draw`，`DebugPause` → `debugPause`，`GetShifted` → `getShifted`。
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

使用 `sonolus.` 前缀的引擎源码是合法的 Go 程序——`go vet`、`gopls` 自动补全均可用。裸名（`draw(...)`）仍可工作但不通过静态检查。
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

详见[快速入门](getting-started.md)和[架构文档](architecture.md)。
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

## 结构体定义
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

type Name struct {
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    Field Type `sonolus:"tag"`
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

}
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

字段标签决定引擎内存布局：
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 标签 | 内存块 | 可写 | 说明 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

|------|--------|------|------|
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `imported` | EntityMemory (0) | 否 | 从父原型传入 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `memory` | EntityMemory (1+) | 是 | 私有每实体存储 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `data` | EntityData | 否 | 只读原型数据 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `shared` | EntityShared | 是 | 共享可变状态 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `input` | EntityInput | 是 | 输入状态 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `despawn` | EntityDespawn | 是 | 消失效果存储 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `info` | EntityInfo | 否 | 只读实体元数据 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `exported` | Exported | 是 | 导出值（仅 Play） |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `scored` | Exported | 是 | 分数计数器（仅 Play） |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `lifed` | Exported | 是 | 生命值（仅 Play） |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

### 特殊结构体名
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

特定名字的 struct 定义引擎资源（大小写敏感）：
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 类型名 | 所在模式 | 说明 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

|--------|---------|------|
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `Skin` | 全部 | 精灵/纹理定义 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `Effect` | Play, Watch, Tutorial | 音效片段定义 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `Particle` | Play, Watch, Tutorial | 粒子效果定义 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `Buckets` | Play, Watch | 桶/生成规则定义 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `Instruction` | Tutorial | 教程文本/图标定义 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `UI` | 全部 | EngineConfiguration UI 覆写。支持 `RuntimeUiConfig` 记录类型嵌套展开 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

#### UI 配置示例
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// RuntimeUiConfig 自动展开（推荐）
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

type UI struct {
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    Menu      RuntimeUiConfig `sonolus:"ui"`
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    Judgment  RuntimeUiConfig `sonolus:"ui"`
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    Combo     RuntimeUiConfig `sonolus:"ui"`
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    Primary   RuntimeUiConfig `sonolus:"ui"`
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    Secondary RuntimeUiConfig `sonolus:"ui"`
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

}
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

var ui = UI{Menu: RuntimeUiConfig{Scale: 1.0, Alpha: 1.0}}
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

> 兼容旧式平坦标签：`sonolus:"ui=menu.scale"`。新项目建议使用 `RuntimeUiConfig`。
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

对标 Python `menu = RuntimeUiConfig(scale=1.0, alpha=1.0)`。
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

### 记录类型字段
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

字段类型可以是已知记录类型——编译器自动展开为多个 float64 槽位：
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

type Note struct {
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    pos Vec2  `sonolus:"memory"`   // 展开为 pos.x, pos.y (2 槽)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    m   Mat   `sonolus:"memory"`   // 展开为 6 槽
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    q   Quad  `sonolus:"memory"`   // 展开为 8 槽
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

}
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// 访问: n.pos.X = value; x := n.pos.Y
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

支持的记录类型：`Vec2`(2)、`Quad`(8)、`Mat`(6)、`Rect`(4)、`Trans`(9)、`Transform2d`(16)、`Pair`(2)、`EntityInfo`(3)、`EntityRef`(1)、`EntityLife`(4)、`EntityScore`(4)、`PlayEntityInput`(5)、`JudgmentWindow`(6)、`ConsecutiveLife`(2)、`ConsecutiveScore`(3)、`RuntimeUiConfig`(2)。支持 `sonolus.Vec2` 等限定名。
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

记录类型字段支持**复合写回**（`n.Q = n.Q.Rotate(a)`）和**链式方法调用**（`n.Q.Rotate(a).Translate(v)`）。
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

## 回调方法
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

方法名决定编译为哪个 Sonolus 回调：
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

**Play 模式（8 个回调）：**
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

`Preprocess`, `SpawnOrder`, `ShouldSpawn`, `Initialize`, `UpdateSequential`, `Touch`, `UpdateParallel`, `Terminate`
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

**Watch 模式（7 个回调）：**
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

`Preprocess`, `SpawnTime`, `DespawnTime`, `Initialize`, `UpdateSequential`, `UpdateParallel`, `Terminate`
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

**Preview 模式（2 个回调）：**
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

`Preprocess`, `Render`
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

**Tutorial 模式（3 个全局函数）：**
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

`Preprocess()`, `Navigate() float64`, `Update()`
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

**Watch 全局**：`UpdateSpawn() float64`
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

## 控制流
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// if / else
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

if condition { ... } else { ... }
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// switch
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

switch value {
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

case 0: ...
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

case 1, 2: ...
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

default: ...
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

}
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// 无标签 switch
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

switch {
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

case x > 0: ...
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

default: ...
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

}
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// for 循环（仅条件）
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

for i < 10 { ... }
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// for range
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

for i := range 5 { ... }  // i = 0, 1, 2, 3, 4
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// 短路运算符
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

if a && b { ... }  // a 为 false 时不求值 b
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

if a || b { ... }  // a 为 true 时不求值 b
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// 复合赋值
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

x += 1; x *= 2; x++
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

## 变量与赋值
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

x := 1.0        // 声明并赋值
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

x = 2.0         // 覆写
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

x += 1.0        // 复合赋值
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

x++             // 递增
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

所有数值类型运行时均为 `float64`。
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

## 类型系统
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 类型 | 运行时表示 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

|------|-----------|
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `float64`, `int`, `bool` | float64 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `Vec2` | 2 个 float64 (x, y) |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `Quad` | 8 个 float64 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `Mat` / `Rect` / `Trans` / `Pair` / `Transform2d` | 2-16 个 float64 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 用户定义 struct | 带标签的 float64 字段 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

不支持：`string`、`map`、`chan`、`interface`、slice、函数类型。
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

## 运行时函数
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

完整运行时函数列表见 `internal/compiler/frontend/builtins.go`。以下是分类概览：
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

> **比较与逻辑运算符**使用原生 Go 语法：`==`、`!=`、`<`、`<=`、`>`、`>=`、`&&`、`||`、`!`。引擎对应的 `Equal`、`Less`、`And` 等 RuntimeFunction 由编译器通过 `applyBinary`/`applyUnary` 自动生成，不作为可调用 stub 暴露。
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 分类 | 函数示例 | 个数 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

|------|---------|------|
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 算术 | `Abs`, `Sign`, `Floor`, `Ceil`, `Round`, `Frac` | ~10 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 数学 | `Sin`, `Cos`, `Tan`, `Log`, `Power`, `Clamp`, `Lerp` | ~30 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 缓动 | `EaseInSine` ... `EaseOutInElastic` | 36 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 内存 | `Get`, `Set`, `GetShifted`, `SetShifted`, `SetAdd*`, `SetMul*` 等 | ~40 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 绘制 | `Draw`, `DrawCurvedB/T/L/R/BT/LR`, `Paint` | 8 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 音频 | `Play`, `PlayScheduled`, `PlayLooped`, `PlayLoopedScheduled`, `StopLooped`, `StopLoopedScheduled` | 6 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 粒子 | `SpawnParticle`, `MoveParticle`, `DestroyParticle` | 3 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 实体 | `Spawn` | 1 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 判定 | `Judge`, `JudgeSimple`, `ExportValue` | 3 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 生命 | `AddLife` | 1 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 调试 | `DebugLog`, `DebugPause`, `DebugError`, `DebugAssertTrue` 等 | 7 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 随机 | `Random`, `RandomInteger` | 2 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| Stream | `StreamSet`, `StreamHas`, `StreamGetNextKey` 等 | 5 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 时间 | `BeatToTime`, `TimeToScaledTime` 等 | 8 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 栈 | `StackInit`, `StackPush`, `StackPop` 等 | 14 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 触摸 | `TouchID`, `TouchStarted`, `TouchEnded`, `TouchX`, `TouchY` | 5 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 资源查询 | `HasSkinSprite`, `HasEffectClip`, `HasParticle` | 3 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 资源引用 | `SkinSprite`, `Skin`, `EffectClip`, `ParticleClip` | 4 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 关卡设置 | `Score`, `Life` | 2 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 矩阵变换 | `SkinTransform`, `SetSkinTransform`, `ParticleTransform`, `SetParticleTransform`, `Background`, `SetBackground` | 6 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 辅助数学 | `Screen`, `SafeArea`, `OffsetAdjustedTime`, `PrevTime`, `Pnpoly`, `PerspectiveApproach` | 6 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 实体信息 | `EntityInfoIndex`, `EntityInfoArchetype`, `EntityInfoState`, `EntityInfoAt`, `SelfInfo` | 5 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

### 引擎全局变量
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

time, deltaTime, scaledTime, touchCount, isSkip, isReplay,
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

isDebug, aspectRatio, audioOffset, inputOffset, isMultiplayer,
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

scrollDirection, canvasSize, navigationDirection,
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

safeAreaXMin, safeAreaXMax, safeAreaYMin, safeAreaYMax,
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

perfectMultiplier, greatMultiplier, goodMultiplier,
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

lifeInitial, lifeMaximum,
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

entityPerfect, entityGreat, entityGood, entityMiss,
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

entityLifePerfect, entityLifeGreat, entityLifeGood, entityLifeMiss
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

### 记录类型方法
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 类型 | 方法 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

|------|------|
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `Vec2` | `Add`, `Sub`, `Mul`, `Div`, `Magnitude`, `Dot`, `Normalize`, `NormalizeOrZero`, `Angle`, `Rotate`, `RotateAbout`, `Orthogonal`, `AngleDiff`, `SignedAngleDiff` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `Quad` | `Center`, `Translate`, `Scale`, `Permute`, `Rotate`, `Top`, `Right`, `Bottom`, `Left`, `Contains` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `Mat` | `Scale`, `Translate`, `Compose`, `Rotate` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `Rect` | `W`, `H`, `Center`, `Translate`, `Scale` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `Trans` | `Compose`, `Translate`, `Scale`, `Rotate`, `TransformVec` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `Transform2d` | `Translate`, `Scale`, `ScaleAbout`, `Rotate`, `RotateAbout`, `Compose`, `ComposeBefore`, `TransformVec`, `TransformQuad`, `PerspectiveX`, `PerspectiveY`, `SimplePerspectiveX`, `SimplePerspectiveY` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `Pair` | `Lt`, `Le`, `Gt`, `Ge`, `Tuple` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `VarArray` | `Len`, `Capacity`, `IsFull`, `Append`, `Pop`, `Insert`, `Sort`, `Clear`, `Contains`, `Index`, `Remove`, `SetAdd`, `SetRemove` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `ArrayMap` | `Len`, `Capacity`, `Clear`, `Keys`, `Values`, `Items`, `Get`, `Set`, `Delete`, `Contains`, `Pop` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `ArraySet` | `Len`, `Capacity`, `Clear`, `Add`, `Remove`, `Contains` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `FrozenNumSet` | `Len`, `Capacity`, `Contains` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `EntityRef` | `Get`, `Set` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `EntityInfo` | `IsWaiting`, `IsActive`, `IsDespawned` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `JudgmentWindow` | `Judge` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

### 矩阵变换 (Transform2d)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

Transform2d 是 4×4 仿射变换矩阵，用于 Skin/Particle/Background 的渲染变换。
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

支持复合字面量创建和方法链：
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// 复合字面量创建
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

t := Transform2d{
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    A00: w, A11: h,
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    A22: 1, A33: 1,
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    A13: stageT,
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

}
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.SetSkinTransform(t)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// 从引擎读取 + 方法链
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

skin := sonolus.SkinTransform()
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

result := skin.Translate(sonolus.NewVec2(1, 2)).Rotate(0.5)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.SetSkinTransform(result)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

对标 Python `Transform2d(a00=w, a11=h)` 和 JS `skin.transform`。
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

### 实体信息 (EntityInfo)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

跨实体信息查询与 JS/Python 对照：
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// 常量
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.EntityStateWaiting   // 0
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.EntityStateActive    // 1
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.EntityStateDespawned // 2
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// 跨实体 — 结构化访问（对应 sonolus.js: entityInfos.get(idx)）
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

info := sonolus.EntityInfoAt(idx)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

info.State                   // entity idx 的状态 (0/1/2)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

info.Index                   // entity idx 的自身索引
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

info.Archetype               // entity idx 的原型 ID
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

info.IsActive()              // 等同于 info.State == 1
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// 跨实体 — 内联访问（不经过临时变量，最优 IR）
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

if sonolus.EntityInfoAt(idx).State == sonolus.EntityStateActive {
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    // ...
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

}
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// 跨实体 — 单字段访问（只需一个字段时避免生成多余 GetShifted）
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.EntityInfoState(idx)      // 等同于 sonolus.EntityInfoAt(idx).State
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.EntityInfoArchetype(idx)  // 等同于 sonolus.EntityInfoAt(idx).Archetype
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.EntityInfoIndex(idx)      // 等同于 sonolus.EntityInfoAt(idx).Index
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// 自身实体 — 结构化访问（对应 sonolus.js: this.info）
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.SelfInfo().State    // 从 block 4003 读取
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// 自身实体 — 标签展开
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

type Note struct {
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    Self EntityInfo `sonolus:"info"`  // → Self.Index, Self.Archetype, Self.State (只读)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

}
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

func (n *Note) Touch() {
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    if n.Self.State == sonolus.EntityStateActive { ... }
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    if n.Self.IsActive() { ... }
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

}
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// 迭代所有实体（终止条件与 JS for..of 等价）
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

for i := float64(0); sonolus.EntityInfoIndex(i) == i; i++ {
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    // i 遍历每个已存在的实体
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

}
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 场景 | sonolus.js | sonolus.py | sonolus-go |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

|------|-----------|-----------|------------|
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 跨实体状态 | `entityInfos.get(idx).state` | `entity_info_at(idx).state` | `sonolus.EntityInfoAt(idx).State` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 活跃检查 | `info.state === EntityState.Active` | `entity_info_at(idx).state == 1` | `info.IsActive()` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 自身状态 | `this.info.state` | `self._info.state` | `sonolus.SelfInfo().State` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 迭代 | `for (const info of entityInfos)` | `for idx: entity_info_at(idx)` | `for i := 0; EntityInfoIndex(i) == i; i++` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

### 皮肤精灵 (Skin / Sprite)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

通过 `sonolus.Skin().Sprites.Name` 按名引用精灵，可赋给局部变量复用：
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

type Skin struct {
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

	Note float64  // ID = 0
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

	Hold float64  // ID = 1
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

}
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// 命名空间引用（对标 JS: skin.sprites.note）
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

skin := sonolus.Skin()
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

skin.Sprites.Note.Draw(quad, z, a)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

skin.Sprites.Note.Exists()
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

skin.Sprites.Exists(0)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// 局部变量复用（对标 JS: const note = skin.sprites.note）
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sprite := skin.Sprites.Note
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sprite.Draw(quad, z, a)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

if sprite.Exists() { ... }
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 场景 | sonolus.js | sonolus.py | sonolus-go |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

|------|-----------|-----------|------------|
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 按名引用 | `skin.sprites.note` | N/A (装饰器模式) | `Skin().Sprites.Note` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 存在检查 | `sprite.exists` | `sprite.is_available` | `sprite.Exists()` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 绘制 | `sprite.draw(quad,z,a)` | `sprite.draw(quad,z,a)` | `sprite.Draw(quad,z,a)` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

### 音效片段 (EffectClip)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

通过 `sonolus.Effect().Clips.Name` 命名空间引用音效片段：
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

type Effect struct {
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

	HitSound  float64  // ID = 0
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

	MissSound float64  // ID = 1
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

}
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// 命名空间引用（对标 JS: effect.clips.hitSound）
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

effect := sonolus.Effect()
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

effect.Clips.HitSound.Play(0.1)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

effect.Clips.HitSound.Schedule(targetTime, 0.1)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 场景 | sonolus.js | sonolus.py | sonolus-go |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

|------|-----------|-----------|------------|
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 即时播放 | `clip.play(distance)` | `effect.play(distance)` | `Effect().Clips.Name.Play(d)` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 预排程 | `clip.schedule(time, d)` | `effect.schedule(time, d)` | `Effect().Clips.Name.Schedule(t, d)` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

### 粒子效果 (ParticleClip)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

通过 `sonolus.Particle().Effects.Name` 命名空间引用粒子效果。支持复合参数（Quad）自动解构：
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

type Particle struct {
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

	Explosion float64  // ID = 0
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

}
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// 命名空间引用（对标 JS: particle.effects.explosion）
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

q := sonolus.NewQuad(...)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

particle := sonolus.Particle()
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

particle.Effects.Explosion.Spawn(q, 1, 0)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

//                                    ^^ Quad 8 字段自动解构为标量
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 场景 | sonolus.js | sonolus.py | sonolus-go |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

|------|-----------|-----------|------------|
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 生成粒子 | `effect.spawn(quad, dur, loop)` | `effect.spawn(quad, dur, loop)` | `Particle().Effects.Name.Spawn(quad, dur, loop)` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 移动粒子 | `handle.move(quad)` | `handle.move(quad)` | `handle.Move(quad)`（已有） |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 销毁粒子 | `handle.destroy()` | `handle.destroy()` | `handle.Destroy()`（已有） |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

### 判定 (Judgment)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

判定计算与写入的三方对照：
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// 判定窗口定义
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

var windows = sonolus.JudgmentWindow{
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    PerfectMin: -0.05, PerfectMax: 0.05,
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    GreatMin:   -0.1,  GreatMax:   0.1,
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    GoodMin:    -0.15, GoodMax:    0.15,
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

}
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// 计算判定等级（对应 JS: input.judge, Python: window.judge）
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

level := windows.Judge(actualTime, targetTime)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// level = 0 (Miss), 1 (Perfect), 2 (Great), 3 (Good)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

// 写入判定结果（推荐：PlayEntityInput）
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

n.Result.Judgment = level
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 场景 | sonolus.js | sonolus.py | sonolus-go |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

|------|-----------|-----------|------------|
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 判定计算 | `input.judge(hitTime, targetTime, windows)` | `window.judge(actual, target)` | `windows.Judge(actual, target)` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 判定写入 | `this.result.judgment = judgment` | `self.result.judgment = judgment` | `n.Result.Judgment = level` 或 `n.Result.Judgment = level` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 桶写入 | `this.result.bucket.index = idx` | `self.result.bucket = Bucket(id=idx)` | `n.Result.BucketIndex = idx` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 判定等级 | `Judgment.Perfect` (1) | `Judgment.PERFECT` (1) | 裸 float64: 0/1/2/3 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 引擎 ops | 1 Judge + 1 SetShifted | ~13 比较 + 1 Set | 1 Judge (+ 1 Set) |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

### Canvas 打印 (Preview 模式)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

canvas := sonolus.Canvas()
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

通过 `canvas.Print()` 在 Preview 模式打印数值。非 Preview 模式编译时消除（零 IR 节点）。
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

canvas := sonolus.Canvas()
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

canvas.Print(PrintOptions{
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    Value:   123,
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    Format:  0,          // 0=Number, 1=Percent, 10=Time, ...
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    AnchorX: 0.5, AnchorY: 0.5,
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    SizeX: 16, SizeY: 16,
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    // Color/Alpha/Rotation 等取零值默认值
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

})
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 场景 | sonolus.js | sonolus.py | sonolus-go |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

|------|-----------|-----------|------------|
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 打印数值 | `canvas.print({value,fmt,...})` | `print_number(value,fmt=...)` | `Canvas().Print(PrintOptions{...})` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 默认值 | ❌ 全部必填 | ✅ 关键字默认值 | ✅ 零值 = 默认 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 模式消除 | N/A (仅 Preview) | N/A (仅 Preview) | ✅ 编译时 no-op |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

### 结构化 Score/Life (EntityScore / EntityLife)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

通过 `sonolus:"scored"` / `sonolus:"lifed"` 标签使用结构化记录类型：
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

type Note struct {
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    Score EntityScore `sonolus:"scored"`  // block 4006: Score.Perfect..Miss
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    Life  EntityLife  `sonolus:"lifed"`   // block 4007: Life.Perfect..Miss
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

}
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

n.Score.Perfect = 100    // 对齐 JS: this.entityScore.perfect = 100
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

n.Life.Miss = -50        // 对齐 JS: this.entityLife.miss = -50
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 场景 | sonolus.js | sonolus.py | sonolus-go |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

|------|-----------|-----------|------------|
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 分数写入 | `this.entityScore.perfect = 100` | `self.entity_score_multiplier` | `n.Score.Perfect = 100` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 生命写入 | `this.entityLife.miss = -50` | `self.entity_life.miss = -50` | `n.Life.Miss = -50` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 关卡基础分 | `score.base.perfect` | 无独立 API | `sonolus.Score().Base.Perfect` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 关卡连击分 | `score.consecutive.perfect.multiplier` | ❌ | `s.Consecutive.Perfect.Multiplier` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 连击生命递增 | `life.consecutive.perfect.increment` | ❌ | `sonolus.Life().Consecutive.Perfect.Increment` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 初始/最大生命 | `life.initial` / `life.max` | ❌ | `sonolus.Life().Initial` / `.Max` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 原型生命 | `life.archetypes.get(idx).miss` | `archetype_life` | `life.Archetypes[idx].Miss` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 延迟变化 | `life.addScheduled(v, t)` | ❌ | `life.AddScheduled(v, t)` |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

### 结构化 EntityInput
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

通过 `sonolus:"input"` 标签使用 `PlayEntityInput` 记录类型，一个字段替代五个：
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

type Note struct {
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    Result PlayEntityInput `sonolus:"input"`  // → Result.Judgment..Result.Haptic
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

}
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

func (n *Note) Touch() {
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    n.Result.Judgment = 1        // 4005[0]
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    n.Result.BucketIndex = 0     // 4005[2]
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

    n.Result.Accuracy = sonolus.Abs(actual - target)  // 4005[1]
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

}
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

对标 JS `this.result.bucket.index`，Python `self.result.bucket`。
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

## 静态构造器
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```go
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.NewVec2(x, y)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.NewQuad(blx, bly, tlx, tly, trx, try, brx, bry)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.NewMat(m11, m12, m13, m21, m22, m23)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.NewRect(t, r, b, l)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.NewPair(first, second)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.NewVarArray(capacity)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.NewArrayMap(capacity)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.NewArraySet(capacity)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.NewFrozenNumSet(capacity)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.NewEffectClip(id)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

sonolus.NewParticleClip(id)
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

```
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

> 兼容旧式裸名：`vec2(x, y)`, `quad(...)`, `mat(...)` 等。 新项目建议使用 `sonolus.` 前缀构造器。
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

## 不支持的特性
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

这些 Go 构造被编译器拒绝：
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 构造 | 原因 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

|------|------|
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `defer` / `go` (goroutine) | 引擎单线程，无运行时调度 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `chan` / `select` | 无并发支持 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| `map` / `interface{}` | 运行时仅有 float64，无堆类型 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 递归 | 函数内联展开，无法递归调用 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 多返回值函数 (`func f() (a, b float64)`) | 用户函数仅单返回值；复合解构 `a, b := pair.Tuple()` 支持 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 变参用户函数 (`func f(args ...float64)`) | 仅内置函数支持变参 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| struct 嵌套 / 匿名嵌入 | 内存布局必须平坦 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

| 类型别名 / 类型定义 (`type A B`) | 仅支持 struct 定义 |
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

---
```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```


```go
s := sonolus.Score()
s.Base.Perfect = 1000000         // 链式写入
s.Consecutive.Perfect.Multiplier = 1
```

> 参考：[快速入门](getting-started.md) · [CLI 参考](cli.md) · [编译器架构](architecture.md) · [优化器](optimization.md)
