# CS Native

<p align="center">
  <strong>README 语言</strong><br />
  <a href="README.md">🇨🇳 中文</a> ·
  <a href="docs/readme/README.en.md">🇺🇸 English</a> ·
  <a href="docs/readme/README.ko.md">🇰🇷 한국어</a> ·
  <a href="docs/readme/README.ar.md">🇸🇦 العربية</a> ·
  <a href="docs/readme/README.de.md">🇩🇪 Deutsch</a> ·
  <a href="docs/readme/README.fr.md">🇫🇷 Français</a> ·
  <a href="docs/readme/README.la.md">🇻🇦 Latina</a>
</p>

<p align="center">
  默认中文。点击上方国旗可切换到对应语言的 README；GitHub Markdown 不支持脚本式语言切换，所以这里使用静态多语言文档链接。
</p>

<p align="center">
  <a href="https://github.com/eust-w/CSNative/actions/workflows/release.yml"><img src="https://github.com/eust-w/CSNative/actions/workflows/release.yml/badge.svg" alt="Release workflow" /></a>
  <a href="https://github.com/eust-w/CSNative/actions/workflows/pages.yml"><img src="https://github.com/eust-w/CSNative/actions/workflows/pages.yml/badge.svg" alt="Pages workflow" /></a>
  <a href="https://github.com/eust-w/CSNative/releases/latest"><img src="https://img.shields.io/github/v/release/eust-w/CSNative?label=release" alt="Latest release" /></a>
</p>

<p align="center">
  <img src="build/appicon.png" alt="CS Native icon" width="104" />
</p>

<p align="center">
  <strong>CS Native 是面向 Claude Science 的本地 provider / runtime control plane。</strong>
</p>

<p align="center">
  它把 Claude Science 放进隔离沙箱运行，把模型请求路由到你自己配置的第三方 provider，同时保留官方 Claude 模式作为独立入口。
</p>

<p align="center">
  <img src="docs/assets/readme/csn-flow.gif" alt="CS Native 页面流程动图" width="860" />
</p>

## 这是什么

CS Native 不是单一的 DeepSeek / Qwen 切换器。它的核心是本地控制面板：

- 管理多个 provider profile：base URL、API key、adapter、默认模型、Science 模型映射。
- 启动本地代理、虚拟 OAuth、Claude Science 沙箱和 fresh nonce 公网入口。
- 只把第三方模型模式放进隔离沙箱，不写真实 Claude Science 凭证目录。
- 官方 Claude 模式保持独立，只负责打开真实 Claude Science。

当前实现是 Go / Wails 端到端重写：

- 桌面壳：Wails + 静态 WebView 前端。
- 代理层：支持 `anthropic_messages` 与 `openai_chat_completions` 两类 adapter。
- 登录层：内置虚拟 OAuth 写入器，写入隔离目录。
- 运行层：内置沙箱进程管理、日志、自检和 fresh nonce public entry。

## 界面预览

### 启动

启动页只显示已经启用并且保存了 key 的 provider。选择 provider 后，一键启动本地代理、沙箱 Science 和 fresh nonce 入口。

![启动页](docs/assets/readme/hero-start.png)

### 模型与凭证

同一页管理 provider、base URL、API key、adapter、默认模型和 Claude Science 模型映射。缺 key 的 provider 会保留在配置列表里，但不会进入启动下拉。

![模型与凭证](docs/assets/readme/provider-management.png)

### 网络与公网入口

网络页只负责端口和公网域名设置。`/` 路由到 fresh nonce helper，其它路径转发到 Science 沙箱。

![网络页](docs/assets/readme/network-public-entry.png)

### 日志与诊断

日志页读取本地真实日志文件，关于页放自检、反馈、更新和版本信息。

![日志页](docs/assets/readme/runtime-logs.png)

## 基本用法

1. 打开 **模型与凭证**。
2. 选择内置 provider 预设，或新增一个自定义 provider。
3. 填写 `base URL`、`API key`、`adapter`、默认模型和 Science 模型映射。
4. 点击 **验证**，确认 provider 可用。
5. 回到 **启动**，选择已启用且有 key 的 provider。
6. 点击 **一键开始**，CS Native 会启动本地代理、虚拟 OAuth、沙箱 Science 和 fresh nonce 入口。
7. 如果要完全回到官方 Claude Science，切换到 **官方 Claude** 模式。

## 多语言与 i18n

CS Native 默认使用中文，并支持以下语言：

| 语言 | README | 应用 / 宣传页语言代码 |
| --- | --- | --- |
| 🇨🇳 中文 | [README.md](README.md) | `zh-CN` |
| 🇺🇸 English | [README.en.md](docs/readme/README.en.md) | `en` |
| 🇰🇷 한국어 | [README.ko.md](docs/readme/README.ko.md) | `ko` |
| 🇸🇦 العربية | [README.ar.md](docs/readme/README.ar.md) | `ar` |
| 🇩🇪 Deutsch | [README.de.md](docs/readme/README.de.md) | `de` |
| 🇫🇷 Français | [README.fr.md](docs/readme/README.fr.md) | `fr` |
| 🇻🇦 Latina | [README.la.md](docs/readme/README.la.md) | `la` |

