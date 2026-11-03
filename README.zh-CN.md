# enver

[**English**](README.md) | **简体中文**

> 面向全栈开发者的环境编排工具，以 [vfox](https://vfox.dev) 为底座。

**当前版本：0.0.1** · MIT · Linux / macOS / Windows

vfox 回答的是：**“当前 Node / Python / Java 用的是哪个版本？”**
enver 回答的是：**“这个项目需要的一整套环境是什么？能不能一键复现？”**

一个 Go 单体二进制、一个本地 Web 界面、一套完整 CLI。不需要账号，不上云，不收集遥测数据。

```
浏览器（React + xterm.js）
   │  HTTP + SSE + WebSocket（仅 127.0.0.1，带会话令牌）
enver-core（Go）  ──  enver CLI
   │  子进程 + 环境变量注入
vfox CLI  ──  vfox 插件  ──  已安装的运行时
```

## 为什么需要 enver

vfox 是优秀的版本管理器，但它一次只管理一个 SDK。真实的全栈项目需要的是一套**组合**：
Node 20 + Python 3.12 + Java 17，再配一个虚拟环境、一组环境变量和固定端口。enver 负责这一层，
下载、解压、插件、校验等事情全部交回 vfox。

- **预设**：把多个运行时版本、环境变量和 venv 组合命名，可全局应用，也可只应用到某个项目。
- **Python 隔离**：基于 vfox 管理的解释器创建、检查、重建和激活 `venv` / `uv` 环境。
- **全局库可见**：按运行时版本查看装了哪些全局包，导出清单，理解切换 Node 版本后“包去哪了”。
- **存储可控**：修改 SDK 根目录，迁移已有运行时并逐步校验，确认无误前保留旧目录。
- **诊断能力**：PATH 冲突、软链接断裂、虚拟环境失效、磁盘空间，以及最经典的
  “另一个版本管理器抢先遮蔽了 vfox”。
- **真正本地的 Web 界面**：只监听回环地址，一次性的会话令牌，除插件目录和你主动调用的包管理器外不发起外部请求。

## 功能

| 领域 | 能力 |
| --- | --- |
| 运行时 | SDK 目录、版本搜索、带实时日志的安装队列、卸载、全局/项目切换 |
| 预设 | 多运行时预设、环境变量、可选 venv、全局或项目级应用 |
| Python | `uv` 或标准库 `venv`、失效检测、重建、终端激活、外部解释器兼容模式 |
| 全局库 | 按版本列出 `npm -g` / `pip` 包、筛选、导出清单 |
| 存储 | `storage.sdkPath`、`VFOX_HOME`、复制 → 校验 → 重建软链 → 可选删除的迁移流程 |
| 诊断 | vfox 健康度、PATH 遮蔽、软链接断链、venv 健康、磁盘空间、Shell 集成 |
| 终端 | 内嵌 PTY 终端，直接注入预设与 venv 环境 |
| CLI | 全功能，支持 JSON 输出，不启动 UI 也能完成所有操作 |

## 环境要求

- [vfox](https://vfox.dev) **>= 1.0.13**，需在 `PATH` 中（或设置 `ENVER_VFOX_BIN`）。
- Go **1.25+**，仅从源码构建时需要。
- Node **22+**，仅构建前端时需要。
- Windows 10 1809+ 才能使用内嵌终端（ConPTY）；Linux 和 macOS 均支持。

## 安装

### 从源码构建

```bash
git clone https://github.com/enver-dev/enver.git
cd enver
make build          # 先构建前端，再嵌入二进制
./dist/enver --version
```

Windows 没有 `make` 时：

```powershell
cd web; npm ci; npm run build; cd ..
go build -trimpath -o dist\enver.exe .\cmd\enver
```

### go install

前端已经嵌入二进制，因此可以直接：

```bash
go install github.com/enver-dev/enver/cmd/enver@latest
```

### 发布包

打 tag 发版时会生成 `linux/amd64`、`linux/arm64`、`darwin/amd64`、`darwin/arm64`、
`windows/amd64`、`windows/arm64` 的压缩包，以及 `checksums.txt`。

## 快速开始

```bash
enver
```

启动本地界面（默认 `127.0.0.1:7788`），打印带一次性令牌的地址，并打开浏览器。令牌换取
HttpOnly Cookie 后即失效于 JavaScript 侧，不会写入前端存储。

```bash
enver ui --no-open --port 7788       # 不自动打开浏览器
enver ui --no-auth                   # 仅用于本地开发
enver ui --no-open --token ci-token  # 为自动化指定固定令牌
```

固定 `--token` 仅用于脚本和 CI；日常使用请依赖启动时打印的一次性地址。

### CLI 示例

```bash
enver sdk list
enver sdk search nodejs
enver sdk add python
enver install nodejs@22.23.4
enver use nodejs@22.23.4 -g
enver use nodejs@22.23.4 -p --dir D:\work\myapp

enver preset ls
enver preset apply fullstack --scope global
enver preset apply fullstack --scope project --dir D:\work\myapp

enver venv create --python 3.12.4 --tool uv --project D:\work\myapp
enver venv ls
enver venv rebuild <id>
enver venv rm <id>

enver pkg ls nodejs@22.23.4
enver pkg export nodejs@22.23.4

enver doctor
enver storage show
enver storage migrate D:\vfox-sdks --delete-old
enver env --preset fullstack --json
```

所有列表类命令都支持 `--json`。

## 界面

| 页面 | 用途 |
| --- | --- |
| 总览 | vfox 状态、已安装运行时、存储空间、就绪清单 |
| 运行时 | 插件列表、版本安装/卸载/切换、实时安装日志 |
| 预设 | 多运行时组合、环境变量、全局/项目应用 |
| 虚拟环境 | venv/uv 创建、健康检查、重建、终端激活 |
| 全局库 | 按版本查看全局包并导出清单 |
| 诊断 | 一键体检并给出修复提示 |
| 终端 | 内嵌 PTY，注入预设与 venv |
| 设置 | SDK 存储位置、迁移、vfox 可执行文件 |

## Python 隔离

- 基础解释器默认来自 vfox。enver 不允许 `uv` 自行下载 Python：创建时传入明确解释器路径，
  并设置 `UV_PYTHON_DOWNLOADS=never`。
- 有 `uv` 时优先使用，否则回退到 `python -m venv`。
- 每个环境记录基础解释器、版本、工具、项目目录和预设。enver 会对照 `pyvenv.cfg` 检测
  Python 升级后的环境失效，并支持一键重建。
- **兼容模式**：如果 vfox 的 Python 插件无法安装（见已知问题），可以显式指定一个已有解释器路径。
  该模式在界面上有明确标注，默认仍然使用 vfox，隔离能力不受影响。

## 存储位置

两个容易混淆的开关：

| 配置 | 影响范围 | 适用场景 |
| --- | --- | --- |
| `vfox config storage.sdkPath <dir>` | 仅 SDK 安装目录，配置/插件/缓存仍在 `~/.vfox` | 想把运行时放到另一块盘 |
| 环境变量 `VFOX_HOME` | 整个 vfox 主目录，包括插件 | 便携模式或多用户共享 |

迁移向导的流程是：预检 → 复制 → 校验 → 写入 vfox 配置 → 重建全局软链 → 可选删除旧目录。
迁移期间会持有独占锁，安装、卸载、切换和预设应用会排队等待，避免复制过程中 SDK 目录被改动。
同时识别当前布局（`<sdkPath>/<sdk>/v-<version>`）和 1.0.2–1.0.7 的旧布局
（`<sdkPath>/cache/<sdk>`），也兼容 1.0 之前的 `~/.version-fox` 主目录。

## 安全模型

- 只绑定 `127.0.0.1`。
- 每次启动生成随机令牌，通过 `/auth` 换取 HttpOnly、SameSite=Strict 的 Cookie。
- 令牌使用常量时间比较；带外部 `Origin` 的写请求会被拒绝。
- 终端 WebSocket 校验 `Origin`。
- 所有命令以参数数组执行，不拼接 shell 字符串。
- 无遥测。外部请求仅限插件目录、vfox 本身，以及你主动调用的包管理器。

## 已知问题

- **Windows 上的 vfox Python 插件。** `vfox 1.0.13` 搭配 `vfox-python 0.6.2` 可能在
  `PostInstall` 阶段失败，报 `attempt to call a non-function object`（插件对 `io.popen`
  句柄调用了 `listing:lines()`）。这是上游问题。enver 会展示失败原因，并在虚拟环境页面提供
  外部解释器作为兼容方案。
- **Windows 项目作用域**依赖软链接。需要开启开发者模式或以管理员身份运行，否则请使用全局作用域。
  enver 会原样展示 vfox 的错误，不会静默失败。
- **Windows 下 `vfox use -g`** 会把用户环境变量写入注册表，这是 vfox 的行为，可能影响
  enver 之外新开的终端。

## 开发

```bash
make web      # 构建 React 前端到 internal/webui/dist
make build    # 生成 ./dist/enver，并注入版本信息
make test     # go test ./...
make vet      # go vet ./...
```

前后端热更新：

```bash
enver ui --no-auth --no-open --port 7788   # 终端 1
cd web && npm run dev                      # 终端 2，/api 代理到 7788
```

### 构建产物与仓库约定

- `dist/`、`.tools/`、`web/node_modules/`、`web/dist/`、`go.work` 都只属于本地开发，不提交。
- `internal/webui/dist/` **是刻意提交的**：这样 `go install` 无需 Node.js 也能得到完整界面。
  修改 `web/` 后请执行 `make web`，并在同一个提交中带上重新生成的全部资源。
- 二进制、覆盖率文件、日志、本地 env 文件和编辑器状态均已忽略，不要提交。
- 前端使用 Vite，并将 React、xterm 拆成独立 vendor chunk；`index.html` 引用带哈希的文件，
  因此 `internal/webui/dist` 必须整体提交，不能只提交其中一部分。

## 项目结构

```
cmd/enver              CLI 入口
internal/app           CLI 与 HTTP 共用的用例层；存储迁移
internal/vfox          vfox 适配层、兼容逻辑、目录扫描
internal/httpapi       REST、SSE 任务流、WebSocket 终端
internal/jobs          后台任务与有界日志流
internal/preset        预设
internal/venv          虚拟环境元数据与健康检查
internal/packages      npm/pip 查询与导出
internal/doctor        诊断
internal/fsutil        原子 JSON、目录复制、体积统计
internal/platform      磁盘空间等平台能力
internal/run           子进程执行与逐行输出
internal/catalog       插件目录缓存与内置兜底
internal/config        应用配置
internal/buildinfo     版本信息
internal/webui         嵌入的前端资源
web                    React + Vite 前端
docs                   架构与运维文档
```

更详细的设计说明见 [docs/architecture.md](docs/architecture.md)（英文）。

## 路线图

- **P1**：全局包跨版本 diff/迁移、快照与回滚、镜像与代理中心、端口管理、`env.lock` 复现、
  项目导入、IDE 解释器写入、任务运行器。
- **P2**：团队预设共享、离线运行时包、进程面板、插件调试界面、WSL 管理、便携模式、
  EOL/CVE 提示、多机同步。

## 贡献

欢迎提交 issue 和 PR。请先阅读 [CONTRIBUTING.md](CONTRIBUTING.md)，并在提交前运行
`make test` 与 `make vet`。

## 许可证

MIT，详见 [LICENSE](LICENSE)。
