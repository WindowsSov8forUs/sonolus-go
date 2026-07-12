# 性能与规模

## 先测量

使用 CLI `-stats` 查看当前项目的阶段耗时：

```bash
sonolus-go build -stats -O 2 -m all ./engine
```

输出区分：

- 每模式 `load`：`packages.Load` 和类型检查。
- 每模式 `frontend`：声明解析与 callback lowering。
- `optimize`：所有 callback 的 IR 优化。
- `backend`：CFG finalization、SNode peephole 和 EngineData 组装。
- `cached`：请求是否完全命中同一 Compiler 快照。

不同 CLI 调用会创建新的 Compiler，不共享源码快照缓存。

## 选择优化等级

- 开发和定位 lowering 问题：`-O 0`，结构最直接。
- 大型项目希望减少优化时间：`-O 1`。
- 发布产物：默认 `-O 2`。

三个等级都必须语义正确；区别是编译成本、Temporary Memory 复用和节点规模，不是功能开关。

## 多模式加载

四模式必须分别执行 `packages.Load`，因为 build tags 决定参与类型检查的文件集合。首次 `CompileAll` 会并行加载缺失模式，但 frontend、共享合并和最终提交保持确定性顺序。

将真正共享的 Configuration、ROM 和 helper 放在无模式 tag 文件中；将模式资源与 archetype 放在对应 tag 文件中。这样既保持声明一致，也避免单个模式加载无关声明。

## Callback 结构

用户 helper 会在编译期内联，不产生运行时函数调用栈。过深或大量重复 helper 调用会扩大初始 IR，Standard 通常能通过 inline cleanup、DCE 和 CSE 回收一部分，但不应依赖优化器消除明显重复工作。

建议：

- 把纯计算保留为短 helper，提高 catalog recipe 和数据流分析可见性。
- 不要为了抽象而复制大型 aggregate。
- 对高频 callback，减少重复 semantic memory 读写和资源操作。
- 使用静态可确定的定长数组和 catalog 容器，避免无法编译的动态结构。

## Temporary Memory

单 callback 最多使用 4096 Temporary Memory slots。多 slot struct、定长数组和容器会连续占用 slots。

如果 Minimal 超限而 Standard 成功，说明活性复用有效；如果 Standard 仍超限，应缩短大 aggregate 生命周期、拆分表达式或减少同时存活的本地数据。该限制不会通过自动降级或溢出到未知 memory block 绕过。

## 节点池

Backend 在模式范围共享 node pool，子节点优先写入并按值或 `RuntimeFunction + argument indexes` 确定性去重。同模式 callback 可以复用等价纯结构，不同模式拥有独立节点索引。

节点数不是唯一性能指标。包含副作用的 RuntimeCall 不会为了减小节点数而删除、合并或重排。

## 开发服务器

`serve` 监听 Compiler 成功快照中的 Go 文件和 embed 文件。每次变化创建新 Compiler，避免把不同源码版本混入同一快照；编译失败时保留上一次成功 artifacts。

大型项目中频繁保存多个文件可能触发连续编译。当前 watcher 不承诺 debounce，因此批量修改时可先使用编辑器的原子保存策略，或在完成一组修改后再启动 `serve`。

## 基准与回归

性能改动必须同时检查：

- 四模式 EngineData 语义一致。
- callback 返回值、semantic memory 和副作用顺序一致。
- node index 全部有效。
- Temporary Memory 不超过 4096。
- 重复与并发编译结果确定。

推荐验收：

```bash
go test -race -count=1 ./internal/compiler/...
go test -count=1 ./...
go vet ./...
go build ./...
gofmt -l internal cmd sonolus
git diff --check
```
