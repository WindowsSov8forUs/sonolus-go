# DSL 语言参考

sonolus-go 将 Go 语法的一个受限子集编译为 Sonolus EngineData。本参考面向引擎作者——如果你刚开始，先看[快速入门](getting-started.md)。

## 快速上手

```go
package myengine

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type Skin struct { Note sonolus.Vec2 }

type Note struct {
    Beat float64 `sonolus:"imported"`
    X    float64 `sonolus:"memory"`
}

func (n *Note) Initialize() {
    n.X = sonolus.Sin(n.Beat)
    sonolus.Draw(sonolus.SpriteNote, n.X, 0, 1, 1, 0, 1, 0, 0)
}
```

编译：

```bash
sonolus-go build -m play engine.go
```

## 包声明与 Import

```go
package myengine    // 包名用作引擎名
```

### 导入子包（多文件引擎）

```go
import "notes"      // → 自动发现 ./notes/*.go 中的原型
import "stage"      // → 自动发现 ./stage/*.go 中的原型
```

子目录中只能定义原型 struct 和回调——资源类型（Skin / Effect / Buckets / Instruction）必须在主包。

### 导入 sonolus 包（推荐）

```go
import "github.com/WindowsSov8forUs/sonolus-go/sonolus"
```

之后所有运行时调用使用 `sonolus.` 前缀：

```go
sonolus.Draw(...)        // 函数调用
x := sonolus.Time        // 全局变量
pos := sonolus.Vec2_(x, y) // 构造器
sonolus.DebugPause()     // 调试
```

**函数名映射**：`lowerFirst(Sel.Name)` — `Draw` → `draw`，`DebugPause` → `debugPause`，`GetShifted` → `getShifted`。

使用 `sonolus.` 前缀的引擎源码是合法的 Go 程序——`go vet`、`gopls` 自动补全均可用。裸名（`draw(...)`）仍可工作但不通过静态检查。

详见[快速入门](getting-started.md)和[架构文档](architecture.md)。

## 结构体定义

```go
type Name struct {
    Field Type `sonolus:"tag"`
}
```

字段标签决定引擎内存布局：

| 标签 | 内存块 | 可写 | 说明 |
|------|--------|------|------|
| `imported` | EntityMemory (0) | 否 | 从父原型传入 |
| `memory` | EntityMemory (1+) | 是 | 私有每实体存储 |
| `data` | EntityData | 否 | 只读原型数据 |
| `shared` | EntityShared | 是 | 共享可变状态 |
| `input` | EntityInput | 是 | 输入状态 |
| `despawn` | EntityDespawn | 是 | 消失效果存储 |
| `info` | EntityInfo | 否 | 只读实体元数据 |
| `exported` | Exported | 是 | 导出值（仅 Play） |
| `scored` | Exported | 是 | 分数计数器（仅 Play） |
| `lifed` | Exported | 是 | 生命值（仅 Play） |

### 特殊结构体名

特定名字的 struct 定义引擎资源（大小写敏感）：

| 类型名 | 所在模式 | 说明 |
|--------|---------|------|
| `Skin` | 全部 | 精灵/纹理定义 |
| `Effect` | Play, Watch, Tutorial | 音效片段定义 |
| `Particle` | Play, Watch, Tutorial | 粒子效果定义 |
| `Buckets` | Play, Watch | 桶/生成规则定义 |
| `Instruction` | Tutorial | 教程文本/图标定义 |
| `UI` | 全部 | EngineConfiguration UI 覆写（键值标签） |

### 记录类型字段

字段类型可以是已知记录类型——编译器自动展开为多个 float64 槽位：

```go
type Note struct {
    pos Vec2  `sonolus:"memory"`   // 展开为 pos.x, pos.y (2 槽)
    m   Mat   `sonolus:"memory"`   // 展开为 6 槽
    q   Quad  `sonolus:"memory"`   // 展开为 8 槽
}
// 访问: n.pos.X = value; x := n.pos.Y
```

支持的记录类型：`Vec2`(2)、`Quad`(8)、`Mat`(6)、`Rect`(4)、`Trans`(9)、`Pair`(2)、`EntityInfo`(3)、`EntityRef`(1)、`JudgmentWindow`(6)。支持 `sonolus.Vec2` 等限定名。

## 回调方法

方法名决定编译为哪个 Sonolus 回调：

**Play 模式（8 个回调）：**
`Preprocess`, `SpawnOrder`, `ShouldSpawn`, `Initialize`, `UpdateSequential`, `Touch`, `UpdateParallel`, `Terminate`

**Watch 模式（7 个回调）：**
`Preprocess`, `SpawnTime`, `DespawnTime`, `Initialize`, `UpdateSequential`, `UpdateParallel`, `Terminate`

**Preview 模式（2 个回调）：**
`Preprocess`, `Render`

**Tutorial 模式（3 个全局函数）：**
`Preprocess()`, `Navigate() float64`, `Update()`

