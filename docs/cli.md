# CLI 参考

## 命令概览

```bash
sonolus-go [command] [flags]
```

| 命令 | 说明 |
|------|------|
| `build` | 编译引擎源码为 EngineData |
| `serve` | 本地开发服务器（含热编译） |
| `pack` | Sonolus 包源树输出 |
| `host` | 生产模式打包 + HTTP 服务 |
| `level` | 关卡数据编译 |

## `build`

```bash
sonolus-go build <source> [-m <mode>] [-o <dir>] [-O <level>] [--rom <path>] [--stats]
```

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `<source>` | （必填） | `.go` 文件或源码目录路径 |
| `-m play` | `play` | 仅编译 Play 模式 |
| `-m watch` | — | 仅编译 Watch 模式 |
| `-m preview` | — | 仅编译 Preview 模式 |
| `-m tutorial` | — | 仅编译 Tutorial 模式 |
| `-m all` | — | 并行编译全部四种模式 |
| `-o <dir>` | `dist` | 输出目录 |
| `-O 0` | `2` | 最小优化（~6 pass，调试用） |
| `-O 1` | — | 快速优化（~26 pass） |
| `-O 2` | — | 标准优化（~44 pass，默认） |
| `--rom <path>` | 内置默认 | 自定义 ROM 文件（raw float32 二进制，4 字节对齐） |
| `--stats` | `false` | 打印各模式编译耗时统计 |

## `serve`

```bash
sonolus-go serve <source> [-addr <host:port>] [--rom <path>]
```

启动本地 HTTP 服务器，**编译全部四种模式**。源码变更时自动重编译并热加载。

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `<source>` | （必填） | `.go` 文件或源码目录路径 |
| `-addr <host:port>` | `:8080` | 监听地址 |
| `--rom <path>` | 内置默认 | 自定义 ROM 文件 |

## `pack`

```bash
sonolus-go pack <source> [-author <name>] [--rom <path>]
```

编译全部四种模式，输出 sonolus-pack 兼容的源树到 `dist/` 目录。
不启动服务器——仅生成文件。

## `host`

```bash
sonolus-go host <source> [-addr <host:port>] [-author <name>]
```

编译引擎 → 打包 → 启动生产级 HTTP 服务器（通过 sonolus-server-go）。

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `<source>` | （必填） | `.go` 文件或源码目录路径 |
| `-addr <host:port>` | `:8080` | 监听地址 |
| `-author <name>` | `sonolus-go` | 引擎作者名 |

## 环境变量

sonolus-go 不依赖环境变量。所有配置通过命令行标志传递。发布 Release 的二进制为静态链接（`CGO_ENABLED=0`），无需任何运行时依赖。

---

> 参考：[快速入门](getting-started.md) · [DSL 语言参考](dsl-reference.md) · [编译器架构](architecture.md)
