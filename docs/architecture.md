# 编译器架构

sonolus-go 编译流水线分六个阶段：

```
Go 源文件 (.go)
  │
  ▼ internal/compiler/frontend     ← go/parser + go/types 类型检查 + AST 追踪 → CFG IR
  │
  ▼ internal/compiler/ir/optimize  ← CFG/SSA 优化器 (~40 pass: SCCP/LICM/CSE/InlineVars...)
  │
  ▼ internal/compiler/ir/finalize  ← 寄存器分配 + 指令扁平化
  │
  ▼ internal/compiler/snode        ← SNode 树 + 去重 + 序列化
  │
  ▼ internal/compiler/{play,watch,preview,tutorial} + internal/compiler/modecompile
  │                       ← 四模式装配 + 回调 body 合成
  │
  ▼ internal/compiler/build        ← EngineData 包装 + ROM
  │
  ▼ internal/compiler/pack         ← sonolus-pack 源树输出
  │
  ▼ cmd/sonolus-go        ← CLI: build / serve / host / pack / level
```

## 包职责

| 包 | 职责 |
|---|---|
| `internal/compiler/frontend` | Go AST → CFG IR 追踪器 |
| `internal/compiler/ir` | IR 节点、CFG、基本块、寄存器分配 |
| `internal/compiler/ir/optimize` | SSA 构造、SCCP、LICM、CSE、内联、DCE/UCE |
| `internal/compiler/snode` | SNode 树 + Peephole 优化 |
| `internal/compiler/engine` | 编译协调（解析、类型检查、回调编译、资源构建） |
| `internal/compiler/play` | Play 模式装配 |
| `internal/compiler/watch` | Watch 模式装配 |
| `internal/compiler/preview` | Preview 模式装配 |
| `internal/compiler/tutorial` | Tutorial 模式装配 |
| `internal/compiler/modecompile` | 跨模式共享编译工具 |
| `internal/compiler/build` | EngineData 输出 + ROM 构建 |
| `internal/compiler/pack` | Sonolus 包源树输出 |
| `internal/compiler/level` | 关卡数据解析 |
| `cmd/sonolus-go` | CLI 入口 |
| `sonolus/` | 公开声明桩包（供引擎源码 import） |

## 优化器流水线

三级优化，通过 `-O` 标志选择：

| 级别 | Pass 数 | 特征 |
|------|---------|------|
| Standard (2) | ~44 | 完整 SSA ×2 + SCCP ×2 + InlineVars ×3 + LICM + CSE |
| Fast (1) | ~26 | 单轮 SSA + InlineVars + DCE |
| Minimal (0) | ~6 | Coalesce + UCE + DCE |

详见[优化器文档](optimization.md)。

## 关键设计决策

- **前端**：`go/parser` + `go/types` 解析 Go 语法子集，prelude 注入提供运行时函数声明
- **IR**：CFG 形状的中间表示（`BasicBlock` → `FlowEdge`），端口自 sonolus.py
- **并发**：`IDGen` 实例隔离 + `CompileCache` sync.RWMutex
- **确定性**：`RenumberVars` 确保变量编号确定性

---

> 参考：[DSL 语言参考](dsl-reference.md) · [优化器](optimization.md) · [性能基准](performance.md)
