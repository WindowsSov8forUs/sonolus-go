# 快速入门

## 环境

- Go 1.25+
- 依赖仓库（本地开发时相邻 checkout）：

```
sonolus-go/          ← 本仓库
sonolus-core-go/     ← EngineData 数据结构
sonolus-pack-go/     ← 打包工具
sonolus-server-go/   ← HTTP 服务器接口
```

```bash
git clone https://github.com/WindowsSov8forUs/sonolus-go.git
git clone https://github.com/WindowsSov8forUs/sonolus-core-go.git
git clone https://github.com/WindowsSov8forUs/sonolus-pack-go.git
git clone https://github.com/WindowsSov8forUs/sonolus-server-go.git
```

## 快速构建

```bash
go build ./...    # 编译
go test ./...     # 测试
go vet ./...      # 静态分析
```

## 第一个引擎

创建 `engine.go`：

```go
package myengine

type Skin struct { Note float64 }

type Note struct {
    Beat float64 `sonolus:"imported"`
}

func (n *Note) Initialize() {
    draw(sonolus.SpriteNote, n.Beat, 0, 1, 1, 0, 1, 0, 0)
}
```

编译：

```bash
go run ./cmd/sonolus-go build -m play engine.go
```

## 本地开发服务器

```bash
go run ./cmd/sonolus-go serve engine.go
```

源码变更时自动重新编译并热加载。

## 多文件引擎

```
my-engine/
├── engine.go           # 主包：Skin、资源定义
├── notes/
│   └── note.go         # 子包：Note 原型
└── stage/
    └── stage.go        # 子包：Stage 原型
```

```bash
sonolus-go build ./my-engine/ -m play
```

详见 [DSL 语言参考](dsl-reference.md)。

---

> 下一步：[DSL 语言参考](dsl-reference.md) · [CLI 参考](cli.md)
