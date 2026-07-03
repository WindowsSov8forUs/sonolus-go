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
sonolus-go build -m <mode> [--output <dir>] <source>
```

| 参数 | 说明 |
|------|------|
| `-m play` | 仅编译 Play 模式 |
| `-m watch` | 仅编译 Watch 模式 |
| `-m preview` | 仅编译 Preview 模式 |
| `-m tutorial` | 仅编译 Tutorial 模式 |
| `-m all` | 编译全部四种模式 |
| `--output <dir>` | 输出目录（默认 `./dist`） |
| `-O <0/1/2>` | 优化级别：0=minimal, 1=fast, 2=standard（默认 2） |
| `--rom <path>` | 自定义 ROM 文件路径（原始 float32 二进制） |
| `--stats` | 打印各模式编译耗时统计 |

## `serve`

```bash
sonolus-go serve <source> [-addr <:8080>]
```

启动本地 HTTP 服务器，提供 Sonolus 引擎接口（四种模式全部编译）。
源码变更时自动重编译并热加载。

| 参数 | 说明 |
|------|------|
| `-addr <host:port>` | 监听地址（默认 `:8080`） |

## `pack`

```bash
sonolus-go pack <source> [-author <name>]
```

编译引擎并输出 sonolus-pack 兼容的源树到 `dist/` 目录。
不启动服务器——仅生成文件。

## `host`

```bash
sonolus-go host <source> [-addr <:8080>] [-author <name>]
```

编译引擎 → 打包 → 启动生产级 HTTP 服务器（通过 sonolus-server-go）。

| 参数 | 说明 |
|------|------|
| `-addr <host:port>` | 监听地址（默认 `:8080`） |
| `-author <name>` | 引擎作者名（默认 `sonolus-go`） |

## 环境变量

sonolus-go 不使用环境变量。所有配置通过命令行标志传递。

---

> 参考：[快速入门](getting-started.md) · [DSL 语言参考](dsl-reference.md) · [编译器架构](architecture.md)
