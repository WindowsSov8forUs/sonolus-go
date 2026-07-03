# sonolus-go

全 Go 语言实现的 Sonolus 引擎编译工具链。

将 Go 源代码子集编译为 Sonolus 可加载的 **EngineData**（EngineConfiguration +
EnginePlayData / EngineWatchData / EnginePreviewData / EngineTutorialData），
支持四种模式完整打包与本地开发服务器。

## 快速开始

```bash
# 编译单一模式
sonolus-go build -m play ./engine/

# 编译全部四种模式
sonolus-go build -m all ./engine/

# 本地开发服务器 (带热编译)
sonolus-go serve -m play ./engine/
```

详见 [快速入门](getting-started.md)。

## 架构

```
Go源文件 (.go)
  → internal/compiler/frontend     (AST 追踪 → CFG IR)
  → internal/compiler/ir/optimize  (~40 个优化 pass)
  → internal/compiler/ir/finalize  (寄存器分配 + 指令扁平化)
  → internal/compiler/snode        (SNode 去重 + 序列化)
  → internal/compiler/{play,watch,preview,tutorial}
  → internal/compiler/build        (EngineData 包装)
  → internal/compiler/pack         (sonolus-pack 源树输出)
```

详见 [编译器架构](architecture.md)。

## 参照项目

- [sonolus.js](https://github.com/NonSpicyBurrito/sonolus.js) — 引擎 DSL 标准库
- [sonolus.js-compiler](https://github.com/NonSpicyBurrito/sonolus.js-compiler) — JS → 引擎节点编译器
- [sonolus.py](https://github.com/NonSpicyBurrito/sonolus.py) — Python 实现（最完整）

## 许可

MIT
