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

## 命令行

```bash
# 编译单一模式
sonolus-go build -m play ./engine/

# 编译全部四种模式
sonolus-go build -m all ./engine/

# 本地开发服务器 (带热编译，自动编译四种模式)
sonolus-go serve ./engine/

# 生产模式打包 + 服务
sonolus-go host ./engine/ -author "YourName"
```

详见 [快速入门](docs/getting-started.md) 和 [CLI 参考](docs/cli.md)。

## 参照项目

- [sonolus.js](https://github.com/NonSpicyBurrito/sonolus.js) — 引擎 DSL 标准库（TypeScript 运行时）
- [sonolus.js-compiler](https://github.com/NonSpicyBurrito/sonolus.js-compiler) — JS → 引擎节点编译器（结构参照）
- [sonolus.py](https://github.com/NonSpicyBurrito/sonolus.py) — Python 实现 / 性能目标（45k LOC，最完整）

## 许可

MIT
