# 编译器架构

sonolus-go 将 Go 源代码子集编译为 Sonolus 引擎可加载的 EngineData。编译流水线分为六个阶段：

```
Go源文件 (.go)
  │
  ▼ internal/compiler/frontend     ← go/parser + go/types 类型检查 + AST 追踪 → CFG IR
  │
  ▼ internal/compiler/ir/optimize  ← CFG/SSA 优化器 (~40 个 pass: SCCP/LICM/CSE/InlineVars...)
  │
  ▼ internal/compiler/ir/finalize  ← 寄存器分配 + 指令扁平化
  │
  ▼ internal/compiler/snode        ← 运行时节点树 (SNode) + 去重 + 序列化
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
| `internal/compiler/ir` | IR 节点定义、CFG、基本块、寄存器分配 |
| `internal/compiler/ir/optimize` | SSA 构造、SCCP、LICM、CSE、内联、DCE/UCE |
| `internal/compiler/snode` | Sonolus 运行时节点树 + Peephole 优化 |
| `internal/compiler/engine` | 引擎编译协调（解析、类型检查、编译回调、资源构建） |
| `internal/compiler/play` | Play 模式装配 |
| `internal/compiler/watch` | Watch 模式装配 |
| `internal/compiler/preview` | Preview 模式装配 |
| `internal/compiler/tutorial` | Tutorial 模式装配 |
| `internal/compiler/modecompile` | 跨模式共享编译工具 |
| `internal/compiler/build` | EngineData 输出包装 + ROM 构建 |
| `internal/compiler/pack` | Sonolus 包源树输出 |
| `internal/compiler/level` | 关卡数据解析 |
| `cmd/sonolus-go` | CLI 入口 |

## 优化器流水线

优化器提供三级优化：

- **Standard**: 完整 SSA 往返 → SCCP ×2 → InlineVars ×2 → LICM → CSE → 寄存器分配
- **Fast**: 单轮 SSA + 内联 + DCE
- **Minimal**: 仅基本合并 + UCE + DCE（无 SSA）

详见 [优化器文档](optimization.md)。