**Watch 全局**：`UpdateSpawn() float64`

## 控制流

```go
// if / else
if condition { ... } else { ... }

// switch
switch value {
case 0: ...
case 1, 2: ...
default: ...
}

// 无标签 switch
switch {
case x > 0: ...
default: ...
}

// for 循环（仅条件）
for i < 10 { ... }

// for range
for i := range 5 { ... }  // i = 0, 1, 2, 3, 4

// 短路运算符
if a && b { ... }  // a 为 false 时不求值 b
if a || b { ... }  // a 为 true 时不求值 b

// 复合赋值
x += 1; x *= 2; x++
```

## 变量与赋值

```go
x := 1.0        // 声明并赋值
x = 2.0         // 覆写
x += 1.0        // 复合赋值
x++             // 递增
```

所有数值类型运行时均为 `float64`。

## 类型系统

| 类型 | 运行时表示 |
|------|-----------|
| `float64`, `int`, `bool` | float64 |
| `Vec2` | 2 个 float64 (x, y) |
| `Quad` | 8 个 float64 |
| `Mat` / `Rect` / `Trans` / `Pair` | 2-9 个 float64 |
| 用户定义 struct | 带标签的 float64 字段 |

不支持：`string`、`map`、`chan`、`interface`、slice、函数类型。

## 运行时函数

完整运行时函数列表见 `internal/compiler/frontend/builtins.go`。以下是分类概览：

| 分类 | 函数示例 | 个数 |
|------|---------|------|
| 算术 | `Abs`, `Sign`, `Floor`, `Ceil`, `Round`, `Frac` | ~10 |
| 比较 | `Equal`, `NotEqual`, `Less`, `Greater` | 6 |
| 逻辑 | `And`, `Or`, `Not` | 3 |
| 数学 | `Sin`, `Cos`, `Tan`, `Log`, `Power`, `Clamp`, `Lerp` | ~30 |
| 缓动 | `EaseInSine` ... `EaseOutInElastic` | 36 |
| 内存 | `Get`, `Set`, `GetShifted`, `SetShifted`, `SetAdd*`, `SetMul*` 等 | ~40 |
| 绘制 | `Draw`, `DrawCurvedB/T/L/R/BT/LR`, `Paint` | 8 |
| 音频 | `Play`, `PlayScheduled`, `PlayLooped`, `PlayLoopedScheduled`, `StopLooped`, `StopLoopedScheduled` | 6 |
| 粒子 | `SpawnParticle`, `MoveParticle`, `DestroyParticle` | 3 |
| 实体 | `Spawn` | 1 |
| 判定 | `Judge`, `JudgeSimple`, `ExportValue` | 3 |
| 生命 | `AddLife` | 1 |
| 调试 | `DebugLog`, `DebugPause`, `DebugError`, `DebugAssertTrue` 等 | 7 |
| 随机 | `Random`, `RandomInteger` | 2 |
| Stream | `StreamSet`, `StreamHas`, `StreamGetNextKey` 等 | 5 |
| 时间 | `BeatToTime`, `TimeToScaledTime` 等 | 8 |
| 栈 | `StackInit`, `StackPush`, `StackPop` 等 | 14 |
| 触摸 | `TouchID`, `TouchStarted`, `TouchEnded`, `TouchX`, `TouchY` | 5 |
| 资源查询 | `HasSkinSprite`, `HasEffectClip`, `HasParticle` | 3 |
| 实体信息 | `EntityInfoIndex`, `EntityInfoArchetype`, `EntityInfoState`, `EntityInfoAt`, `SelfInfo` | 5 |

### 引擎全局变量

```go
time, deltaTime, scaledTime, touchCount, isSkip, isReplay,
isDebug, aspectRatio, audioOffset, inputOffset, isMultiplayer,
scrollDirection, canvasSize, navigationDirection,
safeAreaXMin, safeAreaXMax, safeAreaYMin, safeAreaYMax,
perfectMultiplier, greatMultiplier, goodMultiplier,
lifeInitial, lifeMaximum,
entityPerfect, entityGreat, entityGood, entityMiss,
entityLifePerfect, entityLifeGreat, entityLifeGood, entityLifeMiss
```

### 记录类型方法

