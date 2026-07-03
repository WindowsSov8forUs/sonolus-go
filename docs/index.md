# sonolus-go

全 Go 语言实现的 Sonolus 引擎编译工具链。将 Go 源代码子集编译为 Sonolus EngineData，支持 Play / Watch / Preview / Tutorial 四种模式。

```bash
go install github.com/WindowsSov8forUs/sonolus-go/cmd/sonolus-go@latest
sonolus-go build -m play engine.go
```

## 文档

| 文档 | 说明 |
|------|------|
| [快速入门](getting-started.md) | 环境准备、第一个引擎、基本用法 |
| [DSL 语言参考](dsl-reference.md) | 完整语法、类型系统、运行时函数、多文件引擎 |
| [CLI 参考](cli.md) | build / serve / pack / host / level 命令 |
| [编译器架构](architecture.md) | 编译流水线、包结构 |
| [优化器](optimization.md) | 三级优化流水线、Pass 列表 |
| [性能基准](performance.md) | 编译性能数据 |

## 许可

MIT
