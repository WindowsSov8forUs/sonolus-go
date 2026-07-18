# 命令行参考

## 通用输入

引擎命令直接接受一个或多个 `packages.Load` pattern：

```bash
sonolus-go build ./engine
sonolus-go build ./engines/...
sonolus-go build -o game ./engine
```

未指定 `-o` 时，所有匹配到的 `main` package 都会作为独立引擎编译，名称取各自主 module path 的最后一段。多个 module 导出相同名称时报错。指定 `-o` 时，该值覆盖引擎名称，且 patterns 必须恰好匹配一个引擎，这与 `go build -o` 对多 package 的限制一致。不支持旧式单 `.go` 文件 prelude 输入。

## build

```text
sonolus-go build [-o <name>] [-m <mode>]
                 [-O 0|1|2] [-runtime-checks none|terminate|notify]
                 [-rom <file>] [-stats] <pattern>...
```

参数：

- `-o`：覆盖引擎名称；指定时只允许匹配一个引擎。
- `-m`：`play`、`watch`、`preview`、`tutorial` 或 `all`，默认 `all`。
- `-O`：`0=minimal`、`1=fast`、`2=standard`，默认 `2`。
- `-rom`：原始 little-endian float32 ROM fallback。
- `-stats`：输出各模式 load/frontend 和共享 optimize/backend/total 时间。
- `-runtime-checks`：动态检查策略，默认 `none`。

源码声明 ROM 优先。源码 ROM 缺失或显式为空时使用 `-rom` fallback。fallback 长度必须是 4 的倍数。未声明 ROM、未提供 fallback 且 callback 不读取 ROM 时，输出中省略 `EngineRom`。

输出固定位于 `dist/<name>`，采用原子目录替换；编译或序列化失败不会留下部分新产物。

## vet

```text
sonolus-go vet [-m <mode>] [-O 0|1|2]
                 [-runtime-checks none|terminate|notify]
                 [-rom <file>] [-stats] <pattern>...
```

`vet` 对匹配到的所有引擎执行完整 Compiler 链路，但不序列化、不写入 `dist/`，也不读取 `//sonolus:level` 开发关卡。`-m`、`-O`、`-rom` 和 `-stats` 与 `build` 含义相同；默认检查全部四种模式。它适合在 CI 或提交前验证源码，首个失败引擎会终止检查并报告其 package path。

## list

```text
sonolus-go list <pattern>...
```

`list` 要求 patterns 恰好匹配一个引擎，并将关卡工具可消费的 archetype 字段 schema 输出到 stdout。它只解析 Play、Watch、Preview 声明，不编译 callback、不执行优化或 backend，也不读取 Development Level；行为对应 `go list` 的结构化项目查询用途。

字段按 Play exports、Play imports、Watch imports、Preview imports 的顺序合并去重；Watch 的 `#ACCURACY` 和 `#JUDGMENT` 被排除。输出格式与 `sonolus.py schema` 兼容。该并集是工具字段目录，不会放宽 `dev` 对三模式共同 imported 字段的校验。

## dev

```text
sonolus-go dev [-o <name>] [-addr <:8080>]
                 [-O 0|1|2] [-runtime-checks none|terminate|notify]
                 [-rom <file>] [-stats] <pattern>...
```

`dev` 总是编译四种模式且要求 patterns 恰好匹配一个引擎；`-o` 可覆盖开发服务器显示的引擎名称。它启动完整 Sonolus 开发服务器，提供一个 engine item、一个 `Dev Level` 和内置的有效开发资源。Sonolus 客户端可直接连接 `-addr` 对应的地址。

`dev` 的 runtime checks 默认 `notify`。终端输入 `decode <code>` 可查询当前成功快照的诊断消息，输入 `help` 查看命令；失败重编译不会替换诊断表。

除标准 `/sonolus/*` info、list、details 和 repository 路由外，它保留以下编译诊断端点：

- `/sonolus/engines/info`
- `/sonolus/engine/configuration`
- `/sonolus/engine/play-data`
- `/sonolus/engine/watch-data`
- `/sonolus/engine/preview-data`
- `/sonolus/engine/tutorial-data`
- `/sonolus/engine/rom`（存在 ROM 时）

它监听成功快照中的 Go 和 embed 文件，包括 `//sonolus:level` 绑定的 LevelData JSON。文件变化后创建新的 Compiler 并重新编译；失败时记录错误并继续服务上一次成功快照。开发 LevelData 不属于 `build` 输出。

## version

```bash
sonolus-go version
sonolus-go -version
sonolus-go --version
```

输出构建时注入的版本、commit 和日期；未注入时显示开发默认值。
