# 命令行参考

## 通用输入

引擎命令直接接受一个或多个 `packages.Load` pattern：

```bash
sonolus-go build ./engine
sonolus-go build -name game ./engine ./shared
sonolus-go build -name game ./engines/...
```

只有单个明确目录 pattern 可以自动推导 engine name。多个 pattern、import pattern 或 wildcard 必须显式传 `-name`。不支持旧式单 `.go` 文件 prelude 输入。

## build

```text
sonolus-go build [-name <name>] [-o <dir>] [-m <mode>]
                 [-O 0|1|2] [-rom <file>] [-stats] <pattern>...
```

参数：

- `-name`：引擎名称。
- `-o`：输出根目录，默认 `dist`。
- `-m`：`play`、`watch`、`preview`、`tutorial` 或 `all`，默认 `all`。
- `-O`：`0=minimal`、`1=fast`、`2=standard`，默认 `2`。
- `-rom`：原始 little-endian float32 ROM fallback。
- `-stats`：输出各模式 load/frontend 和共享 optimize/backend/total 时间。

源码声明 ROM 优先。源码 ROM 缺失或显式为空时使用 `-rom` fallback。fallback 长度必须是 4 的倍数。

输出位于 `<out>/<name>`，采用原子目录替换；编译或序列化失败不会留下部分新产物。

## serve

```text
sonolus-go serve [-name <name>] [-addr <:8080>]
                 [-O 0|1|2] [-rom <file>] [-stats] <pattern>...
```

`serve` 总是编译四种模式，提供开发期端点：

- `/sonolus/engines/info`
- `/sonolus/engine/configuration`
- `/sonolus/engine/play-data`
- `/sonolus/engine/watch-data`
- `/sonolus/engine/preview-data`
- `/sonolus/engine/tutorial-data`
- `/sonolus/engine/rom`

它监听成功快照中的 Go 和 embed 文件。文件变化后创建新的 Compiler 并重新编译；失败时记录错误并继续服务上一次成功快照。

## pack

```text
sonolus-go pack [-name <name>] [-author <name>]
                [-O 0|1|2] [-rom <file>] [-stats] <pattern>...
```

`pack` 编译全部四种模式，生成临时 `sonolus-pack-go` source tree，并输出到：

```text
dist/<name>-pack
```

默认 author 为 `sonolus-go`。当前 adapter 使用默认 skin、background、effect 和 particle item 引用，并生成满足 pack schema 的基础 item。

## level

```text
sonolus-go level [-o <dir>] <chart.json>
```

读取 JSON level definition，转换为 `resource.LevelData` 并 gzip 写为：

```text
<out>/<chart-name>/LevelData
```

`level` 是纯数据打包，不经过 engine compiler。

## version

```bash
sonolus-go version
sonolus-go -version
sonolus-go --version
```

输出构建时注入的版本、commit 和日期；未注入时显示开发默认值。
