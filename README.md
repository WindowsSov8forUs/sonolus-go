# sonolus-go

`sonolus-go` 是使用 Go 编写 Sonolus 引擎的编译工具链。它将受支持的 Go DSL 编译为 Play、Watch、Preview 和 Tutorial EngineData，并可生成引擎目录或 Sonolus pack。

## 下载与安装

从 [GitHub Releases](https://github.com/WindowsSov8forUs/sonolus-go/releases) 下载对应平台的预编译可执行文件，并将其加入 `PATH`。

也可以从源码安装。项目需要 Go 1.25 或更高版本：

```bash
go install github.com/WindowsSov8forUs/sonolus-go/cmd/sonolus-go@latest
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
sonolus-go build -name my-engine -m all ./engine
```

只编译 Play：

```bash
sonolus-go build -name my-engine -m play ./engine
```

启动带源码监听和自动重编译的开发服务器：

```bash
sonolus-go serve -name my-engine ./engine
```

生成 pack：

```bash
sonolus-go pack -name my-engine -author "YourName" ./engine
```

打包 level JSON：

```bash
sonolus-go level ./level.json
```

命令输入直接使用 Go package patterns。单个明确目录可以省略 `-name`；多个 pattern、import pattern 或 wildcard 需要显式指定。完整参数见[命令行参考](docs/cli.md)。

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