桌面应用的语言入口在侧栏底部；GitHub Pages 宣传页的语言入口在右上角。两者都会记住本地选择。阿拉伯语会切换为 RTL 排版。

## 下载与发布

产品宣传页：[https://eust-w.github.io/CSNative/](https://eust-w.github.io/CSNative/)

最新构建在 [GitHub Releases](https://github.com/eust-w/CSNative/releases/latest) 下载。按系统选择对应资产：

| 系统 / 架构 | 下载资产 | 内容 |
| --- | --- | --- |
| macOS Apple Silicon | `CSNative-v*-darwin-arm64.zip` | 压缩后的 macOS `.app` |
| macOS Intel | `CSNative-v*-darwin-amd64.zip` | 压缩后的 macOS `.app` |
| Windows x64 | `CSNative-v*-windows-amd64.zip` | Windows 可执行文件包 |
| Linux x64 | `CSNative-v*-linux-amd64.tar.gz` | Linux 可执行文件包 |
| 全平台 | `checksums.txt` | SHA-256 校验文件 |

推送 `v*` tag 会触发自动发布流程：测试、`go vet`、Wails 多平台构建、Release 上传，并同步发布 [`ghcr.io/eust-w/csnative-app`](https://github.com/eust-w/CSNative/pkgs/container/csnative-app) package。

## 内置 provider 预设

| Provider | Adapter | 说明 |
| --- | --- | --- |
| DeepSeek | `anthropic_messages` | 适合 Anthropic-compatible 上游 |
| 阿里云 DashScope / Qwen | `openai_chat_completions` | 使用 OpenAI-compatible chat completions 路径 |
| OpenAI-compatible | `openai_chat_completions` | 通用 OpenAI 兼容端点 |
| Anthropic-compatible | `anthropic_messages` | 通用 Anthropic Messages 兼容端点 |
| OpenRouter | `openai_chat_completions` | 可编辑默认模型和映射 |
| Moonshot / Kimi | `openai_chat_completions` | 可编辑默认模型和映射 |
| SiliconFlow | `openai_chat_completions` | 可编辑默认模型和映射 |
| Ollama | `openai_chat_completions` | 本地模型入口，默认 `127.0.0.1:11434` |

所有预设都是可编辑默认值。模型列表不会自动同步供应商市场，实际可用模型以你的 provider 控制台为准。

## 运行边界

CS Native 的安全边界很明确：

- 配置目录：`~/.csnative`
- 沙箱 HOME：`~/.csnative/sandbox/home`
- 本地日志：`~/.csnative/logs/csnative.log`
- Provider key：保存在 `~/.csnative/config.json`，文件权限为 `0600`
- 不写真实 Claude Science 凭证目录
- 不复制真实 OAuth token 到沙箱
- 不使用修改过的 `ANTHROPIC_*` 环境变量启动真实 Science

默认端口：

| 服务 | 地址 |
| --- | --- |
| 本地代理 | `127.0.0.1:18991` |
| Science 沙箱 | `127.0.0.1:8990` |
| fresh nonce 公网入口 | `127.0.0.1:8992` |

公网域名建议：

| 外部路径 | 本地目标 | 用途 |
| --- | --- | --- |
| `/` | `127.0.0.1:8992` | 生成 fresh nonce |
| `/*` | `127.0.0.1:8990` | 转发到 Science 沙箱 |

## 从源码运行

### 前提

- macOS
- 已安装 Claude Science
- 已安装 Go
- 已安装 Wails CLI
- 至少一个第三方 provider API key

### 构建与测试

```bash
go test ./...
go vet ./...
go build ./cmd/csnative
wails build
```

构建完成后，应用包位于：

```text
build/bin/CSNative.app
```

命令行入口：

```bash
go run ./cmd/csnative
```

## 目录结构

```text
cmd/csnative/                 命令行入口
internal/app/                 Wails app API、配置、启动、日志和自检
internal/config/              provider profile 与本地配置读写
internal/proxy/               Anthropic/OpenAI adapter 与代理逻辑
internal/oauth/               虚拟 OAuth 写入与护栏
internal/science/             Claude Science 沙箱进程和 public entry
internal/desktop/frontend/    设置面板静态前端
docs/assets/readme/           README 截图和动图
docs/brand.md                 品牌说明
build/appicon.png             生产图标源文件
```

## 开发检查

常用验证命令：

```bash
node --check internal/desktop/frontend/dist/main.js
go test ./...
go vet ./...
go build ./cmd/csnative
wails build -clean
```

如果改了页面交互，建议同时用浏览器打开静态前端检查：

```bash
cd internal/desktop/frontend/dist
python3 -m http.server 4179 --bind 127.0.0.1
```

然后访问：

```text
http://127.0.0.1:4179/
```

## 品牌

CS Native 使用 `CSN` 作为应用识别标记，并保留 `eust-w` 品牌方向。产品定位见 [docs/brand.md](docs/brand.md)。

![移动端启动页](docs/assets/readme/mobile-start.png)
