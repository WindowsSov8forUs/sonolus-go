# sonolus-go

全 Go 语言实现的 Sonolus 引擎编译工具链。

将 Go 源代码子集编译为 Sonolus 可加载的 EngineData（EngineConfiguration +
EnginePlayData / EngineWatchData / EnginePreviewData / EngineTutorialData），
支持四种模式完整打包与本地开发服务器。

## 安装

从 [GitHub Releases](https://github.com/WindowsSov8forUs/sonolus-go/releases) 下载预编译二进制，无需 Go 环境。

或从源码编译（需 Go 1.25+，依赖自动拉取）：

```bash
git clone https://github.com/WindowsSov8forUs/sonolus-go.git
cd sonolus-go
go build ./cmd/sonolus-go/
```

### 开发

```bash
go build ./...    # 编译
go test ./...     # 测试
go vet ./...      # 静态分析
```

## 架构

```
Go package patterns
  │
  ▼ compiler/source    ← 每模式 build tag + packages.Load
  │
  ▼ compiler/frontend  ← 声明解析 + Go DSL lowering
  │
  ▼ compiler/ir        ← 强类型 CFG IR
  │
  ▼ compiler/optimize  ← Minimal / Fast / Standard IR 优化与 local 分配
  │
  ▼ compiler/backend   ← SNode peephole + 四模式 EngineData
  │
  ▼ internal/build        ← gzip EngineData / ROM
  │
  ▼ internal/pack         ← sonolus-pack 源树输出
  │
  ▼ cmd/sonolus-go        ← CLI: build / serve / pack / level
```

## 命令行

```bash
# 编译单一模式
sonolus-go build -name my-engine -m play ./engine

# 编译全部四种模式
sonolus-go build -name my-engine -m all ./engine

# 本地开发服务器 (带热编译，自动编译四种模式)
sonolus-go serve -name my-engine ./engine

# 生成可交给独立 Sonolus Server 的 pack
sonolus-go pack -name my-engine -author "YourName" ./engine
```

输入直接传给 `packages.Load`。单个明确目录可以省略 `-name` 并从目录名推导；多个
pattern、import pattern 或 wildcard 必须显式提供 `-name`。`-O` 支持
`0`（Minimal）、`1`（Fast）和 `2`（Standard，默认）。

## 参照项目

- [sonolus.js](https://github.com/NonSpicyBurrito/sonolus.js) — 引擎 DSL 标准库（TypeScript 运行时）
- [sonolus.js-compiler](https://github.com/NonSpicyBurrito/sonolus.js-compiler) — JS → 引擎节点编译器（结构参照）
- [sonolus.py](https://github.com/NonSpicyBurrito/sonolus.py) — Python 实现 / 性能目标（45k LOC，最完整）

## 许可

MIT
