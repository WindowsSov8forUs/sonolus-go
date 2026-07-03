# CLAUDE.md

## 项目概述

sonolus-go 是全 Go 语言实现的 Sonolus 引擎编译工具链。将 Go 源代码子集编译为 Sonolus 可加载的 EngineData（EngineConfiguration + EnginePlayData / EngineWatchData / EnginePreviewData / EngineTutorialData），支持四种模式完整打包与本地开发服务器。

## 构建

- `make build` — 编译所有包
- `make test` — 运行全部测试
- `make vet` — 运行 go vet
- `make fmt` — 格式化所有源文件
- `make fmt-check` — 检查格式 (CI)
- `make clean` — 清除构建产物

## 架构

```
Go源文件 (.go)
  │
  ▼ compiler/frontend     ← go/parser + go/types 类型检查 + AST 追踪 → IR
  │
  ▼ compiler/ir/optimize  ← CFG/SSA 优化器 (~40 个 pass: SCCP/LICM/CSE/...)
  │
  ▼ compiler/ir/finalize  ← 寄存器分配 + 指令扁平化
  │
  ▼ compiler/snode        ← 运行时节点树 (SNode) + 去重 + 序列化
  │
  ▼ compiler/{play,watch,preview,tutorial} + compiler/modecompile
  │                       ← 四模式装配 + 回调 body 合成
  │
  ▼ compiler/build        ← EngineData 包装 + ROM
  │
  ▼ compiler/pack         ← sonolus-pack 源树输出
  │
  ▼ cmd/sonolus-go        ← CLI: build / serve / host / pack / level
```

## 代码生成

- `go generate ./compiler/ir/` — 从 `sonolus.py/sonolus/backend/ops.py` 重新生成 `ops_gen.go`（需要同级目录存在 sonolus.py 克隆）

### 关键设计决策

- **前端**: 使用 Go 标准库 `go/ast` + `go/types` 解析引擎源码。引擎源码是 Go 语法的受限子集——仅支持 struct 定义、方法、基本控制流、运行时函数调用。不支持 defer/go/select/channel/map/interface。
- **IR**: `compiler/ir/` 定义了 CFG 形状的中间表示（`BasicBlock` → `FlowEdge`），直接端口自 sonolus.py 的 `sonolus/backend/`。`IDGen` 是每个编译实例独立的单调 ID 生成器，消除共享可变状态，确保并发编译安全。
- **优化器**: 端口自 sonolus.py 的全部 22 个优化 Pass。支持 3 个优化级别（Minimal/Fast/Standard）。Pass 接口 + ManagedPass 分析声明提供静态依赖验证。
- **模式**: 四种模式（Play/Watch/Preview/Tutorial）通过 `compiler/modecompile` 的泛型框架共享核心装配逻辑，模式特定差异在各自 package 中通过 Callback 常量和 OmitFunc 处理。
- **确定性**: `RenumberVars` pass 确保变量重编号确定性（所有优化级别）。`Appender` 提供 SNode 树的共享节点去重。

## 代码约定

- **Pass 接口**: 所有优化 Pass 实现 `Pass` 接口（`Name() string` + `Run(gen, entry)`）。分析型 Pass（需要 SSA/支配树）额外实现 `ManagedPass` 接口声明依赖。
- **错误处理**: 使用 `fmt.Errorf` + `%w` 包装错误，沿调用链向上传播。
- **测试**: Golden file 测试用于优化器（`optimize_test.go`）和端到端输出（`golden_test.go`）。使用 `-update` 标志重新生成 golden 文件。Fuzz 测试覆盖编译器的 crash 安全性。
- **并发**: 所有编译器入口点均通过 `IDGen` 的实例隔离实现线程安全。`CompileCache` 使用 `sync.RWMutex` 提供并发安全的编译缓存。
- **命名**: 变量/函数使用 camelCase，导出符号使用 PascalCase。与 sonolus.py 对齐的类型名保留原样（如 `BasicBlock`、`SSAPlace`）。

## 参考项目

| 项目 | 路径 | 用途 |
|------|------|------|
| sonolus.py | `../sonolus.py` | 优化器 Pass 端口来源，性能目标 |
| sonolus.js-compiler | `../sonolus.js-compiler` | 回调编译逻辑参照 |
| sonolus-core-go | `../sonolus-core-go` | EngineData 数据结构定义 |
| sonolus-pack-go | `../sonolus-pack-go` | 打包工具 |
| sonolus-server-go | `../sonolus-server-go` | 生产服务器接口 |

### 关键端口对照

| sonolus-go | sonolus.py |
|-----------|-----------|
| `compiler/ir/optimize/ssa.go` | `backend/optimize/ssa.py` |
| `compiler/ir/optimize/sccp.go` | `backend/optimize/constant_evaluation.py` |
| `compiler/ir/optimize/pipeline.go` | `backend/optimize/optimize.py` |
| `compiler/ir/optimize/inlining.go` | `backend/optimize/inlining.py` |
| `compiler/ir/optimize/cse.go` | `backend/optimize/cse.py` |
| `compiler/ir/optimize/licm.go` | `backend/optimize/licm.py` |
| `compiler/modecompile/modecompile.go` | `build/shared/utils/compile.ts` (JS) |

## 测试策略

```bash
# 全部测试
go test ./...

# 仅优化器测试
go test ./compiler/ir/optimize/ -v -count=1

# 更新 golden 文件
go test ./compiler/engine/ -run TestGolden -update

# Fuzz 测试
go test -fuzz=FuzzCompilePlay -fuzztime=30s ./compiler/engine/
```

## 许可

MIT