| 类型 | 方法 |
|------|------|
| `Vec2` | `Add`, `Sub`, `Mul`, `Div`, `Magnitude`, `Dot`, `Normalize`, `NormalizeOrZero`, `Angle`, `Rotate`, `RotateAbout`, `Orthogonal`, `AngleDiff`, `SignedAngleDiff` |
| `Quad` | `Center`, `Translate`, `Scale`, `Permute`, `Rotate`, `Top`, `Right`, `Bottom`, `Left`, `Contains` |
| `Mat` | `Scale`, `Translate`, `Compose`, `Rotate` |
| `Rect` | `W`, `H`, `Center`, `Translate`, `Scale` |
| `Trans` | `Compose`, `Translate`, `Scale`, `Rotate`, `TransformVec` |
| `Pair` | `Lt`, `Le`, `Gt`, `Ge`, `Tuple` |
| `VarArray` | `Len`, `Capacity`, `IsFull`, `Append`, `Pop`, `Insert`, `Sort`, `Clear`, `Contains`, `Index`, `Remove`, `SetAdd`, `SetRemove` |
| `ArrayMap` | `Len`, `Capacity`, `Clear`, `Keys`, `Values`, `Items`, `Get`, `Set`, `Delete`, `Contains`, `Pop` |
| `ArraySet` | `Len`, `Capacity`, `Clear`, `Add`, `Remove`, `Contains` |
| `FrozenNumSet` | `Len`, `Capacity`, `Contains` |
| `EntityRef` | `Get`, `Set` |
| `EntityInfo` | `IsWaiting`, `IsActive`, `IsDespawned` |

### 实体信息 (EntityInfo)

跨实体信息查询与 JS/Python 对照：

```go
// 常量
sonolus.EntityStateWaiting   // 0
sonolus.EntityStateActive    // 1
sonolus.EntityStateDespawned // 2

// 跨实体 — 结构化访问（对应 sonolus.js: entityInfos.get(idx)）
info := sonolus.EntityInfoAt(idx)
info.State                   // entity idx 的状态 (0/1/2)
info.Index                   // entity idx 的自身索引
info.Archetype               // entity idx 的原型 ID
info.IsActive()              // 等同于 info.State == 1

// 跨实体 — 内联访问（不经过临时变量，最优 IR）
if sonolus.EntityInfoAt(idx).State == sonolus.EntityStateActive {
    // ...
}

// 跨实体 — 单字段访问（只需一个字段时避免生成多余 GetShifted）
sonolus.EntityInfoState(idx)      // 等同于 sonolus.EntityInfoAt(idx).State
sonolus.EntityInfoArchetype(idx)  // 等同于 sonolus.EntityInfoAt(idx).Archetype
sonolus.EntityInfoIndex(idx)      // 等同于 sonolus.EntityInfoAt(idx).Index

// 自身实体 — 结构化访问（对应 sonolus.js: this.info）
sonolus.SelfInfo().State    // 从 block 4003 读取

// 自身实体 — 标签展开
type Note struct {
    Self EntityInfo `sonolus:"info"`  // → Self.Index, Self.Archetype, Self.State (只读)
}
func (n *Note) Touch() {
    if n.Self.State == sonolus.EntityStateActive { ... }
    if n.Self.IsActive() { ... }
}

// 迭代所有实体（终止条件与 JS for..of 等价）
for i := float64(0); sonolus.EntityInfoIndex(i) == i; i++ {
    // i 遍历每个已存在的实体
}
```

| 场景 | sonolus.js | sonolus.py | sonolus-go |
|------|-----------|-----------|------------|
| 跨实体状态 | `entityInfos.get(idx).state` | `entity_info_at(idx).state` | `sonolus.EntityInfoAt(idx).State` |
| 活跃检查 | `info.state === EntityState.Active` | `entity_info_at(idx).state == 1` | `info.IsActive()` |
| 自身状态 | `this.info.state` | `self._info.state` | `sonolus.SelfInfo().State` |
| 迭代 | `for (const info of entityInfos)` | `for idx: entity_info_at(idx)` | `for i := 0; EntityInfoIndex(i) == i; i++` |

## 静态构造器

```go
sonolus.Vec2_(x, y)
sonolus.Quad_(blx, bly, tlx, tly, trx, try, brx, bry)
sonolus.Mat_(m11, m12, m13, m21, m22, m23)
sonolus.Rect_(t, r, b, l)
sonolus.Pair_(first, second)
sonolus.VarArray_(capacity)
sonolus.ArrayMap_(capacity)
sonolus.ArraySet_(capacity)
sonolus.FrozenNumSet_(capacity)
```

也可以用裸名：`vec2(x, y)`, `quad(...)`, `mat(...)` 等。

## 不支持的特性

这些 Go 构造被编译器拒绝：

| 构造 | 原因 |
|------|------|
| `defer` / `go` (goroutine) | 引擎单线程，无运行时调度 |
| `chan` / `select` | 无并发支持 |
| `map` / `interface{}` | 运行时仅有 float64，无堆类型 |
| 递归 | 函数内联展开，无法递归调用 |
| 多返回值函数 (`func f() (a, b float64)`) | 用户函数仅单返回值；复合解构 `a, b := pair.Tuple()` 支持 |
| 变参用户函数 (`func f(args ...float64)`) | 仅内置函数支持变参 |
| struct 嵌套 / 匿名嵌入 | 内存布局必须平坦 |
| 类型别名 / 类型定义 (`type A B`) | 仅支持 struct 定义 |

---

> 参考：[快速入门](getting-started.md) · [CLI 参考](cli.md) · [编译器架构](architecture.md) · [优化器](optimization.md)
