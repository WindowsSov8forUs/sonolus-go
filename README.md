# sonolus-go

`sonolus-go` 是使用 Go 编写 Sonolus 引擎的编译工具链。它将受支持的 Go DSL 编译为 Play、Watch、Preview 和 Tutorial EngineData，并生成可供后续工具消费的引擎目录。

## 下载与安装

从 [GitHub Releases](https://github.com/WindowsSov8forUs/sonolus-go/releases) 下载对应平台的预编译可执行文件，并将其加入 `PATH`。

也可以从源码安装。项目需要 Go 1.25.12 或更高版本：

```bash
go install github.com/WindowsSov8forUs/sonolus-go/v2/cmd/sonolus-go@latest
```

开发当前仓库时可直接编译：

```bash
git clone https://github.com/WindowsSov8forUs/sonolus-go.git
cd sonolus-go
go build ./cmd/sonolus-go
```

验证安装：

```bash
sonolus-go version
```

## 基础使用

引擎源码必须位于 Go module 中，入口 package 为 `main`。从一个最小四模式项目开始，请阅读[快速开始](docs/getting-started.md)。

编译全部四种模式：

```bash
sonolus-go build -m all ./engine
```

只编译 Play：

```bash
sonolus-go build -o my-engine -m play ./engine
```

检查引擎能否完整编译但不生成产物：

```bash
sonolus-go vet ./engine
```

输出供关卡工具使用的 archetype 字段 schema：

```bash
sonolus-go list ./engine
```

启动带源码监听和自动重编译的开发服务器：

```bash
sonolus-go dev ./engine
```

`dev` 提供可由 Sonolus 客户端直接打开的 Dev Level。可选的嵌入式开发 LevelData 声明见[快速开始](docs/getting-started.md#开发服务器)。

命令输入直接使用 Go package patterns。未指定 `-o` 时可一次编译多个引擎，各引擎名称取其 module path 的最后一段；指定 `-o` 时只允许匹配一个引擎。产物固定写入 `dist/<name>`。完整参数见[命令行参考](docs/cli.md)。

## 文档

- [文档首页](docs/index.md)
- [快速开始](docs/getting-started.md)
- [Go DSL 参考](docs/dsl-reference.md)
- [命令行参考](docs/cli.md)
- [编译器架构](docs/architecture.md)
- [优化器](docs/optimization.md)
- [性能与规模](docs/performance.md)

## 许可

[MIT](LICENSE)
