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

引擎源码必须位于 Go module 中，入口 package 为 `main`。本仓库的 [`godori/`](godori/) 是一个可游玩的四模式引擎，也是公开示例和端到端验收工程。其声明方式和项目结构见[快速开始](docs/getting-started.md)。

引擎 package 是 `sonolus-go` compiler 的源码输入，不是通过 `go run` 执行玩法逻辑的普通 Go 程序。`sonolus` 及其子包只提供编译期 API 桩；其中的零值函数体和空操作不代表 Sonolus Runtime 行为。

初始化最小四模式引擎：

```bash
sonolus-go mod init example.com/sirius sirius
sonolus-go work init ./sirius
sonolus-go init ./sirius/engine
go -C sirius mod tidy
sonolus-go vet ./sirius/engine
```

编译全部四种模式：

```bash
sonolus-go build -o godori -m all ./godori
```

只编译 Play：

```bash
sonolus-go build -o my-engine -m play ./engine
```

检查引擎能否完整编译但不生成产物：

```bash
sonolus-go vet ./godori
```

输出供关卡工具使用的 archetype 字段 schema：

```bash
sonolus-go list ./godori
```

启动带源码监听和自动重编译的开发服务器：

```bash
sonolus-go dev -o godori ./godori
```

`dev` 提供可由 Sonolus 客户端直接打开的开发关卡列表。可选的嵌入式 Development Level 声明见[快速开始](docs/getting-started.md#开发服务器)。

命令输入直接使用 Go package patterns。未指定 `-o` 时可一次编译多个引擎，各引擎名称取其 module path 的最后一段；指定 `-o` 时只允许匹配一个引擎。产物固定写入 `dist/<name>`。完整参数见[命令行参考](docs/cli.md)。

## 文档

- [文档首页](docs/index.md)
- [快速开始](docs/getting-started.md)
- [Go DSL 参考](docs/dsl-reference.md)
- [命令行参考](docs/cli.md)
- [编译器架构](docs/architecture.md)
- [优化器](docs/optimization.md)
- [性能与规模](docs/performance.md)
- [维护指南](docs/maintenance.md)

## 许可

[MIT](LICENSE)
