# 快速入门

## 安装

### 方式一：下载预编译二进制（推荐）

从 [GitHub Releases](https://github.com/WindowsSov8forUs/sonolus-go/releases) 下载对应平台的最新版本，解压后放入 PATH 即可使用，无需安装 Go 或任何依赖仓库。

```bash
# Linux x86_64
curl -L https://github.com/WindowsSov8forUs/sonolus-go/releases/latest/download/sonolus-go_linux_x86_64.tar.gz | tar xz
sudo mv sonolus-go /usr/local/bin/

# macOS (Apple Silicon)
curl -L https://github.com/WindowsSov8forUs/sonolus-go/releases/latest/download/sonolus-go_darwin_arm64.tar.gz | tar xz
sudo mv sonolus-go /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/WindowsSov8forUs/sonolus-go/releases/latest/download/sonolus-go_darwin_x86_64.tar.gz | tar xz
sudo mv sonolus-go /usr/local/bin/

# Windows — 从 Releases 页面下载 zip 解压，或 PowerShell：
Invoke-WebRequest -Uri "https://github.com/WindowsSov8forUs/sonolus-go/releases/latest/download/sonolus-go_windows_x86_64.zip" -OutFile sonolus-go.zip
Expand-Archive sonolus-go.zip -DestinationPath .
```

### 方式二：从源码编译

需要 Go 1.25+。依赖仓库会通过 Go 模块代理自动拉取，无需手动 clone。

```bash
git clone https://github.com/WindowsSov8forUs/sonolus-go.git
cd sonolus-go
go build ./cmd/sonolus-go/
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
