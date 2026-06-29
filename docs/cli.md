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

## `serve`

```bash
sonolus-go serve -m <mode> [--port <n>] <source>
```

启动本地 HTTP 服务器，提供 Sonolus 引擎接口。源码变更时自动重编译。

| 参数 | 说明 |
|------|------|
| `-m <mode>` | 编译模式（play/watch/preview/tutorial/all） |
| `--port <n>` | 监听端口（默认 8080） |

## `pack`

```bash
sonolus-go pack serve <source> -author "YourName"
```

生产模式打包 + 服务。

## 环境变量

| 变量 | 说明 |
|------|------|
| `SONOLUS_PORT` | 服务器端口（默认 8080） |
| `SONOLUS_AUTHOR` | 引擎作者名 |
